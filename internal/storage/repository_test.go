package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

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
	for _, id := range []string{"../outside", ".", ""} {
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
