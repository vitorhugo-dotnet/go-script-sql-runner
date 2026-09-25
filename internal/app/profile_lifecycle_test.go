package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func lifecycleProfile(name string) profile.Profile {
	return profile.Profile{
		Name:       name,
		Connection: profile.Connection{Host: "db.example", Port: 3306, Username: "root", Password: "secret"},
		Execution:  profile.Execution{OnError: profile.OnErrorStop, TransactionMode: profile.TransactionRunnerManaged},
	}
}

func TestCloneProfileCopiesSettingsScriptsAndGeneratesUniqueIDs(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	first, err := service.CreateProfile(context.Background(), lifecycleProfile("Shared name"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateProfile(context.Background(), lifecycleProfile("Shared name"))
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "setup.sql")
	if err := os.WriteFile(sourcePath, []byte("SELECT 42;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddScript(context.Background(), first.ID, sourcePath); err != nil {
		t.Fatal(err)
	}
	first, err = repo.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := service.CloneProfile(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("CloneProfile() error: %v", err)
	}
	if clone.ID == "" || clone.ID == first.ID || clone.ID == second.ID || clone.Name != "Shared name (copy)" {
		t.Fatalf("clone identity = %#v", clone)
	}
	if clone.Version != first.Version || clone.Connection != first.Connection || clone.Execution != first.Execution || !reflect.DeepEqual(clone.Scripts, first.Scripts) {
		t.Fatalf("clone lost settings or scripts: %#v; source %#v", clone, first)
	}
	stored, err := repo.Get(clone.ID)
	if err != nil || !reflect.DeepEqual(stored, clone) {
		t.Fatalf("stored clone = %#v, %v", stored, err)
	}
	data, err := os.ReadFile(filepath.Join(repo.ScriptRoot(clone.ID), filepath.Base(first.Scripts[0].File)))
	if err != nil || string(data) != "SELECT 42;\n" {
		t.Fatalf("clone script = %q, %v", data, err)
	}
	otherClone, err := service.CloneProfile(context.Background(), second.ID)
	if err != nil || otherClone.ID == clone.ID || otherClone.ID == second.ID || otherClone.Name != clone.Name {
		t.Fatalf("second clone = %#v, %v", otherClone, err)
	}
}

func TestDeleteProfileClosesOnlyMatchingActiveConnection(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	active, err := service.CreateProfile(context.Background(), lifecycleProfile("Active"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateProfile(context.Background(), lifecycleProfile("Other"))
	if err != nil {
		t.Fatal(err)
	}
	recorded := &recordingConn{}
	service.connectServer = func(ctx context.Context, _ profile.Connection) (*database.Client, database.ConnectionResult, error) {
		db := sql.OpenDB(recordingConnector{conn: recorded})
		if err := db.PingContext(ctx); err != nil {
			return nil, database.ConnectionResult{}, err
		}
		return &database.Client{DB: db}, database.ConnectionResult{}, nil
	}
	if _, err := service.Connect(context.Background(), active.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteProfile(context.Background(), other.ID); err != nil {
		t.Fatalf("DeleteProfile(other) error: %v", err)
	}
	if recorded.closed != 0 || service.activeClient == nil || service.activeProfileID != active.ID {
		t.Fatal("deleting another profile closed the active connection")
	}
	if _, err := repo.Get(other.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted profile Get() error = %v", err)
	}
	if err := service.DeleteProfile(context.Background(), "../outside"); err == nil {
		t.Fatal("DeleteProfile(invalid ID) succeeded")
	}
	if recorded.closed != 0 || service.activeClient == nil {
		t.Fatal("failed deletion changed active connection")
	}
	if err := service.DeleteProfile(context.Background(), active.ID); err != nil {
		t.Fatalf("DeleteProfile(active) error: %v", err)
	}
	if recorded.closed != 1 || service.activeClient != nil || service.activeProfileID != "" {
		t.Fatalf("active connection after deletion: closed=%d client=%#v id=%q", recorded.closed, service.activeClient, service.activeProfileID)
	}
	if _, err := repo.Get(active.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("active profile Get() error = %v", err)
	}
}

func TestProfileLifecycleCancelledContextDoesNotMutate(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	service := NewService(repo)
	source, err := service.CreateProfile(context.Background(), lifecycleProfile("Source"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.CloneProfile(ctx, source.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("CloneProfile(cancelled) error = %v", err)
	}
	if err := service.DeleteProfile(ctx, source.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteProfile(cancelled) error = %v", err)
	}
	profiles, err := repo.List()
	if err != nil || len(profiles) != 1 || profiles[0].ID != source.ID {
		t.Fatalf("profiles after cancelled operations = %#v, %v", profiles, err)
	}
}
