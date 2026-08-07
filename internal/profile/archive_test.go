package profile

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type archiveTestEntry struct {
	name string
	data string
	mode os.FileMode
}

func validArchiveEntries(t *testing.T) []archiveTestEntry {
	t.Helper()
	p := validProfile()
	p.ID = "profile-archive"
	p.Name = "Synthetic Archive"
	p.Scripts = []Script{
		{ID: "script-a", Name: "A", File: "scripts/script-a.sql", Enabled: true, Order: 10},
		{ID: "script-b", Name: "B", File: "scripts/script-b.sql", Enabled: false, Order: 20},
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, p); err != nil {
		t.Fatalf("Encode() error: %v", err)
	}
	return []archiveTestEntry{
		{name: "manifest.yaml", data: "format_version: 1\nprofile_id: profile-archive\nprofile_name: Synthetic Archive\n"},
		{name: "profile.yaml", data: encoded.String()},
		{name: "scripts/script-a.sql", data: "SELECT 1;"},
		{name: "scripts/script-b.sql", data: "SELECT 2;"},
	}
}

func writeArchiveForTest(t *testing.T, entries []archiveTestEntry) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "profile.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func removeEntry(entries []archiveTestEntry, name string) []archiveTestEntry {
	out := make([]archiveTestEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.name != name {
			out = append(out, entry)
		}
	}
	return out
}

func TestInspectArchiveAcceptsValidArchive(t *testing.T) {
	archivePath := writeArchiveForTest(t, validArchiveEntries(t))
	inspection, err := InspectArchive(archivePath)
	if err != nil {
		t.Fatalf("InspectArchive() error: %v", err)
	}
	if inspection.Manifest.FormatVersion != 1 || inspection.Manifest.ProfileID != "profile-archive" {
		t.Fatalf("unexpected manifest: %#v", inspection.Manifest)
	}
	if inspection.Profile.ID != "profile-archive" || len(inspection.Profile.Scripts) != 2 {
		t.Fatalf("unexpected profile: %#v", inspection.Profile)
	}
}

func TestInspectArchiveRejectsRequiredStructureProblems(t *testing.T) {
	tests := []struct {
		name    string
		entries func(*testing.T) []archiveTestEntry
	}{
		{"missing manifest", func(t *testing.T) []archiveTestEntry { return removeEntry(validArchiveEntries(t), "manifest.yaml") }},
		{"missing profile", func(t *testing.T) []archiveTestEntry { return removeEntry(validArchiveEntries(t), "profile.yaml") }},
		{"duplicate manifest", func(t *testing.T) []archiveTestEntry {
			entries := validArchiveEntries(t)
			return append(entries, entries[0])
		}},
		{"unsupported version", func(t *testing.T) []archiveTestEntry {
			entries := validArchiveEntries(t)
			entries[0].data = strings.Replace(entries[0].data, "format_version: 1", "format_version: 2", 1)
			return entries
		}},
		{"id mismatch", func(t *testing.T) []archiveTestEntry {
			entries := validArchiveEntries(t)
			entries[0].data = strings.Replace(entries[0].data, "profile_id: profile-archive", "profile_id: other-profile", 1)
			return entries
		}},
		{"missing referenced script", func(t *testing.T) []archiveTestEntry { return removeEntry(validArchiveEntries(t), "scripts/script-b.sql") }},
		{"unexpected file", func(t *testing.T) []archiveTestEntry {
			return append(validArchiveEntries(t), archiveTestEntry{name: "notes.txt", data: "nope"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := InspectArchive(writeArchiveForTest(t, tt.entries(t))); err == nil {
				t.Fatal("InspectArchive() expected error")
			}
		})
	}
}

func TestInspectArchiveRejectsUnsafePathsAndSymlinks(t *testing.T) {
	unsafeNames := []string{
		"../outside.sql",
		"scripts/../../outside.sql",
		"/absolute.sql",
		"C:/outside.sql",
		`scripts\..\outside.sql`,
	}
	for _, name := range unsafeNames {
		t.Run(name, func(t *testing.T) {
			entries := validArchiveEntries(t)
			entries = append(entries, archiveTestEntry{name: name, data: "SELECT 1;"})
			if _, err := InspectArchive(writeArchiveForTest(t, entries)); err == nil {
				t.Fatalf("InspectArchive() accepted unsafe entry %q", name)
			}
		})
	}

	t.Run("symlink", func(t *testing.T) {
		entries := validArchiveEntries(t)
		entries = append(entries, archiveTestEntry{name: "link", data: "target", mode: os.ModeSymlink | 0o777})
		if _, err := InspectArchive(writeArchiveForTest(t, entries)); err == nil {
			t.Fatal("InspectArchive() accepted symlink")
		}
	})
}

func TestInspectArchiveRequiresCanonicalScriptPath(t *testing.T) {
	entries := validArchiveEntries(t)
	entries[1].data = strings.Replace(entries[1].data, "file: scripts/script-a.sql", "file: scripts/custom.sql", 1)
	entries = append(entries, archiveTestEntry{name: "scripts/custom.sql", data: "SELECT 1;"})
	if _, err := InspectArchive(writeArchiveForTest(t, entries)); err == nil {
		t.Fatal("InspectArchive() accepted non-canonical script path")
	}
}

func TestInspectArchiveRejectsUnknownManifestFields(t *testing.T) {
	entries := validArchiveEntries(t)
	entries[0].data += "unknown: true\n"
	if _, err := InspectArchive(writeArchiveForTest(t, entries)); err == nil {
		t.Fatal("InspectArchive() accepted unknown manifest field")
	}
}
