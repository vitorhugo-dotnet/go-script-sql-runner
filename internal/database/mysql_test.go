package database

import (
	"testing"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func TestBuildConfig(t *testing.T) {
	conn := profile.Connection{
		Host: "127.0.0.1", Port: 3306, Database: "example", Username: "dev", Password: "secret",
	}
	cfg := buildConfig(conn, "example")
	if !cfg.MultiStatements {
		t.Fatal("MultiStatements = false, want true")
	}
	if cfg.Net != "tcp" {
		t.Fatalf("Net = %q", cfg.Net)
	}
	if cfg.Addr != "127.0.0.1:3306" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.DBName != "example" {
		t.Fatalf("DBName = %q", cfg.DBName)
	}
	if cfg.User != "dev" || cfg.Passwd != "secret" {
		t.Fatalf("credentials not copied to config")
	}
	if cfg.Timeout <= 0 || cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 {
		t.Fatalf("timeouts must be non-zero: %s %s %s", cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if cfg.Timeout != 10*time.Second {
		t.Fatalf("Timeout = %s", cfg.Timeout)
	}
}

func TestBuildConfigWithoutDatabaseForProbe(t *testing.T) {
	conn := profile.Connection{Host: "db.internal", Port: 3307, Username: "dev", Password: "secret"}
	cfg := buildConfig(conn, "")
	if cfg.DBName != "" {
		t.Fatalf("probe DBName = %q, want blank", cfg.DBName)
	}
	if cfg.Addr != "db.internal:3307" {
		t.Fatalf("probe Addr = %q", cfg.Addr)
	}
}
