package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func TestCreateProfileAppliesDefaults(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	created, err := service.CreateProfile(context.Background(), profile.Profile{
		Name: "Dev",
		Connection: profile.Connection{Host: "127.0.0.1", Database: "demo", Username: "root"},
	})
	if err != nil {
		t.Fatalf("CreateProfile() error: %v", err)
	}
	if created.ID == "" || created.Version != 1 || created.Connection.Port != 3306 {
		t.Fatalf("missing defaults: %#v", created)
	}
	if created.Execution.OnError != profile.OnErrorContinue || created.Execution.TransactionMode != profile.TransactionAutoCommit {
		t.Fatalf("execution defaults: %#v", created.Execution)
	}
	stored, err := repo.Get(created.ID)
	if err != nil || stored.ID != created.ID {
		t.Fatalf("profile not persisted: %#v, %v", stored, err)
	}
}

func TestAddScriptCreatesCanonicalMetadata(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name: "Dev",
		Connection: profile.Connection{Host: "127.0.0.1", Database: "demo", Username: "root"},
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

func TestTestConnectionUsesConfiguredConnectorAndClosesNilDBSafely(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	p, err := service.CreateProfile(context.Background(), profile.Profile{
		Name: "Dev",
		Connection: profile.Connection{Host: "db.example", Database: "demo", Username: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	service.connect = func(_ context.Context, connection profile.Connection) (*database.Client, error) {
		called = true
		if connection.Host != "db.example" {
			t.Fatalf("connector host = %q", connection.Host)
		}
		return &database.Client{Capabilities: database.ServerCapabilities{Vendor: database.VendorMySQL, Major: 5, Minor: 7, Patch: 44, RawVersion: "5.7.44", VersionLabel: "MySQL 5.7"}}, nil
	}
	caps, err := service.TestConnection(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("TestConnection() error: %v", err)
	}
	if !called || caps.VersionLabel != "MySQL 5.7" {
		t.Fatalf("unexpected capabilities: %#v", caps)
	}
}
