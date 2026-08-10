package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func editingService(t *testing.T) (*Service, profile.Profile) {
	t.Helper()
	repo := storage.NewRepository(t.TempDir())
	p := sharingProfile("edit-profile", "Before", "old-password")
	if err := repo.Save(p); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"one", "two", "three"} {
		source := filepath.Join(t.TempDir(), id+".sql")
		if err := os.WriteFile(source, []byte(fmt.Sprintf("SELECT %d;", i+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddScript(p.ID, profile.Script{ID: id, Name: id, Enabled: true, Order: (i + 1) * 10}, source); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := repo.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(repo), stored
}

func TestUpdateProfilePreservesStoredIdentityAndScriptsAndDropsLegacyDatabase(t *testing.T) {
	service, before := editingService(t)
	updated := before
	updated.Name = "After"
	updated.Connection.Host = "db.synthetic.internal"
	updated.Connection.Database = "must-not-persist"
	updated.Connection.Password = "new-password"
	updated.Execution.OnError = profile.OnErrorStop
	updated.Scripts = nil
	updated.Version = 999

	got, err := service.UpdateProfile(context.Background(), updated)
	if err != nil {
		t.Fatalf("UpdateProfile() error: %v", err)
	}
	if got.ID != before.ID || got.Version != before.Version || got.Name != "After" || got.Connection.Password != "new-password" || got.Execution.OnError != profile.OnErrorStop {
		t.Fatalf("unexpected updated profile: %#v", got)
	}
	if got.Connection.Database != "" {
		t.Fatalf("updated database = %q, want runtime-only schema", got.Connection.Database)
	}
	if len(got.Scripts) != len(before.Scripts) {
		t.Fatalf("scripts were replaced: %#v", got.Scripts)
	}
	for i := range got.Scripts {
		if got.Scripts[i].ID != before.Scripts[i].ID || got.Scripts[i].File != before.Scripts[i].File {
			t.Fatalf("script identity/reference changed: before=%#v got=%#v", before.Scripts[i], got.Scripts[i])
		}
	}
}

func TestReorderScriptsAssignsDeterministicOrders(t *testing.T) {
	service, before := editingService(t)
	got, err := service.ReorderScripts(context.Background(), before.ID, []string{"three", "one", "two"})
	if err != nil {
		t.Fatalf("ReorderScripts() error: %v", err)
	}
	orders := map[string]int{}
	for _, script := range got.Scripts {
		orders[script.ID] = script.Order
	}
	if orders["three"] != 10 || orders["one"] != 20 || orders["two"] != 30 {
		t.Fatalf("unexpected orders: %#v", orders)
	}
	if _, err := service.ReorderScripts(context.Background(), before.ID, []string{"one", "two"}); err == nil {
		t.Fatal("missing ID reorder should fail")
	}
	if _, err := service.ReorderScripts(context.Background(), before.ID, []string{"one", "one", "three"}); err == nil {
		t.Fatal("duplicate ID reorder should fail")
	}
}

func TestScriptEditingSetters(t *testing.T) {
	service, p := editingService(t)
	got, err := service.SetScriptEnabled(context.Background(), p.ID, "two", false)
	if err != nil {
		t.Fatalf("SetScriptEnabled() error: %v", err)
	}
	for _, script := range got.Scripts {
		if script.ID == "two" && script.Enabled {
			t.Fatal("script two still enabled")
		}
	}
	got, err = service.SetScriptTransactionMode(context.Background(), p.ID, "two", profile.TransactionRunnerManaged)
	if err != nil {
		t.Fatalf("SetScriptTransactionMode(transaction) error: %v", err)
	}
	got, err = service.SetScriptTransactionMode(context.Background(), p.ID, "two", "")
	if err != nil {
		t.Fatalf("SetScriptTransactionMode(inherit) error: %v", err)
	}
	for _, script := range got.Scripts {
		if script.ID == "two" && script.TransactionMode != "" {
			t.Fatalf("script transaction mode = %q, want inheritance", script.TransactionMode)
		}
	}
	if _, err := service.SetScriptEnabled(context.Background(), p.ID, "missing", true); err == nil {
		t.Fatal("unknown script should fail")
	}
	if _, err := service.SetScriptTransactionMode(context.Background(), p.ID, "two", "invalid"); err == nil {
		t.Fatal("invalid transaction mode should fail validation")
	}
}
