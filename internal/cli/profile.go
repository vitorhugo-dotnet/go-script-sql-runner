package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func newProfileCommand(ctx context.Context, service Service, stdin io.Reader, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "profile", Short: "Manage script profiles"}
	command.AddCommand(
		newProfileListCommand(ctx, service),
		newProfileCreateCommand(ctx, service),
		newProfileShowCommand(ctx, service),
		newProfileExportCommand(ctx, service, stderr),
		newProfileImportCommand(ctx, service, stdin, stderr),
	)
	return command
}

func newProfileListCommand(ctx context.Context, service Service) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, err := service.ListProfiles(ctx)
			if err != nil {
				return err
			}
			for _, p := range profiles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.ID, p.Name)
			}
			return nil
		},
	}
}

func newProfileCreateCommand(ctx context.Context, service Service) *cobra.Command {
	var name, host, databaseName, username, password string
	var port int
	var onError, transaction string
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			created, err := service.CreateProfile(ctx, profile.Profile{
				Name: name,
				Connection: profile.Connection{Host: host, Port: port, Database: databaseName, Username: username, Password: password},
				Execution: profile.Execution{OnError: profile.OnError(onError), TransactionMode: profile.TransactionMode(transaction)},
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created profile %s (%s)\n", created.Name, created.ID)
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&name, "name", "", "profile name")
	flags.StringVar(&host, "host", "", "MySQL host")
	flags.IntVar(&port, "port", 3306, "MySQL port")
	flags.StringVar(&databaseName, "database", "", "database/schema")
	flags.StringVar(&username, "username", "", "database username")
	flags.StringVar(&password, "password", "", "database password")
	flags.StringVar(&onError, "on-error", string(profile.OnErrorContinue), "failure policy: continue or stop")
	flags.StringVar(&transaction, "transaction-mode", string(profile.TransactionAutoCommit), "transaction mode: auto_commit, transaction, or script_managed")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("host")
	_ = command.MarkFlagRequired("database")
	_ = command.MarkFlagRequired("username")
	return command
}

func newProfileShowCommand(ctx context.Context, service Service) *cobra.Command {
	return &cobra.Command{
		Use:   "show <profile-id>",
		Short: "Show a profile with its password redacted",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := service.GetProfile(ctx, args[0])
			if err != nil {
				return err
			}
			if p.Connection.Password != "" {
				p.Connection.Password = "***"
			}
			var output bytes.Buffer
			if err := profile.Encode(&output, p); err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(output.Bytes())
			return err
		},
	}
}
