package commands

import (
	"fmt"

	sender_ui "github.com/zydou/portal/cmd/portal/tui/sender"
	"github.com/zydou/portal/internal/file"
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
			return nil

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file.RemoveTemporaryFiles(file.SEND_TEMP_FILE_NAME_PREFIX)

			logFile, err := setupLoggingFromViper("send")
			if err != nil {
				return err
			}
			defer func() { _ = logFile.Close() }()

			if err := handleSendCommand(version, args); err != nil {
				return fmt.Errorf("running send command: %w", err)
			}
			return nil
		},
	}
	sendCmd.Flags().StringP("relay", "r", "", relayFlagDesc)
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
