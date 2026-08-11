package commands

import (
	"fmt"
	"strconv"

	receiver_tui "github.com/zydou/portal/cmd/portal/tui/receiver"
	"github.com/zydou/portal/internal/file"
	"github.com/zydou/portal/internal/semver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ------------------------------------------------------ Receive ------------------------------------------------------

func Receive(version string) *cobra.Command {
	receiveCmd := &cobra.Command{
		Use:   "receive",
		Short: "Receive files",
		Long:  "The receive command receives files from the sender with the matching password.",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Bind flags to viper.
			if err := viper.BindPFlag("relay", cmd.Flags().Lookup("relay")); err != nil {
				return fmt.Errorf("binding relay flag: %w", err)
			}

			// Reverse the --yes/-y flag value as it has an inverse relationship
			// with the configuration value 'prompt_overwrite_files'.
			overwriteFlag := cmd.Flags().Lookup("yes")
			if overwriteFlag.Changed {
				shouldOverwrite, _ := strconv.ParseBool(overwriteFlag.Value.String())
				_ = overwriteFlag.Value.Set(strconv.FormatBool(!shouldOverwrite))
			}

			if err := viper.BindPFlag("prompt_overwrite_files", overwriteFlag); err != nil {
				return fmt.Errorf("binding yes flag: %w", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file.RemoveTemporaryFiles(file.RECEIVE_TEMP_FILE_NAME_PREFIX)

			logFile, err := setupLoggingFromViper("receive")
			if err != nil {
				return err
			}
			defer func() { _ = logFile.Close() }()

			pwd := args[0]
			if err := handleReceiveCommand(version, pwd); err != nil {
				return fmt.Errorf("running receive command: %w", err)
			}
			return nil
		},
	}
	receiveCmd.Flags().StringP("relay", "r", "", relayFlagDesc)
	receiveCmd.Flags().BoolP("yes", "y", false, "Overwrite existing files without [Y/n] prompts")
	return receiveCmd
}

// ------------------------------------------------------ Handlers -----------------------------------------------------

// handleReceiveCommand is the receive application.
func handleReceiveCommand(version string, password string) error {
	var opts []receiver_tui.Option
	ver, err := semver.Parse(version)
	if err == nil {
		opts = append(opts, receiver_tui.WithVersion(ver))
	}
	receiver := receiver_tui.New(viper.GetString("relay"), password, opts...)

	if _, err := receiver.Run(); err != nil {
		return fmt.Errorf("running receiver tui: %w", err)
	}
	fmt.Println("")
	return nil
}
