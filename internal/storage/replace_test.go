package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeDirFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceDirCompletelyReplacesExistingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "profiles", "demo")
	staged := filepath.Join(root, "imports", "staged")
	writeDirFile(t, target, "old.sql", "old")
	writeDirFile(t, staged, "new.sql", "new")

	if err := ReplaceDir(staged, target); err != nil {
		t.Fatalf("ReplaceDir() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "old.sql")); !os.IsNotExist(err) {
		t.Fatalf("old target content survived replacement: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "new.sql"))
	if err != nil || string(got) != "new" {
		t.Fatalf("new target content = %q, %v", got, err)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged directory still exists: %v", err)
	}
}

func TestReplaceDirMovesIntoAbsentTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "profiles", "demo")
	staged := filepath.Join(root, "imports", "staged")
	writeDirFile(t, staged, "new.sql", "new")
	if err := ReplaceDir(staged, target); err != nil {
		t.Fatalf("ReplaceDir() error: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "new.sql")); err != nil || string(got) != "new" {
		t.Fatalf("target content = %q, %v", got, err)
	}
}

func TestReplaceDirRestoresOldTargetWhenPublishRenameFails(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "profiles", "demo")
	staged := filepath.Join(root, "imports", "staged")
	writeDirFile(t, target, "old.sql", "old")
	writeDirFile(t, staged, "new.sql", "new")

	originalRename := rename
	calls := 0
	rename = func(oldPath, newPath string) error {
		calls++
		if calls == 2 {
			return errors.New("synthetic publish failure")
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() { rename = originalRename })

	if err := ReplaceDir(staged, target); err == nil {
		t.Fatal("ReplaceDir() expected error")
	}
	got, err := os.ReadFile(filepath.Join(target, "old.sql"))
	if err != nil || string(got) != "old" {
		t.Fatalf("old target was not restored: %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(staged, "new.sql")); err != nil || string(got) != "new" {
		t.Fatalf("staged content unexpectedly changed: %q, %v", got, err)
	}
}
