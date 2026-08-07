package wailsui

import "testing"

func TestShouldHideOwnConsole(t *testing.T) {
	if !shouldHideOwnConsole(1) {
		t.Fatal("single-process console should be hidden for GUI launch")
	}
	for _, count := range []uintptr{0, 2, 3} {
		if shouldHideOwnConsole(count) {
			t.Fatalf("console with %d attached processes should remain visible", count)
		}
	}
}
