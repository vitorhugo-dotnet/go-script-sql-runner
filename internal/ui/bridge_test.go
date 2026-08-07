package ui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/app"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type fakeDialogs struct {
	sqlFile      string
	zipFile      string
	saveFile     string
	confirm      bool
	confirmCalls int
	defaultName  string
}

func (d *fakeDialogs) OpenSQLFile(context.Context) (string, error) { return d.sqlFile, nil }
func (d *fakeDialogs) OpenProfileZIP(context.Context) (string, error) { return d.zipFile, nil }
func (d *fakeDialogs) SaveProfileZIP(_ context.Context, defaultName string) (string, error) {
	d.defaultName = defaultName
	return d.saveFile, nil
}
func (d *fakeDialogs) ConfirmOverwrite(_ context.Context, _ string) (bool, error) {
	d.confirmCalls++
	return d.confirm, nil
}

type fakeEvents struct {
	mu       sync.Mutex
	events   []executor.Event
	finished []executor.Summary
}

func (e *fakeEvents) EmitExecutionEvent(_ context.Context, event executor.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
}
func (e *fakeEvents) EmitExecutionFinished(_ context.Context, summary executor.Summary) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.finished = append(e.finished, summary)
}

type fakeService struct {
	profiles       []profile.Profile
	archive        profile.ArchiveInspection
	importConflict bool
	imports        []bool
	exportedTo     string
	addedPath      string
	addedPaths     []string
	runStarted     chan struct{}
}

func (s *fakeService) CreateProfile(_ context.Context, p profile.Profile) (profile.Profile, error) { return p, nil }
func (s *fakeService) ListProfiles(context.Context) ([]profile.Profile, error) { return s.profiles, nil }
func (s *fakeService) GetProfile(_ context.Context, id string) (profile.Profile, error) {
	for _, p := range s.profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return profile.Profile{}, errors.New("not found")
}
func (s *fakeService) UpdateProfile(_ context.Context, p profile.Profile) (profile.Profile, error) { return p, nil }
func (s *fakeService) AddScript(_ context.Context, _ string, source string) (profile.Script, error) {
	s.addedPath = source
	s.addedPaths = append(s.addedPaths, source)
	return profile.Script{ID: "added", Name: "added", File: "scripts/added.sql", Enabled: true, Order: 10}, nil
}
func (s *fakeService) RemoveScript(context.Context, string, string) error { return nil }
func (s *fakeService) ReorderScripts(_ context.Context, id string, _ []string) (profile.Profile, error) { return s.GetProfile(context.Background(), id) }
func (s *fakeService) SetScriptEnabled(_ context.Context, id, _ string, _ bool) (profile.Profile, error) { return s.GetProfile(context.Background(), id) }
func (s *fakeService) SetScriptTransactionMode(_ context.Context, id, _ string, _ profile.TransactionMode) (profile.Profile, error) { return s.GetProfile(context.Background(), id) }
func (s *fakeService) TestConnection(context.Context, string) (database.ServerCapabilities, error) {
	return database.ServerCapabilities{Vendor: database.VendorMySQL, Major: 8, Minor: 0, Patch: 43, RawVersion: "8.0.43", VersionLabel: "MySQL 8.x"}, nil
}
func (s *fakeService) RunProfile(ctx context.Context, _ string, _ executor.RunOptions, sink executor.Sink) (executor.Summary, error) {
	if s.runStarted != nil {
		close(s.runStarted)
	}
	sink.Emit(executor.Event{Time: time.Now(), Level: executor.LevelInfo, ScriptID: "one", Message: "Starting One"})
	if s.runStarted != nil {
		<-ctx.Done()
		return executor.Summary{Aborted: true}, ctx.Err()
	}
	sink.Emit(executor.Event{Time: time.Now(), Level: executor.LevelError, ScriptID: "one", Message: "Failed One", Detail: "authentication failed: ***"})
	return executor.Summary{Failed: 1}, nil
}
func (s *fakeService) InspectProfileArchive(context.Context, string) (profile.ArchiveInspection, error) { return s.archive, nil }
func (s *fakeService) ImportProfile(_ context.Context, _ string, overwrite bool) (profile.Profile, error) {
	s.imports = append(s.imports, overwrite)
	if s.importConflict && !overwrite {
		return profile.Profile{}, &app.ProfileConflictError{Existing: profile.Profile{ID: s.archive.Profile.ID, Name: "Old"}, Incoming: s.archive.Profile}
	}
	return s.archive.Profile, nil
}
func (s *fakeService) ExportProfile(_ context.Context, _ string, destination string) error { s.exportedTo = destination; return nil }

