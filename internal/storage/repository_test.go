package storage

import (
	"os"
	"path/filepath"
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
	if err != nil || len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if _, err := repo.Get(p.ID); err != ErrNotFound {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
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
