package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
)

const plaintextCredentialsWarning = "WARNING: exported profile contains database credentials in plaintext."

func newProfileExportCommand(ctx context.Context, service Service, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "export <profile-id> <file.zip>",
		Short: "Export a self-contained profile ZIP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(stderr, plaintextCredentialsWarning)
			if err := service.ExportProfile(ctx, args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported profile to %s\n", args[1])
			return nil
		},
	}
}

func newProfileImportCommand(ctx context.Context, service Service, stdin io.Reader, stderr io.Writer) *cobra.Command {
	var force bool
	command := &cobra.Command{
		Use:   "import <file.zip>",
		Short: "Import a self-contained profile ZIP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspection, err := service.InspectProfileArchive(ctx, args[0])
			if err != nil {
				return err
			}
			imported, err := service.ImportProfile(ctx, args[0], false)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Imported profile %s (%s)\n", imported.Name, imported.ID)
				return nil
			}

			var conflict *app.ProfileConflictError
			if !errors.As(err, &conflict) {
				return err
			}
			if force {
				imported, err = service.ImportProfile(ctx, args[0], true)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Imported profile %s (%s)\n", imported.Name, imported.ID)
				return nil
			}

			fmt.Fprintf(stderr, "Profile %q already exists and will be completely overwritten. Continue? [y/N]:\n", inspection.Profile.Name)
			answer, readErr := bufio.NewReader(stdin).ReadString('\n')
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return fmt.Errorf("read overwrite confirmation: %w", readErr)
			}
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "y", "yes":
				imported, err = service.ImportProfile(ctx, args[0], true)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Imported profile %s (%s)\n", imported.Name, imported.ID)
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "Import cancelled.")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&force, "force", false, "overwrite an existing profile without confirmation")
	return command
}
