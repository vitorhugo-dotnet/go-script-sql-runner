package profile

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func createExportProfile(t *testing.T) (string, Profile) {
	t.Helper()
	profileDir := filepath.Join(t.TempDir(), "profiles", "export-profile")
	if err := os.MkdirAll(filepath.Join(profileDir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	p := validProfile()
	p.ID = "export-profile"
	p.Name = "Synthetic Export"
	p.Connection.Host = "db.synthetic.internal"
	p.Connection.Database = "synthetic_database"
	p.Connection.Username = "synthetic_user"
	p.Connection.Password = "synthetic-password-123"
	p.Scripts = []Script{
		{ID: "script-b", Name: "B", File: "scripts/script-b.sql", Enabled: true, Order: 20},
		{ID: "script-a", Name: "A", File: "scripts/script-a.sql", Enabled: true, Order: 10},
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "profile.yaml"), encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "scripts", "script-a.sql"), []byte("SELECT 'A';"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "scripts", "script-b.sql"), []byte("SELECT 'B';"), 0o600); err != nil {
		t.Fatal(err)
	}
	return profileDir, p
}

func TestExportArchiveIsSelfContainedAndDeterministic(t *testing.T) {
	profileDir, wantProfile := createExportProfile(t)
	first := filepath.Join(t.TempDir(), "first.zip")
	second := filepath.Join(t.TempDir(), "second.zip")
	if err := ExportArchive(profileDir, first); err != nil {
		t.Fatalf("ExportArchive(first) error: %v", err)
	}
	if err := ExportArchive(profileDir, second); err != nil {
		t.Fatalf("ExportArchive(second) error: %v", err)
	}
	firstBytes, _ := os.ReadFile(first)
	secondBytes, _ := os.ReadFile(second)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("same profile produced non-deterministic ZIP bytes")
	}

	reader, err := zip.OpenReader(first)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	contents := map[string]string{}
	for _, entry := range reader.File {
		names = append(names, entry.Name)
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buffer bytes.Buffer
		if _, err := buffer.ReadFrom(file); err != nil {
			file.Close()
			t.Fatal(err)
		}
		file.Close()
		contents[entry.Name] = buffer.String()
	}
	wantNames := []string{"manifest.yaml", "profile.yaml", "scripts/script-a.sql", "scripts/script-b.sql"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("archive order = %#v, want %#v", names, wantNames)
	}
	inspection, err := InspectArchive(first)
	if err != nil {
		t.Fatalf("InspectArchive(export) error: %v", err)
	}
	got := inspection.Profile.Connection
	want := wantProfile.Connection
	if got.Host != want.Host || got.Port != want.Port || got.Database != want.Database || got.Username != want.Username || got.Password != want.Password {
		t.Fatalf("connection changed during export: %#v", got)
	}
	if contents["scripts/script-a.sql"] != "SELECT 'A';" || contents["scripts/script-b.sql"] != "SELECT 'B';" {
		t.Fatalf("script contents changed: %#v", contents)
	}
	if strings.Contains(contents["profile.yaml"], profileDir) || strings.Contains(string(firstBytes), filepath.Dir(profileDir)) {
		t.Fatal("archive leaked an absolute AppData/source path")
	}
}

func TestExportArchiveReplacesExistingDestinationOnlyAfterSuccessfulWrite(t *testing.T) {
	profileDir, _ := createExportProfile(t)
	destination := filepath.Join(t.TempDir(), "profile.zip")
	if err := os.WriteFile(destination, []byte("previous archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ExportArchive(profileDir, destination); err != nil {
		t.Fatalf("ExportArchive() error: %v", err)
	}
	if _, err := InspectArchive(destination); err != nil {
		t.Fatalf("replacement is not a valid archive: %v", err)
	}
}

func TestExportArchiveFailureLeavesExistingDestinationIntact(t *testing.T) {
	profileDir, _ := createExportProfile(t)
	destination := filepath.Join(t.TempDir(), "profile.zip")
	old := []byte("previous archive must survive")
	if err := os.WriteFile(destination, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(profileDir, "scripts", "script-b.sql")); err != nil {
		t.Fatal(err)
	}
	if err := ExportArchive(profileDir, destination); err == nil {
		t.Fatal("ExportArchive() expected missing-script error")
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Fatalf("failed export changed destination: %q", got)
	}
	if _, err := os.Stat(destination + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary export was not removed: %v", err)
	}
}
