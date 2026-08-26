package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newConnectionCommand(ctx context.Context, service Service) *cobra.Command {
	command := &cobra.Command{Use: "connection", Short: "Test database connections"}
	command.AddCommand(&cobra.Command{
		Use:   "test <profile-id>",
		Short: "Connect, detect the MySQL server version, and discover schemas",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := service.Connect(ctx, args[0])
			if err != nil {
				return err
			}
			caps := result.Capabilities
			name := caps.VersionLabel
			if caps.RawVersion != "" {
				name = fmt.Sprintf("%s (%s)", caps.VersionLabel, caps.RawVersion)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Connected — %s\n", name)
			return nil
		},
	})
	return command
}
