package executor

import (
	"context"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func TestRunOptionsOverrideTransactionMode(t *testing.T) {
	db, state := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "script", "SELECT 1;")

	p := runnerProfile(profile.OnErrorContinue)
	p.Execution.TransactionMode = profile.TransactionAutoCommit
	p.Scripts = []profile.Script{{
		ID:      "script",
		Name:    "Script",
		File:    "scripts/script.sql",
		Enabled: true,
		Order:   10,
	}}

	summary := Run(
		context.Background(),
		&database.Client{DB: db},
		p,
		root,
		RunOptions{TransactionMode: profile.TransactionRunnerManaged},
		nil,
	)
	if summary.Failed != 0 || summary.Succeeded != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.begins != 1 || state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("runtime transaction override was ignored: begins=%d commits=%d rollbacks=%d", state.begins, state.commits, state.rollbacks)
	}
}
