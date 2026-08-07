package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newScriptCommand(ctx context.Context, service Service) *cobra.Command {
	command := &cobra.Command{Use: "script", Short: "Manage profile SQL scripts"}
	command.AddCommand(&cobra.Command{
		Use:   "add <profile-id> <file.sql>",
		Short: "Copy a SQL script into the profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			script, err := service.AddScript(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added script %s (%s)\n", script.Name, script.ID)
			return nil
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "remove <profile-id> <script-id>",
		Short: "Remove a script from a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.RemoveScript(ctx, args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Removed script")
			return nil
		},
	})
	return command
}
