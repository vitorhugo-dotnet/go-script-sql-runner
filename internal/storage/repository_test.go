package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func scriptContentRepository(t *testing.T) (*Repository, profile.Profile, string) {
	t.Helper()
	repo := NewRepository(t.TempDir())
	p := testProfile()
	p.Scripts = []profile.Script{{ID: "one", Name: "One", File: "scripts/one.sql", Enabled: true, Order: 10}}
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.ScriptRoot(p.ID), "one.sql")
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo, p, path
}

func TestScriptContentRoundTripPreservesMetadata(t *testing.T) {
	repo, before, path := scriptContentRepository(t)
	metadataBefore, err := os.ReadFile(filepath.Join(repo.ProfileDir(before.ID), "profile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ReadScriptContent(before.ID, "one")
	if err != nil || got != "SELECT 1;\n" {
		t.Fatalf("ReadScriptContent() = %q, %v", got, err)
	}
	for _, content := range []string{"-- Café\nSELECT 2;\n", ""} {
		if err := repo.WriteScriptContent(before.ID, "one", content); err != nil {
			t.Fatalf("WriteScriptContent(%q): %v", content, err)
		}
		got, err := repo.ReadScriptContent(before.ID, "one")
		if err != nil || got != content {
			t.Fatalf("ReadScriptContent() = %q, %v; want %q", got, err, content)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			t.Fatalf("SQL bytes = %q, %v; want %q", data, err, content)
		}
		metadataAfter, err := os.ReadFile(filepath.Join(repo.ProfileDir(before.ID), "profile.yaml"))
		if err != nil || !bytes.Equal(metadataAfter, metadataBefore) {
			t.Fatalf("metadata changed after content write: %v", err)
		}
		stored, err := repo.Get(before.ID)
		if err != nil || !reflect.DeepEqual(stored, before) {
			t.Fatalf("profile changed after content write: %#v, %v", stored, err)
		}
	}
}

func TestScriptContentRejectsUnknownOrInvalidReferences(t *testing.T) {
	repo, p, path := scriptContentRepository(t)
	for _, tc := range []struct{ profileID, scriptID string }{
		{"missing-profile", "one"}, {p.ID, ""}, {p.ID, "missing"},
	} {
		if _, err := repo.ReadScriptContent(tc.profileID, tc.scriptID); err == nil {
			t.Fatalf("ReadScriptContent(%q, %q) succeeded", tc.profileID, tc.scriptID)
		}
		if err := repo.WriteScriptContent(tc.profileID, tc.scriptID, "changed"); err == nil {
			t.Fatalf("WriteScriptContent(%q, %q) succeeded", tc.profileID, tc.scriptID)
		}
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "SELECT 1;\n" {
		t.Fatalf("original SQL changed: %q, %v", data, err)
	}

	metadataPath := filepath.Join(repo.ProfileDir(p.ID), "profile.yaml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	malformed := bytes.Replace(metadata, []byte("scripts/one.sql"), []byte("../escape.sql"), 1)
	if bytes.Equal(malformed, metadata) {
		t.Fatal("could not mutate fixture metadata")
	}
	if err := os.WriteFile(metadataPath, malformed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReadScriptContent(p.ID, "one"); err == nil {
		t.Fatal("ReadScriptContent accepted escaping metadata")
	}
	if err := repo.WriteScriptContent(p.ID, "one", "changed"); err == nil {
		t.Fatal("WriteScriptContent accepted escaping metadata")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "SELECT 1;\n" {
		t.Fatalf("original SQL changed: %q, %v", data, err)
	}
}

func TestScriptContentMissingFileIsNotRecreated(t *testing.T) {
	repo, p, filename := scriptContentRepository(t)
	metadataPath := filepath.Join(repo.ProfileDir(p.ID), "profile.yaml")
	metadataBefore, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReadScriptContent(p.ID, "one"); err == nil {
		t.Fatal("ReadScriptContent succeeded after SQL file removal")
	}
	if err := repo.WriteScriptContent(p.ID, "one", "replacement"); err == nil {
		t.Fatal("WriteScriptContent recreated the removed SQL file")
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		t.Fatalf("removed SQL file was recreated: %v", err)
	}
	metadataAfter, err := os.ReadFile(metadataPath)
	if err != nil || !bytes.Equal(metadataAfter, metadataBefore) {
		t.Fatalf("metadata changed after missing-file operations: %v", err)
	}
}

func TestScriptContentAtomicWriteFailurePreservesData(t *testing.T) {
	repo, p, filename := scriptContentRepository(t)
	metadataPath := filepath.Join(repo.ProfileDir(p.ID), "profile.yaml")
	metadataBefore, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	originalSQL, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	renameFailure := errors.New("injected rename failure")
	originalRename := atomicRename
	atomicRename = func(source, destination string) error {
		if destination != filename {
			return originalRename(source, destination)
		}
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("atomic temporary file missing before rename: %v", err)
		}
		return renameFailure
	}
	t.Cleanup(func() { atomicRename = originalRename })
	if err := repo.WriteScriptContent(p.ID, "one", "replacement"); !errors.Is(err, renameFailure) {
		t.Fatalf("WriteScriptContent() error = %v, want injected rename failure", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(data, originalSQL) {
		t.Fatalf("original SQL changed after failed repository save: %q, %v", data, err)
	}
	metadataAfter, err := os.ReadFile(metadataPath)
	if err != nil || !bytes.Equal(metadataAfter, metadataBefore) {
		t.Fatalf("metadata changed after failed atomic write: %v", err)
	}
	entries, err := os.ReadDir(repo.ScriptRoot(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".runner-") {
			t.Fatalf("temporary atomic-write file remains: %q", entry.Name())
		}
	}
}

func TestScriptContentRejectsSymlinksAndNonRegularFiles(t *testing.T) {
	repo, p, path := scriptContentRepository(t)
	outside := filepath.Join(t.TempDir(), "outside.sql")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Logf("symlinks unavailable: %v", err)
	} else {
		if _, err := repo.ReadScriptContent(p.ID, "one"); err == nil {
			t.Fatal("ReadScriptContent followed a symlink")
		}
		if err := repo.WriteScriptContent(p.ID, "one", "changed"); err == nil {
			t.Fatal("WriteScriptContent replaced a symlink")
		}
		if data, err := os.ReadFile(outside); err != nil || string(data) != "outside" {
			t.Fatalf("outside file changed: %q, %v", data, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReadScriptContent(p.ID, "one"); err == nil {
		t.Fatal("ReadScriptContent accepted a directory")
	}
	if err := repo.WriteScriptContent(p.ID, "one", "changed"); err == nil {
		t.Fatal("WriteScriptContent accepted a directory")
	}
}

func TestScriptContentRejectsSymlinkedDirectory(t *testing.T) {
	repo, p, _ := scriptContentRepository(t)
	p.Scripts[0].File = "scripts/nested/one.sql"
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "one.sql")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo.ScriptRoot(p.ID), "nested")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := repo.ReadScriptContent(p.ID, "one"); err == nil {
		t.Fatal("ReadScriptContent followed a symlinked directory")
	}
	if err := repo.WriteScriptContent(p.ID, "one", "changed"); err == nil {
		t.Fatal("WriteScriptContent followed a symlinked directory")
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "outside" {
		t.Fatalf("outside file changed: %q, %v", data, err)
	}
}

func testProfile() profile.Profile {
	return profile.Profile{
		ID:      "profile-1",
		Name:    "Synthetic Profile",
		Version: 1,
		Connection: profile.Connection{
			Host: "127.0.0.1", Port: 3306, Database: "example", Username: "dev", Password: "secret",
		},
		Execution: profile.Execution{OnError: profile.OnErrorContinue, TransactionMode: profile.TransactionAutoCommit},
	}
}

func TestRepositoryCRUDAndLayout(t *testing.T) {
	root := t.TempDir()
	repo := NewRepository(root)
	p := testProfile()
	if err := repo.Save(p); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	other := p
	other.ID = "profile-2"
	other.Name = "Other Profile"
	if err := repo.Save(other); err != nil {
		t.Fatalf("Save(other) error: %v", err)
	}
	for _, path := range []string{
		filepath.Join(root, "profiles", p.ID, "profile.yaml"),
		filepath.Join(root, "profiles", p.ID, "scripts"),
		filepath.Join(root, "logs"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}

	got, err := repo.Get(p.ID)
	if err != nil || got.Name != p.Name {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	list, err := repo.List()
	if err != nil || len(list) != 2 || list[0].ID != other.ID || list[1].ID != p.ID {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if _, err := repo.Get(p.ID); err != ErrNotFound {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(repo.ProfileDir(p.ID)); !os.IsNotExist(err) {
		t.Fatalf("deleted profile directory remains: %v", err)
	}
	if got, err := repo.Get(other.ID); err != nil || got.ID != other.ID {
		t.Fatalf("other profile after delete = %#v, %v", got, err)
	}
}

func TestDeleteRejectsInvalidIDs(t *testing.T) {
	root := t.TempDir()
	repo := NewRepository(root)
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	p := testProfile()
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../outside", ".", "", p.ID + ".", p.ID + " ", " " + p.ID, "CON", "nul.txt", "LPT1"} {
		t.Run(id, func(t *testing.T) {
			if err := repo.Delete(id); err == nil {
				t.Fatalf("Delete(%q) succeeded", id)
			}
			for _, dir := range []string{outside, repo.ProfileDir(p.ID)} {
				if _, err := os.Stat(dir); err != nil {
					t.Fatalf("Delete(%q) changed %q: %v", id, dir, err)
				}
			}
		})
	}
}

func TestRepositoryCloneRejectsWindowsAliasIDs(t *testing.T) {
	repo := NewRepository(t.TempDir())
	source := testProfile()
	if err := repo.Save(source); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{source.ID + ".", source.ID + " ", " " + source.ID} {
		t.Run(alias, func(t *testing.T) {
			invalid := source
			invalid.ID = alias
			if err := repo.Save(invalid); err == nil || !strings.Contains(err.Error(), "invalid profile id") {
				t.Fatalf("Save(alias %q) error = %v, want invalid profile id", alias, err)
			}
			clone := source
			clone.ID = "copy-1"
			if err := repo.Clone(alias, clone); err == nil || !strings.Contains(err.Error(), "invalid profile id") {
				t.Fatalf("Clone(alias source %q) error = %v, want invalid profile id", alias, err)
			}
			clone.ID = alias
			if err := repo.Clone(source.ID, clone); err == nil || !strings.Contains(err.Error(), "invalid profile id") {
				t.Fatalf("Clone(alias destination %q) error = %v, want invalid profile id", alias, err)
			}
			if got, err := repo.Get(source.ID); err != nil || got.ID != source.ID {
				t.Fatalf("genuine profile changed after clone aliases: %#v, %v", got, err)
			}
			if _, err := os.Stat(repo.ProfileDir("copy-1")); !os.IsNotExist(err) {
				t.Fatalf("clone was published from alias: %v", err)
			}
		})
	}
}

func TestRepositoryRejectsCaseAliasIdentity(t *testing.T) {
	repo := NewRepository(t.TempDir())
	source := testProfile()
	if err := repo.Save(source); err != nil {
		t.Fatal(err)
	}
	alias := strings.ToUpper(source.ID)
	if _, err := repo.Get(alias); err == nil {
		t.Fatalf("Get(%q) accepted an alias for %q", alias, source.ID)
	}
	clone := source
	clone.ID = "copy-1"
	if err := repo.Clone(alias, clone); err == nil {
		t.Fatalf("Clone(%q) accepted an alias source", alias)
	}
	if runtime.GOOS == "windows" {
		aliasedSave := source
		aliasedSave.ID = alias
		if err := repo.Save(aliasedSave); err == nil {
			t.Fatalf("Save(%q) replaced an existing profile via alias", alias)
		}
		aliasedDestination := source
		aliasedDestination.ID = alias
		if err := repo.Clone(source.ID, aliasedDestination); err == nil {
			t.Fatalf("Clone(destination %q) replaced an existing profile via alias", alias)
		}
	}
	if err := repo.Delete(alias); err == nil {
		t.Fatalf("Delete(%q) accepted an alias for %q", alias, source.ID)
	}
	if got, err := repo.Get(source.ID); err != nil || !reflect.DeepEqual(got, source) {
		t.Fatalf("genuine profile changed after case aliases: %#v, %v", got, err)
	}
	if _, err := os.Stat(repo.ProfileDir("copy-1")); !os.IsNotExist(err) {
		t.Fatalf("clone was published from case alias: %v", err)
	}
}

func TestRepositoryCloneCopiesManagedSQLAndIsolatesProfiles(t *testing.T) {
	repo := NewRepository(t.TempDir())
	source := testProfile()
	source.Scripts = []profile.Script{
		{ID: "one", Name: "First", File: "scripts/one.sql", Enabled: true, Order: 10, TransactionMode: profile.TransactionRunnerManaged},
		{ID: "two", Name: "Second", File: "scripts/two.sql", Enabled: false, Order: 20, TransactionMode: profile.TransactionScriptManaged},
	}
	if err := repo.Save(source); err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{"one.sql": "SELECT 1;\n", "two.sql": "SELECT 2;\n"}
	for name, data := range contents {
		if err := os.WriteFile(filepath.Join(repo.ScriptRoot(source.ID), name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	clone := source
	clone.ID = "copy-1"
	clone.Name += " (copy)"
	if err := repo.Clone(source.ID, clone); err != nil {
		t.Fatalf("Clone() error: %v", err)
	}
	got, err := repo.Get(clone.ID)
	if err != nil || !reflect.DeepEqual(got, clone) {
		t.Fatalf("cloned metadata = %#v, %v; want %#v", got, err, clone)
	}
	for name, data := range contents {
		copied, err := os.ReadFile(filepath.Join(repo.ScriptRoot(clone.ID), name))
		if err != nil || string(copied) != data {
			t.Fatalf("cloned %s = %q, %v", name, copied, err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo.ScriptRoot(clone.ID), "one.sql"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveScript(clone.ID, "two"); err != nil {
		t.Fatal(err)
	}
	for name, data := range contents {
		original, err := os.ReadFile(filepath.Join(repo.ScriptRoot(source.ID), name))
		if err != nil || string(original) != data {
			t.Fatalf("source %s after clone mutation = %q, %v", name, original, err)
		}
	}
	if err := repo.Clone(source.ID, clone); err == nil {
		t.Fatal("Clone() replaced an existing destination")
	}
	if got, err := repo.Get(clone.ID); err != nil || len(got.Scripts) != 1 {
		t.Fatalf("existing destination changed: %#v, %v", got, err)
	}
}

func TestRepositoryCloneCleansUpAfterSourceErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(string) error
	}{
		{name: "missing file", setup: func(string) error { return nil }},
		{name: "directory as SQL file", setup: func(path string) error { return os.Mkdir(path, 0o700) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewRepository(t.TempDir())
			source := testProfile()
			source.Scripts = []profile.Script{
				{ID: "one", Name: "First", File: "scripts/one.sql", Enabled: true, Order: 10},
				{ID: "two", Name: "Second", File: "scripts/two.sql", Enabled: true, Order: 20},
			}
			if err := repo.Save(source); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo.ScriptRoot(source.ID), "one.sql"), []byte("SELECT 1;"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.setup(filepath.Join(repo.ScriptRoot(source.ID), "two.sql")); err != nil {
				t.Fatal(err)
			}
			clone := source
			clone.ID = "copy-1"
			if err := repo.Clone(source.ID, clone); err == nil {
				t.Fatal("Clone() succeeded with an invalid source script")
			}
			if _, err := os.Stat(repo.ProfileDir(clone.ID)); !os.IsNotExist(err) {
				t.Fatalf("partial destination remains: %v", err)
			}
			entries, err := os.ReadDir(filepath.Join(repo.Root(), "profiles"))
			if err != nil || len(entries) != 1 || entries[0].Name() != source.ID {
				t.Fatalf("staging directory remains after clone failure: %#v, %v", entries, err)
			}
			if got, err := repo.Get(source.ID); err != nil || !reflect.DeepEqual(got, source) {
				t.Fatalf("source changed: %#v, %v", got, err)
			}
		})
	}
}

func TestAddRemoveAndReorderScripts(t *testing.T) {
	root := t.TempDir()
	repo := NewRepository(root)
	p := testProfile()
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}

	sourceOne := filepath.Join(t.TempDir(), "source-one.sql")
	sourceTwo := filepath.Join(t.TempDir(), "source-two.sql")
	if err := os.WriteFile(sourceOne, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceTwo, []byte("SELECT 2;"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := profile.Script{ID: "script-one", Name: "First", Enabled: true, Order: 10}
	second := profile.Script{ID: "script-two", Name: "Second", Enabled: true, Order: 20}
	if err := repo.AddScript(p.ID, first, sourceOne); err != nil {
		t.Fatalf("AddScript(first) error: %v", err)
	}
	if err := repo.AddScript(p.ID, second, sourceTwo); err != nil {
		t.Fatalf("AddScript(second) error: %v", err)
	}

	stored, err := repo.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.Scripts[0].File; got != "scripts/script-one.sql" {
		t.Fatalf("stored source path = %q", got)
	}
	copied, err := os.ReadFile(filepath.Join(repo.ScriptRoot(p.ID), "script-one.sql"))
	if err != nil || string(copied) != "SELECT 1;" {
		t.Fatalf("copied script = %q, %v", copied, err)
	}

	if err := repo.ReorderScripts(p.ID, []string{"script-two", "script-one"}); err != nil {
		t.Fatalf("ReorderScripts() error: %v", err)
	}
	stored, _ = repo.Get(p.ID)
	orders := map[string]int{}
	for _, script := range stored.Scripts {
		orders[script.ID] = script.Order
	}
	if orders["script-two"] != 10 || orders["script-one"] != 20 {
		t.Fatalf("unexpected orders: %#v", orders)
	}

	if err := repo.RemoveScript(p.ID, "script-one"); err != nil {
		t.Fatalf("RemoveScript() error: %v", err)
	}
	stored, _ = repo.Get(p.ID)
	if len(stored.Scripts) != 1 || stored.Scripts[0].ID != "script-two" {
		t.Fatalf("unexpected scripts after removal: %#v", stored.Scripts)
	}
	if _, err := os.Stat(filepath.Join(repo.ScriptRoot(p.ID), "script-one.sql")); !os.IsNotExist(err) {
		t.Fatalf("expected removed script file, stat error: %v", err)
	}
}
