package redact

import "strings"

// Secrets replaces every non-empty secret in text with a fixed marker.
func Secrets(text string, secrets ...string) string {
	out := text
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, "***")
	}
	return out
}
