package ui

import (
	"context"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type multiSQLDialogs struct {
	*fakeDialogs
	files []string
}

func (d *multiSQLDialogs) OpenSQLFiles(context.Context) ([]string, error) {
	return d.files, nil
}

func TestAddScriptsFromDialogAddsSelectedFilesInOrder(t *testing.T) {
	service := &fakeService{profiles: []profile.Profile{{ID: "p", Name: "Profile"}}}
	dialogs := &multiSQLDialogs{
		fakeDialogs: &fakeDialogs{},
		files:       []string{`C:\temp\001-schema.sql`, `C:\temp\002-data.sql`},
	}
	bridge := NewBridge(service, dialogs, nil)

	scripts, err := bridge.AddScriptsFromDialog(context.Background(), "p")
	if err != nil {
		t.Fatalf("AddScriptsFromDialog() error: %v", err)
	}
	if len(scripts) != 2 {
		t.Fatalf("AddScriptsFromDialog() added %d scripts, want 2", len(scripts))
	}
	if len(service.addedPaths) != 2 || service.addedPaths[0] != dialogs.files[0] || service.addedPaths[1] != dialogs.files[1] {
		t.Fatalf("added paths = %#v, want %#v", service.addedPaths, dialogs.files)
	}
}

func TestAddScriptsFromDialogCancellationAddsNothing(t *testing.T) {
	service := &fakeService{profiles: []profile.Profile{{ID: "p", Name: "Profile"}}}
	dialogs := &multiSQLDialogs{fakeDialogs: &fakeDialogs{}, files: nil}
	bridge := NewBridge(service, dialogs, nil)

	scripts, err := bridge.AddScriptsFromDialog(context.Background(), "p")
	if err != nil || len(scripts) != 0 || len(service.addedPaths) != 0 {
		t.Fatalf("cancelled multi SQL dialog scripts=%#v paths=%#v err=%v", scripts, service.addedPaths, err)
	}
}
