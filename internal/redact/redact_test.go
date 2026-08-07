package redact

import (
	"strings"
	"testing"
)

func TestSecretsRemovesEveryNonEmptySecret(t *testing.T) {
	out := Secrets("password=secret token=other", "secret", "other", "")
	if strings.Contains(out, "secret") || strings.Contains(out, "other") {
		t.Fatalf("Secrets() leaked secret: %q", out)
	}
	if strings.Count(out, "***") != 2 {
		t.Fatalf("Secrets() = %q, want two redactions", out)
	}
}
