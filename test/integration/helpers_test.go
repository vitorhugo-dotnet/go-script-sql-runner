//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

type serverCase struct {
	name      string
	port      int
	wantLabel string
}

var servers = []serverCase{
	{name: "mysql56", port: 3356, wantLabel: "MySQL 5.6"},
	{name: "mysql57", port: 3357, wantLabel: "MySQL 5.7"},
	{name: "mysql80", port: 3380, wantLabel: "MySQL 8.x"},
}

var tableCounter atomic.Uint64

func connectionFor(server serverCase) profile.Connection {
	return profile.Connection{
		Host: "127.0.0.1", Port: server.port, Database: "runner_test", Username: "root", Password: "runnerpass",
	}
}

func connectServer(t *testing.T, server serverCase) *database.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := database.Connect(ctx, connectionFor(server))
	if err != nil {
		t.Fatalf("database.Connect(%s) error: %v", server.name, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func uniqueTable(prefix string) string {
	clean := strings.NewReplacer("/", "_", "-", "_").Replace(prefix)
	return fmt.Sprintf("runner_%s_%d", clean, tableCounter.Add(1))
}

func writeScript(t *testing.T, root, id, sqlText string) profile.Script {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id+".sql"), []byte(sqlText), 0o600); err != nil {
		t.Fatal(err)
	}
	return profile.Script{ID: id, Name: id, File: "scripts/" + id + ".sql", Enabled: true}
}
