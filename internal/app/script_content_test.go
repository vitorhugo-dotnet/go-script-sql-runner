package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/storage"
)

func TestScriptContentServiceRoundTripPreservesMetadata(t *testing.T) {
	service, before := editingService(t)
	got, err := service.GetScriptContent(context.Background(), before.ID, "one")
	if err != nil || got != "SELECT 1;" {
		t.Fatalf("GetScriptContent() = %q, %v", got, err)
	}
	for _, content := range []string{"-- Café\nSELECT 2;\n", ""} {
		if err := service.SaveScriptContent(context.Background(), before.ID, "one", content); err != nil {
			t.Fatalf("SaveScriptContent(%q): %v", content, err)
		}
		got, err := service.GetScriptContent(context.Background(), before.ID, "one")
		if err != nil || got != content {
			t.Fatalf("GetScriptContent() = %q, %v; want %q", got, err, content)
		}
		stored, err := service.GetProfile(context.Background(), before.ID)
		if err != nil || !reflect.DeepEqual(stored.Scripts, before.Scripts) {
			t.Fatalf("script metadata changed: %#v, %v", stored.Scripts, err)
		}
	}
}

func TestScriptContentServiceRejectsCancelledContextAndUnknownIDs(t *testing.T) {
	service, p := editingService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetScriptContent(ctx, p.ID, "one"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetScriptContent(cancelled) error = %v", err)
	}
	if err := service.SaveScriptContent(ctx, p.ID, "one", "changed"); !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveScriptContent(cancelled) error = %v", err)
	}
	for _, tc := range []struct{ profileID, scriptID string }{
		{"missing", "one"}, {p.ID, ""}, {p.ID, "missing"},
	} {
		if _, err := service.GetScriptContent(context.Background(), tc.profileID, tc.scriptID); err == nil {
			t.Fatalf("GetScriptContent(%q, %q) succeeded", tc.profileID, tc.scriptID)
		} else if tc.profileID == "missing" && !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("GetScriptContent(missing profile) error = %v, want ErrNotFound", err)
		}
		if err := service.SaveScriptContent(context.Background(), tc.profileID, tc.scriptID, "changed"); err == nil {
			t.Fatalf("SaveScriptContent(%q, %q) succeeded", tc.profileID, tc.scriptID)
		}
	}
	got, err := service.GetScriptContent(context.Background(), p.ID, "one")
	if err != nil || got != "SELECT 1;" {
		t.Fatalf("content changed after rejected saves: %q, %v", got, err)
	}
}
