package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func newRunCommand(ctx context.Context, service Service, stdout io.Writer, state *executionState) *cobra.Command {
	var verbose, stopOnError, continueOnError bool
	command := &cobra.Command{
		Use:   "run <profile-id>",
		Short: "Execute an ordered SQL profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			opts := executor.RunOptions{}
			if stopOnError {
				opts.OnError = profile.OnErrorStop
			} else if continueOnError {
				opts.OnError = profile.OnErrorContinue
			}
			sink := executor.SinkFunc(func(event executor.Event) {
				if verbose {
					fmt.Fprintf(stdout, "%s %-5s", event.Time.Format("15:04:05.000"), event.Level)
					if event.ScriptID != "" {
						fmt.Fprintf(stdout, " [%s]", event.ScriptID)
					}
					fmt.Fprintf(stdout, " %s", event.Message)
					if event.Detail != "" {
						fmt.Fprintf(stdout, " — %s", event.Detail)
					}
					fmt.Fprintln(stdout)
					return
				}
				switch {
				case event.Level == executor.LevelWarn:
					fmt.Fprintf(stdout, "! %s\n", event.Message)
				case event.Level == executor.LevelError:
					fmt.Fprintf(stdout, "✗ %s", event.Message)
					if event.Detail != "" {
						fmt.Fprintf(stdout, " — %s", event.Detail)
					}
					fmt.Fprintln(stdout)
				case event.Level == executor.LevelInfo && strings.HasPrefix(event.Message, "Completed "):
					fmt.Fprintf(stdout, "✓ %s\n", strings.TrimPrefix(event.Message, "Completed "))
				}
			})
			summary, err := service.RunProfile(ctx, args[0], opts, sink)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "\n%d succeeded, %d failed\n", summary.Succeeded, summary.Failed)
			if summary.Failed > 0 || summary.Aborted {
				state.exitCode = 1
			}
			return nil
		},
	}
	command.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed execution logs")
	command.Flags().BoolVar(&stopOnError, "stop-on-error", false, "stop after the first failed script")
	command.Flags().BoolVar(&continueOnError, "continue-on-error", false, "continue after failed scripts")
	command.MarkFlagsMutuallyExclusive("stop-on-error", "continue-on-error")
	return command
}
