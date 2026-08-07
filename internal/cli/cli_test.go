package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func executeTest(t *testing.T, service Service, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := executeWithService(context.Background(), service, args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestProfileAndScriptCommandsUseRealStorage(t *testing.T) {
	service := app.NewService(storage.NewRepository(t.TempDir()))
	code, output, stderr := executeTest(t, service, "profile", "create", "--name", "Dev", "--host", "127.0.0.1", "--database", "demo", "--username", "root", "--password", "example")
	if code != 0 || stderr != "" {
		t.Fatalf("create code=%d stderr=%q", code, stderr)
	}
	start := strings.Index(output, "(")
	end := strings.Index(output, ")")
	if start < 0 || end <= start {
		t.Fatalf("could not parse profile id from %q", output)
	}
	profileID := output[start+1 : end]

	code, list, _ := executeTest(t, service, "profile", "list")
	if code != 0 || !strings.Contains(list, profileID+"\tDev") {
		t.Fatalf("list output=%q code=%d", list, code)
	}
	_, shown, _ := executeTest(t, service, "profile", "show", profileID)
	if strings.Contains(shown, "example") || !strings.Contains(shown, "***") {
		t.Fatalf("profile show did not redact password: %q", shown)
	}

	source := filepath.Join(t.TempDir(), "synthetic.sql")
	if err := os.WriteFile(source, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, added, _ := executeTest(t, service, "script", "add", profileID, source)
	if code != 0 || !strings.Contains(added, "Added script synthetic") {
		t.Fatalf("script add output=%q code=%d", added, code)
	}
	p, err := service.GetProfile(context.Background(), profileID)
	if err != nil || len(p.Scripts) != 1 {
		t.Fatalf("stored scripts=%#v err=%v", p.Scripts, err)
	}
	code, _, stderr = executeTest(t, service, "script", "remove", profileID, p.Scripts[0].ID)
	if code != 0 || stderr != "" {
		t.Fatalf("script remove code=%d stderr=%q", code, stderr)
	}
}

type fakeService struct {
	lastOptions executor.RunOptions
}

func (f *fakeService) CreateProfile(context.Context, profile.Profile) (profile.Profile, error) { return profile.Profile{}, nil }
func (f *fakeService) ListProfiles(context.Context) ([]profile.Profile, error) { return nil, nil }
func (f *fakeService) GetProfile(context.Context, string) (profile.Profile, error) { return profile.Profile{}, nil }
func (f *fakeService) AddScript(context.Context, string, string) (profile.Script, error) { return profile.Script{}, nil }
func (f *fakeService) RemoveScript(context.Context, string, string) error { return nil }
func (f *fakeService) TestConnection(context.Context, string) (database.ServerCapabilities, error) {
	return database.ServerCapabilities{Vendor: database.VendorMySQL, Major: 8, RawVersion: "8.0.43", VersionLabel: "MySQL 8.x"}, nil
}
func (f *fakeService) RunProfile(_ context.Context, _ string, opts executor.RunOptions, sink executor.Sink) (executor.Summary, error) {
	f.lastOptions = opts
	sink.Emit(executor.Event{Level: executor.LevelInfo, ScriptID: "one", Message: "Starting One"})
	sink.Emit(executor.Event{Level: executor.LevelWarn, ScriptID: "one", Message: "synthetic warning"})
	sink.Emit(executor.Event{Level: executor.LevelInfo, ScriptID: "one", Message: "Completed One"})
	return executor.Summary{Succeeded: 1}, nil
}
func (f *fakeService) InspectProfileArchive(context.Context, string) (profile.ArchiveInspection, error) { return profile.ArchiveInspection{}, nil }
func (f *fakeService) ExportProfile(context.Context, string, string) error { return nil }
func (f *fakeService) ImportProfile(context.Context, string, bool) (profile.Profile, error) { return profile.Profile{}, nil }

func TestConnectionAndRunCommands(t *testing.T) {
	service := &fakeService{}
	code, output, _ := executeTest(t, service, "connection", "test", "profile")
	if code != 0 || !strings.Contains(output, "MySQL 8.x (8.0.43)") {
		t.Fatalf("connection output=%q code=%d", output, code)
	}
	code, output, _ = executeTest(t, service, "run", "profile", "--stop-on-error")
	if code != 0 || service.lastOptions.OnError != profile.OnErrorStop || !strings.Contains(output, "✓ One") || !strings.Contains(output, "1 succeeded, 0 failed") {
		t.Fatalf("run output=%q code=%d opts=%#v", output, code, service.lastOptions)
	}
	code, output, _ = executeTest(t, service, "run", "profile", "--verbose", "--continue-on-error")
	if code != 0 || service.lastOptions.OnError != profile.OnErrorContinue || !strings.Contains(output, "INFO") || !strings.Contains(output, "WARN") {
		t.Fatalf("verbose output=%q code=%d", output, code)
	}
}

func TestRunFlagsAreMutuallyExclusive(t *testing.T) {
	code, _, stderr := executeTest(t, &fakeService{}, "run", "profile", "--stop-on-error", "--continue-on-error")
	if code == 0 || !strings.Contains(stderr, "stop-on-error") || !strings.Contains(stderr, "continue-on-error") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}
