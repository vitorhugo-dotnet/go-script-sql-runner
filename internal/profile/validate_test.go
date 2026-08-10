package profile

import "testing"

func validProfile() Profile {
	return Profile{
		ID:      "a1b2c3",
		Name:    "Local Dev",
		Version: 1,
		Connection: Connection{
			Host: "127.0.0.1", Port: 3306, Database: "example", Username: "dev", Password: "secret",
		},
		Execution: Execution{OnError: OnErrorContinue, TransactionMode: TransactionAutoCommit},
	}
}

func TestValidateAcceptsValidProfile(t *testing.T) {
	if err := Validate(validProfile()); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidateAcceptsProfileWithoutDatabase(t *testing.T) {
	p := validProfile()
	p.Connection.Database = ""
	if err := Validate(p); err != nil {
		t.Fatalf("Validate() with runtime-only schema returned error: %v", err)
	}
}

func TestValidateRejectsCoreInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Profile)
	}{
		{"empty id", func(p *Profile) { p.ID = "" }},
		{"unsafe id", func(p *Profile) { p.ID = "../escape" }},
		{"empty name", func(p *Profile) { p.Name = "" }},
		{"bad version", func(p *Profile) { p.Version = 2 }},
		{"empty host", func(p *Profile) { p.Connection.Host = "" }},
		{"zero port", func(p *Profile) { p.Connection.Port = 0 }},
		{"large port", func(p *Profile) { p.Connection.Port = 65536 }},
		{"bad failure mode", func(p *Profile) { p.Execution.OnError = "explode" }},
		{"bad transaction", func(p *Profile) { p.Execution.TransactionMode = "magic" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)
			if err := Validate(p); err == nil {
				t.Fatal("Validate() expected error, got nil")
			}
		})
	}
}

func TestValidateRejectsDuplicateAndUnsafeScripts(t *testing.T) {
	p := validProfile()
	p.Scripts = []Script{
		{ID: "one", Name: "One", File: "scripts/one.sql", Enabled: true, Order: 10},
		{ID: "one", Name: "Two", File: "../outside.sql", Enabled: true, Order: 10, TransactionMode: "magic"},
	}
	if err := Validate(p); err == nil {
		t.Fatal("Validate() expected script validation errors")
	}
}
