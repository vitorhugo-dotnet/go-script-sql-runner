//go:build integration

package integration

import (
	"context"
	"testing"
)

func TestDatabaseCompatibility(t *testing.T) {
	for _, server := range servers {
		t.Run(server.name, func(t *testing.T) {
			client := connectServer(t, server)
			if client.Capabilities.VersionLabel != server.wantLabel {
				t.Fatalf("version label = %q, want %q (raw=%q)", client.Capabilities.VersionLabel, server.wantLabel, client.Capabilities.RawVersion)
			}
			var one int
			if err := client.DB.QueryRowContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
				t.Fatalf("SELECT 1 error: %v", err)
			}
			if one != 1 {
				t.Fatalf("SELECT 1 = %d", one)
			}
		})
	}
}
