package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zydou/portal/cmd/portal/config"
	sender_ui "github.com/zydou/portal/cmd/portal/tui/sender"
	"github.com/zydou/portal/internal/file"
	"github.com/zydou/portal/internal/portal"
	"github.com/zydou/portal/internal/semver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// -------------------------------------------------------- Send -------------------------------------------------------

func Send(version string) *cobra.Command {
	sendCmd := &cobra.Command{
		Use:   "send file1 file2...",
		Short: "Send one or more files",
		Long:  "The send command adds one or more files to be sent. Files are archived before sending.",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("relay", cmd.Flags().Lookup("relay")); err != nil {
				return fmt.Errorf("binding relay flag: %w", err)
			}
			if err := viper.BindPFlag("tui_style", cmd.Flags().Lookup("tui-style")); err != nil {
				return fmt.Errorf("binding tui-style flag: %w", err)
			}
			return nil

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file.RemoveTemporaryFiles(file.SEND_TEMP_FILE_NAME_PREFIX)

			logFile, err := setupLoggingFromViper("send")
			if err != nil {
				return err
			}
			defer func() { _ = logFile.Close() }()
			switch viper.GetString("tui_style") {
			case config.StyleRich:
				if err := handleSendCommand(version, args); err != nil {
					return fmt.Errorf("running rich send command: %w", err)
				}
			case config.StyleRaw:
				if err := handleSendCommandRaw(version, args); err != nil {
					return fmt.Errorf("running raw send command: %w", err)
				}
			default:
				return errors.New("invalid tui style provided")
			}
			return nil
		},
	}
	sendCmd.Flags().StringP("relay", "r", "", relayFlagDesc)
	sendCmd.Flags().StringP("tui-style", "s", "", tuiStyleFlagDesc)
	return sendCmd
}

// ------------------------------------------------------ Handlers -----------------------------------------------------

// handleSendCommand is the sender application.
func handleSendCommand(version string, fileNames []string) error {
	var opts []sender_ui.Option
	ver, err := semver.Parse(version)
	// Conditionally add option to sender ui
	if err == nil {
		opts = append(opts, sender_ui.WithVersion(ver))
	}
	relayAddr := viper.GetString("relay")
	sender := sender_ui.New(fileNames, relayAddr, opts...)
	if _, err := sender.Run(); err != nil {
		return fmt.Errorf("running tui: %w", err)
	}
	fmt.Println("")
	return nil
}

func handleSendCommandRaw(version string, filenames []string) error {
	ctx := context.Background()
	relayAddr := viper.GetString("relay")
	ver, err := semver.Parse(version)
	if err != nil {
		return fmt.Errorf("parsing version: %w", err)
	}
	serverVer, err := semver.GetRendezvousVersion(ctx, relayAddr)
	if err != nil {
		return fmt.Errorf("fetching version from relay: %w", err)
	}
	switch ver.Compare(serverVer) {
	case semver.CompareNewMajor, semver.CompareOldMajor:
		return fmt.Errorf("incompatible version %s -> %s", ver, serverVer)
	case semver.CompareEqual:
		fmt.Printf("• Portal version (%s) compatible with server version (%s)\n", ver, serverVer)
	case semver.CompareNewMinor, semver.CompareNewPatch:
		fmt.Printf("• Portal version (%s) newer than server version (%s)\n", ver, serverVer)
	case semver.CompareOldMinor, semver.CompareOldPatch:
		fmt.Printf("• Server version (%s) newer than Portal version (%s)\n", serverVer, ver)
	}
	files := make([]*os.File, 0, len(filenames))
	for _, name := range filenames {
		f, err := os.Open(name)
		if err != nil {
			return fmt.Errorf("unable to open file %q: %w", name, err)
		}
		defer func() { _ = f.Close() }()
		files = append(files, f)
	}
	payload, size, err := file.PackFiles(files)
	if err != nil {
		return fmt.Errorf("error packing files: %w", err)
	}
	defer func() { _ = payload.Close() }()
	defer file.RemoveTemporaryFiles(file.SEND_TEMP_FILE_NAME_PREFIX)
	cnf := portal.Config{
		RendezvousAddr: relayAddr,
	}
	password, errC, err := portal.Send(ctx, payload, size, &cnf)
	if err != nil {
		return fmt.Errorf("doing initial handshake: %w", err)
	}
	fmt.Println(password)
	err = <-errC
	if err != nil {
		return fmt.Errorf("doing portal transfer: %w", err)
	}
	return nil
}
