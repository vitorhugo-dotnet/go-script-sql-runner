package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
)

func TestFileSinkPersistsStructuredExecutionEvents(t *testing.T) {
	logDir := t.TempDir()
	sink, err := NewFileSink(logDir)
	if err != nil {
		t.Fatalf("NewFileSink() error: %v", err)
	}
	secret := "synthetic-password-must-not-appear"
	now := time.Now()
	sink.Emit(executor.Event{Time: now, Level: executor.LevelInfo, ScriptID: "one", Message: "Started", Detail: "***"})
	sink.Emit(executor.Event{Time: now, Level: executor.LevelWarn, ScriptID: "two", Message: "DDL warning"})
	sink.Emit(executor.Event{Time: now, Level: executor.LevelError, ScriptID: "three", Message: "Failed", Detail: "synthetic database error"})
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	filename := filepath.Join(logDir, "runner-"+time.Now().Format("2006-01-02")+".log")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	text := string(data)
	for _, want := range []string{"level=INFO", "level=WARN", "level=ERROR", "script_id=one", "script_id=two", "script_id=three", "message=Started", "DDL warning", "synthetic database error"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, secret) {
		t.Fatalf("log leaked synthetic secret: %s", text)
	}
}
