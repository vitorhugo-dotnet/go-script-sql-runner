package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type Service interface {
	CreateProfile(context.Context, profile.Profile) (profile.Profile, error)
	ListProfiles(context.Context) ([]profile.Profile, error)
	GetProfile(context.Context, string) (profile.Profile, error)
	AddScript(context.Context, string, string) (profile.Script, error)
	RemoveScript(context.Context, string, string) error
	TestConnection(context.Context, string) (database.ServerCapabilities, error)
	RunProfile(context.Context, string, executor.RunOptions, executor.Sink) (executor.Summary, error)
}

type executionState struct {
	exitCode int
}

func Execute(ctx context.Context, service Service, args []string, stdout, stderr io.Writer) int {
	state := &executionState{}
	root := newRootCommand(ctx, service, stdout, stderr, state)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return state.exitCode
}

func newRootCommand(ctx context.Context, service Service, stdout, stderr io.Writer, state *executionState) *cobra.Command {
	root := &cobra.Command{
		Use:           "runner",
		Short:         "Run configured MySQL SQL profiles",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newProfileCommand(ctx, service), newScriptCommand(ctx, service), newConnectionCommand(ctx, service), newRunCommand(ctx, service, stdout, state))
	return root
}
