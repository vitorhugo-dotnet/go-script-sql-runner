package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

type recordingConnector struct {
	conn *recordingConn
}

func (c recordingConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c recordingConnector) Driver() driver.Driver                        { return recordingDriver{} }

type recordingDriver struct{}

func (recordingDriver) Open(string) (driver.Conn, error) { return nil, io.EOF }

type recordingConn struct {
	executed []string
	closed   int
}

func (c *recordingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *recordingConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c *recordingConn) Close() error {
	c.closed++
	return nil
}
func (c *recordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.executed = append(c.executed, query)
	return driver.RowsAffected(1), nil
}

func TestConnectKeepsSessionForRunAndCloseIsIdempotent(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(t.TempDir(), "setup.sql")
	if err := os.WriteFile(scriptPath, []byte("SELECT 42;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddScript(context.Background(), p.ID, scriptPath); err != nil {
		t.Fatal(err)
	}

	recorded := &recordingConn{}
	db := sql.OpenDB(recordingConnector{conn: recorded})
	connectCalls := 0
	service.connectServer = func(_ context.Context, connection profile.Connection) (*database.Client, database.ConnectionResult, error) {
		connectCalls++
		return &database.Client{DB: db}, database.ConnectionResult{
			Capabilities: database.ServerCapabilities{Vendor: database.VendorMySQL, VersionLabel: "MySQL 8.0"},
			Schemas:      []string{"apollo"},
		}, nil
	}

	result, err := service.Connect(context.Background(), p.ID)
	if err != nil || result.Capabilities.VersionLabel != "MySQL 8.0" {
		t.Fatalf("Connect() result=%#v err=%v", result, err)
	}
	summary, err := service.RunProfile(context.Background(), p.ID, executor.RunOptions{Schema: "apollo"}, nil)
	if err != nil || summary.Succeeded != 1 {
		t.Fatalf("RunProfile() summary=%#v err=%v", summary, err)
	}
	if connectCalls != 1 {
		t.Fatalf("server connections = %d, want one persistent session", connectCalls)
	}
	wantExecuted := []string{"USE `apollo`", "SELECT 42;"}
	if strings.Join(recorded.executed, "|") != strings.Join(wantExecuted, "|") {
		t.Fatalf("executed = %#v, want %#v", recorded.executed, wantExecuted)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
	if recorded.closed != 1 {
		t.Fatalf("physical closes = %d, want one", recorded.closed)
	}
}

func TestCloseRejectsAndReleasesConnectionThatFinishesDuringShutdown(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	recorded := &recordingConn{}
	service.connectServer = func(ctx context.Context, _ profile.Connection) (*database.Client, database.ConnectionResult, error) {
		close(started)
		<-release
		db := sql.OpenDB(recordingConnector{conn: recorded})
		if err := db.PingContext(ctx); err != nil {
			return nil, database.ConnectionResult{}, err
		}
		return &database.Client{DB: db}, database.ConnectionResult{}, nil
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.Connect(context.Background(), p.ID)
		done <- err
	}()
	<-started
	if err := service.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("Connect() succeeded after shutdown")
	}
	if recorded.closed != 1 {
		t.Fatalf("late physical closes = %d, want one", recorded.closed)
	}
}

func TestCreateProfileAppliesDefaultsAndDropsLegacyDatabaseTarget(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	created, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "127.0.0.1", Database: "must-not-persist", Username: "root"},
	})
	if err != nil {
		t.Fatalf("CreateProfile() error: %v", err)
	}
	if created.ID == "" || created.Version != 1 || created.Connection.Port != 3306 {
		t.Fatalf("missing defaults: %#v", created)
	}
	if created.Connection.Database != "" {
		t.Fatalf("database persisted = %q, want blank runtime target", created.Connection.Database)
	}
	if created.Execution.OnError != profile.OnErrorContinue || created.Execution.TransactionMode != profile.TransactionAutoCommit {
		t.Fatalf("execution defaults: %#v", created.Execution)
	}
	stored, err := repo.Get(created.ID)
	if err != nil || stored.ID != created.ID {
		t.Fatalf("profile not persisted: %#v, %v", stored, err)
	}
	if stored.Connection.Database != "" {
		t.Fatalf("stored database = %q, want blank runtime target", stored.Connection.Database)
	}
}

func TestAddScriptCreatesCanonicalMetadata(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "127.0.0.1", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "base-setup.sql")
	if err := os.WriteFile(source, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	script, err := service.AddScript(context.Background(), p.ID, source)
	if err != nil {
		t.Fatalf("AddScript() error: %v", err)
	}
	if script.ID == "" || script.Name != "base-setup" || script.Order != 10 || !script.Enabled {
		t.Fatalf("unexpected script: %#v", script)
	}
	if script.File != "scripts/"+script.ID+".sql" {
		t.Fatalf("canonical file = %q", script.File)
	}
}

func TestConnectReturnsCapabilitiesAndVisibleSchemas(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	service.connectServer = func(_ context.Context, connection profile.Connection) (*database.Client, database.ConnectionResult, error) {
		called = true
		if connection.Host != "db.example" {
			t.Fatalf("tester host = %q", connection.Host)
		}
		return &database.Client{}, database.ConnectionResult{
			Capabilities: database.ServerCapabilities{Vendor: database.VendorMySQL, Major: 5, Minor: 7, Patch: 44, RawVersion: "5.7.44", VersionLabel: "MySQL 5.7"},
			Schemas:      []string{"apollo", "mysql"},
		}, nil
	}
	result, err := service.Connect(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	if !called || result.Capabilities.VersionLabel != "MySQL 5.7" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Schemas) != 2 || result.Schemas[0] != "apollo" || result.Schemas[1] != "mysql" {
		t.Fatalf("schemas = %#v", result.Schemas)
	}
}

func TestRunProfileRejectsMissingRuntimeSchemaBeforeConnecting(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name:       "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RunProfile(context.Background(), p.ID, executor.RunOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "schema is required") {
		t.Fatalf("RunProfile() error = %v, want schema validation", err)
	}
}

func TestRunProfileUsesExplicitRuntimeSchemaInsteadOfLegacyProfileDatabase(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	legacy := profile.Profile{
		ID:         "legacy",
		Name:       "Legacy",
		Version:    1,
		Connection: profile.Connection{Host: "db.example", Port: 3306, Database: "legacy_db", Username: "root"},
		Execution:  profile.Execution{OnError: profile.OnErrorContinue, TransactionMode: profile.TransactionAutoCommit},
	}
	if err := repo.Save(legacy); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo)
	recorded := &recordingConn{}
	service.connectServer = func(_ context.Context, connection profile.Connection) (*database.Client, database.ConnectionResult, error) {
		if connection.Database != "legacy_db" {
			t.Fatalf("legacy profile unexpectedly rewritten during load: %#v", connection)
		}
		return &database.Client{DB: sql.OpenDB(recordingConnector{conn: recorded})}, database.ConnectionResult{}, nil
	}
	if _, err := service.Connect(context.Background(), legacy.ID); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	_, err := service.RunProfile(context.Background(), legacy.ID, executor.RunOptions{Schema: " runtime_db "}, nil)
	if err != nil {
		t.Fatalf("RunProfile() error: %v", err)
	}
	if len(recorded.executed) != 1 || recorded.executed[0] != "USE `runtime_db`" {
		t.Fatalf("executed = %#v, want runtime schema selection", recorded.executed)
	}
}
