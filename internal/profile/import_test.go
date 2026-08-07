package profile

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStageArchiveRoundTripFromExport(t *testing.T) {
	profileDir, want := createExportProfile(t)
	archivePath := filepath.Join(t.TempDir(), "profile.zip")
	if err := ExportArchive(profileDir, archivePath); err != nil {
		t.Fatalf("ExportArchive() error: %v", err)
	}
	stagingRoot := filepath.Join(t.TempDir(), ".imports")
	staged, inspection, err := StageArchive(archivePath, stagingRoot)
	if err != nil {
		t.Fatalf("StageArchive() error: %v", err)
	}
	if inspection.Profile.ID != want.ID {
		t.Fatalf("inspection profile = %#v", inspection.Profile)
	}
	profileBytes, err := os.ReadFile(filepath.Join(staged, "profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(profileBytes))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("staged profile mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	for _, script := range want.Scripts {
		original, err := os.ReadFile(filepath.Join(profileDir, filepath.FromSlash(script.File)))
		if err != nil {
			t.Fatal(err)
		}
		stagedBytes, err := os.ReadFile(filepath.Join(staged, filepath.FromSlash(script.File)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(original, stagedBytes) {
			t.Fatalf("script %s changed during round trip", script.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(staged, "manifest.yaml")); !os.IsNotExist(err) {
		t.Fatalf("transport manifest should not enter canonical profile directory: %v", err)
	}
}

func TestStageArchiveRejectsMaliciousArchiveBeforeCreatingStagingFiles(t *testing.T) {
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
			archivePath := writeArchiveForTest(t, entries)
			root := t.TempDir()
			stagingRoot := filepath.Join(root, ".imports")
			if _, _, err := StageArchive(archivePath, stagingRoot); err == nil {
				t.Fatalf("StageArchive() accepted malicious entry %q", name)
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("invalid archive created staging content: entries=%v err=%v", entries, err)
			}
		})
	}
}
