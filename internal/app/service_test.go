package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func TestCreateProfileAppliesDefaultsAndDropsLegacyDatabaseTarget(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	created, err := service.CreateProfile(context.Background(), profile.Profile{
		Name: "Dev",
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
		Name: "Dev",
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

func TestTestConnectionReturnsCapabilitiesAndVisibleSchemas(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name: "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	service.testConnection = func(_ context.Context, connection profile.Connection) (database.ConnectionResult, error) {
		called = true
		if connection.Host != "db.example" {
			t.Fatalf("tester host = %q", connection.Host)
		}
		return database.ConnectionResult{
			Capabilities: database.ServerCapabilities{Vendor: database.VendorMySQL, Major: 5, Minor: 7, Patch: 44, RawVersion: "5.7.44", VersionLabel: "MySQL 5.7"},
			Schemas:      []string{"apollo", "mysql"},
		}, nil
	}
	result, err := service.TestConnection(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("TestConnection() error: %v", err)
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
		Name: "Dev",
		Connection: profile.Connection{Host: "db.example", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	service.connectDatabase = func(context.Context, profile.Connection, string) (*database.Client, error) {
		called = true
		return &database.Client{}, nil
	}
	_, err = service.RunProfile(context.Background(), p.ID, executor.RunOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "schema is required") {
		t.Fatalf("RunProfile() error = %v, want schema validation", err)
	}
	if called {
		t.Fatal("database connector called before schema validation")
	}
}

func TestRunProfileUsesExplicitRuntimeSchemaInsteadOfLegacyProfileDatabase(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	legacy := profile.Profile{
		ID:      "legacy",
		Name:    "Legacy",
		Version: 1,
		Connection: profile.Connection{Host: "db.example", Port: 3306, Database: "legacy_db", Username: "root"},
		Execution: profile.Execution{OnError: profile.OnErrorContinue, TransactionMode: profile.TransactionAutoCommit},
	}
	if err := repo.Save(legacy); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo)
	var gotSchema string
	service.connectDatabase = func(_ context.Context, connection profile.Connection, schema string) (*database.Client, error) {
		if connection.Database != "legacy_db" {
			t.Fatalf("legacy profile unexpectedly rewritten during load: %#v", connection)
		}
		gotSchema = schema
		return &database.Client{}, nil
	}
	_, err := service.RunProfile(context.Background(), legacy.ID, executor.RunOptions{Schema: " runtime_db "}, nil)
	if err != nil {
		t.Fatalf("RunProfile() error: %v", err)
	}
	if gotSchema != "runtime_db" {
		t.Fatalf("runtime schema = %q, want runtime_db", gotSchema)
	}
}
