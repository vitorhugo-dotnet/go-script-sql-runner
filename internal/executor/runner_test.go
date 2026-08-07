package executor

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

var testDriverCounter atomic.Uint64

type dbState struct {
	mu        sync.Mutex
	execs     []string
	begins    int
	commits   int
	rollbacks int
}

type fakeDriver struct{ state *dbState }
type fakeConn struct{ state *dbState }
type fakeTx struct{ state *dbState }

func (d *fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{state: d.state}, nil }
func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("prepare unsupported") }
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }
func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.begins++
	c.state.mu.Unlock()
	return &fakeTx{state: c.state}, nil
}
func (c *fakeConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execs = append(c.state.execs, query)
	if strings.Contains(query, "FAIL") {
		return nil, errors.New("synthetic failure containing secret")
	}
	return driver.RowsAffected(1), nil
}
func (tx *fakeTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.commits++
	return nil
}
func (tx *fakeTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rollbacks++
	return nil
}

func openFakeDB(t *testing.T) (*sql.DB, *dbState) {
	t.Helper()
	state := &dbState{}
	name := fmt.Sprintf("runner_fake_%d", testDriverCounter.Add(1))
	sql.Register(name, &fakeDriver{state: state})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, state
}

func writeSQL(t *testing.T, root, id, content string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id+".sql"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runnerProfile(onError profile.OnError) profile.Profile {
	return profile.Profile{
		ID: "profile", Name: "Profile", Version: 1,
		Connection: profile.Connection{Host: "127.0.0.1", Port: 3306, Database: "demo", Username: "dev", Password: "secret"},
		Execution: profile.Execution{OnError: onError, TransactionMode: profile.TransactionAutoCommit},
	}
}

func TestRunOrdersScriptsSkipsDisabledAndContinues(t *testing.T) {
	db, state := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "first", "SELECT 'FIRST';")
	writeSQL(t, root, "fail", "SELECT 'FAIL secret';")
	writeSQL(t, root, "last", "SELECT 'LAST';")

	p := runnerProfile(profile.OnErrorContinue)
	p.Scripts = []profile.Script{
		{ID: "last", Name: "Last", File: "scripts/last.sql", Enabled: true, Order: 30},
		{ID: "disabled", Name: "Disabled", File: "scripts/disabled.sql", Enabled: false, Order: 5},
		{ID: "fail", Name: "Fail", File: "scripts/fail.sql", Enabled: true, Order: 20},
		{ID: "first", Name: "First", File: "scripts/first.sql", Enabled: true, Order: 10},
	}
	var events []Event
	summary := Run(context.Background(), &database.Client{DB: db}, p, root, RunOptions{}, SinkFunc(func(event Event) { events = append(events, event) }))
	if summary.Succeeded != 2 || summary.Failed != 1 || summary.Aborted {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.Results) != 3 || summary.Results[0].ScriptID != "first" || summary.Results[1].ScriptID != "fail" || summary.Results[2].ScriptID != "last" {
		t.Fatalf("unexpected result order: %#v", summary.Results)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 3 || !strings.Contains(state.execs[0], "FIRST") || !strings.Contains(state.execs[2], "LAST") {
		t.Fatalf("unexpected execution order: %#v", state.execs)
	}
	for _, event := range events {
		if strings.Contains(event.Detail, "secret") {
			t.Fatalf("event leaked password: %#v", event)
		}
	}
	if strings.Contains(summary.Results[1].Error, "secret") {
		t.Fatalf("result leaked password: %q", summary.Results[1].Error)
	}
}

func TestRunStopsAfterFailureWhenConfigured(t *testing.T) {
	db, state := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "fail", "FAIL;")
	writeSQL(t, root, "next", "SELECT 'NEXT';")
	p := runnerProfile(profile.OnErrorStop)
	p.Scripts = []profile.Script{
		{ID: "fail", Name: "Fail", File: "scripts/fail.sql", Enabled: true, Order: 10},
		{ID: "next", Name: "Next", File: "scripts/next.sql", Enabled: true, Order: 20},
	}
	summary := Run(context.Background(), &database.Client{DB: db}, p, root, RunOptions{}, nil)
	if !summary.Aborted || summary.Failed != 1 || len(summary.Results) != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 1 {
		t.Fatalf("executed %d scripts, want 1", len(state.execs))
	}
}

func TestRunOptionsOverrideFailurePolicy(t *testing.T) {
	db, state := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "fail", "FAIL;")
	writeSQL(t, root, "next", "SELECT 1;")
	p := runnerProfile(profile.OnErrorContinue)
	p.Scripts = []profile.Script{
		{ID: "fail", Name: "Fail", File: "scripts/fail.sql", Enabled: true, Order: 10},
		{ID: "next", Name: "Next", File: "scripts/next.sql", Enabled: true, Order: 20},
	}
	Run(context.Background(), &database.Client{DB: db}, p, root, RunOptions{OnError: profile.OnErrorStop}, nil)
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 1 {
		t.Fatalf("runtime stop override executed %d scripts", len(state.execs))
	}
}

func TestRunnerManagedTransactionCommitAndRollback(t *testing.T) {
	db, state := openFakeDB(t)
	if err := executeSQL(context.Background(), db, "SELECT 1;", profile.TransactionRunnerManaged); err != nil {
		t.Fatalf("executeSQL success error: %v", err)
	}
	if err := executeSQL(context.Background(), db, "FAIL;", profile.TransactionRunnerManaged); err == nil {
		t.Fatal("executeSQL failure expected error")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.begins != 2 || state.commits != 1 || state.rollbacks != 1 {
		t.Fatalf("transaction counts: begins=%d commits=%d rollbacks=%d", state.begins, state.commits, state.rollbacks)
	}
}

func TestRunWarnsAboutDDLInRunnerManagedTransaction(t *testing.T) {
	db, _ := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "ddl", "CREATE TABLE demo(id INT);")
	p := runnerProfile(profile.OnErrorContinue)
	p.Execution.TransactionMode = profile.TransactionRunnerManaged
	p.Scripts = []profile.Script{{ID: "ddl", Name: "DDL", File: "scripts/ddl.sql", Enabled: true, Order: 10}}
	warned := false
	Run(context.Background(), &database.Client{DB: db}, p, root, RunOptions{}, SinkFunc(func(event Event) {
		if event.Level == LevelWarn && strings.Contains(event.Message, "implicit commit") {
			warned = true
		}
	}))
	if !warned {
		t.Fatal("expected implicit-commit warning")
	}
}

func TestRunRejectsDelimiterBeforeDatabaseExecution(t *testing.T) {
	db, state := openFakeDB(t)
	root := t.TempDir()
	writeSQL(t, root, "proc", "DELIMITER //\nCREATE PROCEDURE p() SELECT 1//")
	p := runnerProfile(profile.OnErrorContinue)
	p.Scripts = []profile.Script{{ID: "proc", Name: "Procedure", File: "scripts/proc.sql", Enabled: true, Order: 10}}
	summary := Run(context.Background(), &database.Client{DB: db}, p, root, RunOptions{}, nil)
	if summary.Failed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execs) != 0 {
		t.Fatalf("DELIMITER SQL reached database: %#v", state.execs)
	}
}
