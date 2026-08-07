package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func sharingProfile(id, name, password string) profile.Profile {
	return profile.Profile{
		ID: id, Name: name, Version: 1,
		Connection: profile.Connection{Host: "127.0.0.1", Port: 3306, Database: "synthetic", Username: "dev", Password: password},
		Execution: profile.Execution{OnError: profile.OnErrorContinue, TransactionMode: profile.TransactionAutoCommit},
	}
}

func addSyntheticScript(t *testing.T, repo *storage.Repository, profileID, scriptID, sqlText string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), scriptID+".sql")
	if err := os.WriteFile(source, []byte(sqlText), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := repo.Get(profileID)
	if err != nil {
		t.Fatal(err)
	}
	order := (len(p.Scripts) + 1) * 10
	if err := repo.AddScript(profileID, profile.Script{ID: scriptID, Name: scriptID, Enabled: true, Order: order}, source); err != nil {
		t.Fatal(err)
	}
}

func createSharingArchive(t *testing.T, id, name, password string) (string, profile.Profile) {
	t.Helper()
	repo := storage.NewRepository(t.TempDir())
	p := sharingProfile(id, name, password)
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	addSyntheticScript(t, repo, id, "incoming-script", "SELECT 'incoming';")
	p, err := repo.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "profile.zip")
	if err := profile.ExportArchive(repo.ProfileDir(id), archivePath); err != nil {
		t.Fatal(err)
	}
	return archivePath, p
}

func TestImportProfileReturnsTypedConflictWithoutChangingExistingFiles(t *testing.T) {
	targetRepo := storage.NewRepository(t.TempDir())
	existing := sharingProfile("shared-id", "Existing", "existing-secret")
	if err := targetRepo.Save(existing); err != nil {
		t.Fatal(err)
	}
	addSyntheticScript(t, targetRepo, existing.ID, "old-script", "SELECT 'old';")
	archivePath, incoming := createSharingArchive(t, existing.ID, "Incoming", "incoming-secret")
	service := NewService(targetRepo)

	_, err := service.ImportProfile(context.Background(), archivePath, false)
	var conflict *ProfileConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("ImportProfile() error = %v, want ProfileConflictError", err)
	}
	if conflict.Existing.Name != "Existing" || conflict.Incoming.Name != incoming.Name {
		t.Fatalf("unexpected conflict: %#v", conflict)
	}
	if strings.Contains(err.Error(), "existing-secret") || strings.Contains(err.Error(), "incoming-secret") {
		t.Fatalf("conflict error leaked credentials: %q", err)
	}
	stored, err := targetRepo.Get(existing.ID)
	if err != nil || stored.Name != "Existing" || len(stored.Scripts) != 1 || stored.Scripts[0].ID != "old-script" {
		t.Fatalf("existing profile changed on conflict: %#v, %v", stored, err)
	}
}

func TestImportProfileOverwriteCompletelyReplacesProfile(t *testing.T) {
	targetRepo := storage.NewRepository(t.TempDir())
	existing := sharingProfile("shared-id", "Existing", "existing-secret")
	if err := targetRepo.Save(existing); err != nil {
		t.Fatal(err)
	}
	addSyntheticScript(t, targetRepo, existing.ID, "old-script", "SELECT 'old';")
	archivePath, incoming := createSharingArchive(t, existing.ID, "Incoming", "incoming-secret")
	service := NewService(targetRepo)

	imported, err := service.ImportProfile(context.Background(), archivePath, true)
	if err != nil {
		t.Fatalf("ImportProfile(overwrite) error: %v", err)
	}
	if imported.Name != incoming.Name || imported.Connection.Password != "incoming-secret" || len(imported.Scripts) != 1 || imported.Scripts[0].ID != "incoming-script" {
		t.Fatalf("unexpected imported profile: %#v", imported)
	}
	if _, err := os.Stat(filepath.Join(targetRepo.ScriptRoot(existing.ID), "old-script.sql")); !os.IsNotExist(err) {
		t.Fatalf("old script survived overwrite: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(targetRepo.ScriptRoot(existing.ID), "incoming-script.sql"))
	if err != nil || string(got) != "SELECT 'incoming';" {
		t.Fatalf("incoming script = %q, %v", got, err)
	}
}

func TestInspectAndExportProfileRoundTrip(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	p := sharingProfile("export-id", "Export", "synthetic-password-123")
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	addSyntheticScript(t, repo, p.ID, "script-one", "SELECT 1;")
	service := NewService(repo)
	destination := filepath.Join(t.TempDir(), "out.zip")
	if err := service.ExportProfile(context.Background(), p.ID, destination); err != nil {
		t.Fatalf("ExportProfile() error: %v", err)
	}
	inspection, err := service.InspectProfileArchive(context.Background(), destination)
	if err != nil {
		t.Fatalf("InspectProfileArchive() error: %v", err)
	}
	if inspection.Profile.ID != p.ID || inspection.Profile.Connection.Password != "synthetic-password-123" {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}

func TestExportErrorsDoNotLeakPassword(t *testing.T) {
	repo := storage.NewRepository(t.TempDir())
	p := sharingProfile("broken-export", "Broken", "never-print-this-password")
	p.Scripts = []profile.Script{{ID: "missing", Name: "Missing", File: "scripts/missing.sql", Enabled: true, Order: 10}}
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	service := NewService(repo)
	err := service.ExportProfile(context.Background(), p.ID, filepath.Join(t.TempDir(), "out.zip"))
	if err == nil {
		t.Fatal("ExportProfile() expected error")
	}
	if strings.Contains(err.Error(), p.Connection.Password) {
		t.Fatalf("error leaked password: %q", err)
	}
}
