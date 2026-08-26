package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
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
	Connect(context.Context, string) (database.ConnectionResult, error)
	RunProfile(context.Context, string, executor.RunOptions, executor.Sink) (executor.Summary, error)
	InspectProfileArchive(context.Context, string) (profile.ArchiveInspection, error)
	ExportProfile(context.Context, string, string) error
	ImportProfile(context.Context, string, bool) (profile.Profile, error)
}

type executionState struct {
	exitCode int
}

func Execute(ctx context.Context, service *app.Service, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return executeWithService(ctx, service, args, stdin, stdout, stderr)
}

func executeWithService(ctx context.Context, service Service, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	state := &executionState{}
	root := newRootCommand(ctx, service, stdin, stdout, stderr, state)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return state.exitCode
}

func newRootCommand(ctx context.Context, service Service, stdin io.Reader, stdout, stderr io.Writer, state *executionState) *cobra.Command {
	root := &cobra.Command{
		Use:           "runner",
		Short:         "Run configured MySQL SQL profiles",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newProfileCommand(ctx, service, stdin, stderr), newScriptCommand(ctx, service), newConnectionCommand(ctx, service), newRunCommand(ctx, service, stdout, state))
	return root
}
