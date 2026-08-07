package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func cliProfile(id, name, password string) profile.Profile {
	return profile.Profile{
		ID: id, Name: name, Version: 1,
		Connection: profile.Connection{Host: "127.0.0.1", Port: 3306, Database: "synthetic", Username: "dev", Password: password},
		Execution: profile.Execution{OnError: profile.OnErrorContinue, TransactionMode: profile.TransactionAutoCommit},
	}
}

func createCLIServiceWithScript(t *testing.T, root string, p profile.Profile, scriptID, sqlText string) *app.Service {
	t.Helper()
	repo := storage.NewRepository(root)
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), scriptID+".sql")
	if err := os.WriteFile(source, []byte(sqlText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddScript(p.ID, profile.Script{ID: scriptID, Name: scriptID, Enabled: true, Order: 10}, source); err != nil {
		t.Fatal(err)
	}
	return app.NewService(repo)
}

func executeRealCLI(t *testing.T, service *app.Service, stdin io.Reader, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), service, args, stdin, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestProfileExportWarnsBeforeSuccessAndKeepsCredentialsInsideArchive(t *testing.T) {
	p := cliProfile("export-cli", "Export CLI", "synthetic-password-123")
	service := createCLIServiceWithScript(t, t.TempDir(), p, "script-one", "SELECT 1;")
	destination := filepath.Join(t.TempDir(), "profile.zip")
	code, stdout, stderr := executeRealCLI(t, service, strings.NewReader(""), "profile", "export", p.ID, destination)
	if code != 0 {
		t.Fatalf("export code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, plaintextCredentialsWarning) || !strings.Contains(stdout, destination) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
	inspection, err := profile.InspectArchive(destination)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Profile.Connection.Password != "synthetic-password-123" {
		t.Fatal("exported archive did not retain credentials")
	}
	if strings.Contains(stdout+stderr, "synthetic-password-123") {
		t.Fatal("CLI output leaked password")
	}
}

func TestProfileImportConflictNoLeavesExistingProfileUntouched(t *testing.T) {
	root := t.TempDir()
	existing := cliProfile("shared-cli", "Existing CLI", "old-password")
	target := createCLIServiceWithScript(t, root, existing, "old-script", "SELECT 'old';")
	incomingService := createCLIServiceWithScript(t, t.TempDir(), cliProfile(existing.ID, "Incoming CLI", "new-password"), "new-script", "SELECT 'new';")
	archive := filepath.Join(t.TempDir(), "incoming.zip")
	if err := incomingService.ExportProfile(context.Background(), existing.ID, archive); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := executeRealCLI(t, target, strings.NewReader("n\n"), "profile", "import", archive)
	if code != 0 || !strings.Contains(stderr, `Profile "Incoming CLI" already exists and will be completely overwritten. Continue? [y/N]:`) || !strings.Contains(stdout, "Import cancelled.") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stored, err := target.GetProfile(context.Background(), existing.ID)
	if err != nil || stored.Name != "Existing CLI" || len(stored.Scripts) != 1 || stored.Scripts[0].ID != "old-script" {
		t.Fatalf("existing profile changed: %#v err=%v", stored, err)
	}
}

func TestProfileImportConflictYesOverwritesCompletely(t *testing.T) {
	root := t.TempDir()
	existing := cliProfile("shared-cli", "Existing CLI", "old-password")
	target := createCLIServiceWithScript(t, root, existing, "old-script", "SELECT 'old';")
	incomingService := createCLIServiceWithScript(t, t.TempDir(), cliProfile(existing.ID, "Incoming CLI", "new-password"), "new-script", "SELECT 'new';")
	archive := filepath.Join(t.TempDir(), "incoming.zip")
	if err := incomingService.ExportProfile(context.Background(), existing.ID, archive); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := executeRealCLI(t, target, strings.NewReader("yes\n"), "profile", "import", archive)
	if code != 0 || !strings.Contains(stdout, "Imported profile Incoming CLI") || !strings.Contains(stderr, "will be completely overwritten") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stored, err := target.GetProfile(context.Background(), existing.ID)
	if err != nil || stored.Name != "Incoming CLI" || stored.Connection.Password != "new-password" || len(stored.Scripts) != 1 || stored.Scripts[0].ID != "new-script" {
		t.Fatalf("profile was not overwritten: %#v err=%v", stored, err)
	}
}

type failIfRead struct{}

func (failIfRead) Read([]byte) (int, error) { return 0, errors.New("stdin must not be read with --force") }

func TestProfileImportForceOverwritesWithoutReadingStdin(t *testing.T) {
	root := t.TempDir()
	existing := cliProfile("force-cli", "Existing", "old")
	target := createCLIServiceWithScript(t, root, existing, "old-script", "SELECT 'old';")
	incomingService := createCLIServiceWithScript(t, t.TempDir(), cliProfile(existing.ID, "Forced", "new"), "new-script", "SELECT 'new';")
	archive := filepath.Join(t.TempDir(), "incoming.zip")
	if err := incomingService.ExportProfile(context.Background(), existing.ID, archive); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := executeRealCLI(t, target, failIfRead{}, "profile", "import", archive, "--force")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Imported profile Forced") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stored, err := target.GetProfile(context.Background(), existing.ID)
	if err != nil || stored.Name != "Forced" {
		t.Fatalf("force import failed: %#v err=%v", stored, err)
	}
}
