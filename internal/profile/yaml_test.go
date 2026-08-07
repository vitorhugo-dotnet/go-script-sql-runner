package profile

import (
	"bytes"
	"strings"
	"testing"
)

func TestYAMLRoundTrip(t *testing.T) {
	want := validProfile()
	want.Scripts = []Script{{ID: "script-1", Name: "Synthetic", File: "scripts/script-1.sql", Enabled: true, Order: 10}}

	var buf bytes.Buffer
	if err := Encode(&buf, want); err != nil {
		t.Fatalf("Encode() error: %v", err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode() error: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name || len(got.Scripts) != 1 || got.Scripts[0].ID != "script-1" {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := Decode(strings.NewReader("name: x\nunknown: true\n"))
	if err == nil {
		t.Fatal("Decode() expected unknown-field error")
	}
}

func TestDecodeRejectsTrailingDocument(t *testing.T) {
	yamlText := `id: demo
name: Demo
version: 1
connection:
  host: 127.0.0.1
  port: 3306
  database: example
  username: dev
  password: ""
execution:
  on_error: continue
  transaction_mode: auto_commit
---
id: second
`
	if _, err := Decode(strings.NewReader(yamlText)); err == nil {
		t.Fatal("Decode() expected multiple-document error")
	}
}