func TestDialogCancellationDoesNothing(t *testing.T) {
	service := &fakeService{profiles: []profile.Profile{{ID: "p", Name: "Profile"}}}
	dialogs := &fakeDialogs{}
	bridge := NewBridge(service, dialogs, nil)
	if script, err := bridge.AddScriptFromDialog(context.Background(), "p"); err != nil || script != nil {
		t.Fatalf("cancelled SQL dialog = %#v, %v", script, err)
	}
	if imported, err := bridge.ImportProfileFromDialog(context.Background()); err != nil || imported != nil {
		t.Fatalf("cancelled import = %#v, %v", imported, err)
	}
	if destination, err := bridge.ExportProfileToDialog(context.Background(), "p"); err != nil || destination != "" {
		t.Fatalf("cancelled export = %q, %v", destination, err)
	}
}

func TestDialogOperationsUseSelectedFiles(t *testing.T) {
	p := profile.Profile{ID: "profile-id", Name: "Profile"}
	service := &fakeService{profiles: []profile.Profile{p}, archive: profile.ArchiveInspection{Profile: p}}
	dialogs := &fakeDialogs{sqlFile: `C:\temp\setup.sql`, zipFile: `C:\temp\profile.zip`, saveFile: `C:\temp\out.zip`}
	bridge := NewBridge(service, dialogs, nil)
	script, err := bridge.AddScriptFromDialog(context.Background(), p.ID)
	if err != nil || script == nil || service.addedPath != dialogs.sqlFile {
		t.Fatalf("AddScriptFromDialog() script=%#v path=%q err=%v", script, service.addedPath, err)
	}
	if _, err := bridge.ImportProfileFromDialog(context.Background()); err != nil {
		t.Fatalf("ImportProfileFromDialog() error: %v", err)
	}
	destination, err := bridge.ExportProfileToDialog(context.Background(), p.ID)
	if err != nil || destination != dialogs.saveFile || service.exportedTo != dialogs.saveFile || dialogs.defaultName != p.ID+".zip" {
		t.Fatalf("ExportProfileToDialog() destination=%q exported=%q default=%q err=%v", destination, service.exportedTo, dialogs.defaultName, err)
	}
}

func TestImportConflictRequiresExplicitConfirmation(t *testing.T) {
	incoming := profile.Profile{ID: "same-id", Name: "Incoming"}
	service := &fakeService{archive: profile.ArchiveInspection{Profile: incoming}, importConflict: true}
	dialogs := &fakeDialogs{zipFile: `C:\temp\profile.zip`, confirm: false}
	bridge := NewBridge(service, dialogs, nil)
	imported, err := bridge.ImportProfileFromDialog(context.Background())
	if err != nil || imported != nil || dialogs.confirmCalls != 1 || len(service.imports) != 1 || service.imports[0] {
		t.Fatalf("declined conflict imported=%#v calls=%d imports=%#v err=%v", imported, dialogs.confirmCalls, service.imports, err)
	}
	dialogs.confirm = true
	imported, err = bridge.ImportProfileFromDialog(context.Background())
	if err != nil || imported == nil || imported.Name != "Incoming" || len(service.imports) != 3 || !service.imports[2] {
		t.Fatalf("confirmed conflict imported=%#v imports=%#v err=%v", imported, service.imports, err)
	}
}

func TestRunStreamsRedactedEventsAndFinishedSummary(t *testing.T) {
	service := &fakeService{}
	events := &fakeEvents{}
	bridge := NewBridge(service, &fakeDialogs{}, events)
	summary, err := bridge.RunProfile(context.Background(), "p", executor.RunOptions{})
	if err != nil || summary.Failed != 1 {
		t.Fatalf("RunProfile() summary=%#v err=%v", summary, err)
	}
	if len(events.events) != 2 || len(events.finished) != 1 {
		t.Fatalf("events=%#v finished=%#v", events.events, events.finished)
	}
	for _, event := range events.events {
		if strings.Contains(event.Detail, "synthetic-password") {
			t.Fatalf("event leaked password: %#v", event)
		}
	}
}

func TestStopRunCancelsActiveExecution(t *testing.T) {
	started := make(chan struct{})
	service := &fakeService{runStarted: started}
	bridge := NewBridge(service, &fakeDialogs{}, &fakeEvents{})
	done := make(chan error, 1)
	go func() {
		_, err := bridge.RunProfile(context.Background(), "p", executor.RunOptions{})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("run did not start")
	}
	if !bridge.StopRun() {
		t.Fatal("StopRun() returned false while running")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop")
	}
	if bridge.StopRun() {
		t.Fatal("StopRun() returned true with no active run")
	}
}
