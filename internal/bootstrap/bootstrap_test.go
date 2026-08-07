package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRuntimeCreatesCanonicalAppDataLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "GoScriptSQLRunner")
	runtime, err := newRuntime(root)
	if err != nil {
		t.Fatalf("newRuntime() error: %v", err)
	}
	if runtime.Service == nil {
		t.Fatal("runtime service is nil")
	}
	for _, dir := range []string{root, filepath.Join(root, "profiles"), filepath.Join(root, "logs")} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("runtime directory %s: info=%v err=%v", dir, info, err)
		}
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("Runtime.Close() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "logs", "runner-"+today()+".log")); err != nil {
		t.Fatalf("daily log file missing: %v", err)
	}
}
