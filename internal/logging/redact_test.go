package logging

import (
	"strings"
	"testing"
)

func TestRedactRemovesSecrets(t *testing.T) {
	out := Redact("dsn user:secret@tcp(localhost) token=other", "secret", "other", "")
	if strings.Contains(out, "secret") || strings.Contains(out, "other") {
		t.Fatalf("Redact() leaked a secret: %q", out)
	}
	if strings.Count(out, "***") != 2 {
		t.Fatalf("Redact() = %q, want two redaction markers", out)
	}
}
