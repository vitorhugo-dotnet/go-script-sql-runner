package id

import "testing"

func TestNewReturnsUniqueHexIDs(t *testing.T) {
	first, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	second, err := New()
	if err != nil {
		t.Fatalf("New() second error: %v", err)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("expected 32-character IDs, got %q and %q", first, second)
	}
	if first == second {
		t.Fatalf("expected unique IDs, got %q twice", first)
	}
}
