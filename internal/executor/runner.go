package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/redact"
)

const implicitCommitWarning = "script contains MySQL DDL that can cause an implicit commit; complete rollback is not guaranteed"

func Run(ctx context.Context, client *database.Client, p profile.Profile, scriptRoot string, options RunOptions, sink Sink) Summary {
	summary := Summary{Results: make([]ScriptResult, 0, len(p.Scripts))}
	if client == nil || client.DB == nil {
		emit(sink, Event{Time: time.Now(), Level: LevelError, Message: "database client is not available"})
		summary.Aborted = true
		return summary
	}

	scripts := append([]profile.Script(nil), p.Scripts...)
	sort.SliceStable(scripts, func(i, j int) bool { return scripts[i].Order < scripts[j].Order })

	onError := p.Execution.OnError
	if options.OnError != "" {
		onError = options.OnError
	}

	transactionMode := p.Execution.TransactionMode
	if options.TransactionMode != "" {
		transactionMode = options.TransactionMode
	}

	for _, script := range scripts {
		if !script.Enabled {
			continue
		}
		if ctx.Err() != nil {
			summary.Aborted = true
			break
		}

		started := time.Now()
		emit(sink, Event{Time: started, Level: LevelInfo, ScriptID: script.ID, Message: "Starting " + script.Name})

		mode := transactionMode
		if script.TransactionMode != "" {
			mode = script.TransactionMode
		}

		scriptPath := filepath.Join(scriptRoot, filepath.Base(filepath.FromSlash(script.File)))
		sqlBytes, err := os.ReadFile(scriptPath)
		if err == nil {
			var analysis Analysis
			analysis, err = Analyze(string(sqlBytes))
			if err == nil && mode == profile.TransactionRunnerManaged && analysis.HasImplicitCommitDDL {
				emit(sink, Event{Time: time.Now(), Level: LevelWarn, ScriptID: script.ID, Message: implicitCommitWarning})
			}
			if err == nil {
				err = executeSQL(ctx, client.DB, string(sqlBytes), mode)
			}
		}

		ended := time.Now()
		result := ScriptResult{ScriptID: script.ID, Name: script.Name, StartedAt: started, EndedAt: ended, Success: err == nil}
		if err != nil {
			redacted := redact.Secrets(err.Error(), p.Connection.Password)
			result.Error = redacted
			summary.Failed++
			emit(sink, Event{Time: ended, Level: LevelError, ScriptID: script.ID, Message: "Failed " + script.Name, Detail: redacted})
		} else {
			summary.Succeeded++
			emit(sink, Event{Time: ended, Level: LevelInfo, ScriptID: script.ID, Message: "Completed " + script.Name})
		}
		summary.Results = append(summary.Results, result)

		if err != nil && (onError == profile.OnErrorStop || ctx.Err() != nil) {
			summary.Aborted = true
			break
		}
	}
	return summary
}

func emit(sink Sink, event Event) {
	if sink != nil {
		sink.Emit(event)
	}
}

func ResultError(summary Summary) error {
	if summary.Failed == 0 {
		return nil
	}
	return fmt.Errorf("%d script(s) failed", summary.Failed)
}
