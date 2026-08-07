package id

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a cryptographically random, filesystem-safe 128-bit identifier.
func New() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
