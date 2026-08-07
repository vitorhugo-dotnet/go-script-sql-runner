package logging

import "strings"

// Redact replaces every non-empty secret in text with a fixed marker.
func Redact(text string, secrets ...string) string {
	out := text
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, "***")
	}
	return out
}
