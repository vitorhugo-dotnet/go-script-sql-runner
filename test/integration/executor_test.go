//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/database"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/executor"
	"github.com/vitorhugo-dotnet/go-script-sql-runner/internal/profile"
)

func TestExecutorCompatibility(t *testing.T) {
	for _, server := range servers {
		server := server
		t.Run(server.name, func(t *testing.T) {
			client := connectServer(t, server)

			t.Run("auto_commit_multi_statement", func(t *testing.T) {
				table := uniqueTable("multi")
				defer client.DB.Exec("DROP TABLE IF EXISTS " + table)
				root := t.TempDir()
				script := writeScript(t, root, "multi", fmt.Sprintf("CREATE TABLE %s(id INT PRIMARY KEY); INSERT INTO %s(id) VALUES (1),(2);", table, table))
				script.Order = 10
				p := executionProfile(server, profile.OnErrorContinue, profile.TransactionAutoCommit, script)
				summary := executor.Run(context.Background(), client, p, root, executor.RunOptions{}, nil)
				if summary.Succeeded != 1 || summary.Failed != 0 {
					t.Fatalf("summary = %#v", summary)
				}
				assertCount(t, client, table, 2)
			})

			t.Run("transaction_commit", func(t *testing.T) {
				table := uniqueTable("commit")
				mustExec(t, client, "CREATE TABLE "+table+"(id INT PRIMARY KEY)")
				defer client.DB.Exec("DROP TABLE IF EXISTS " + table)
				root := t.TempDir()
				script := writeScript(t, root, "commit", fmt.Sprintf("INSERT INTO %s(id) VALUES (1); INSERT INTO %s(id) VALUES (2);", table, table))
				script.Order = 10
				p := executionProfile(server, profile.OnErrorContinue, profile.TransactionRunnerManaged, script)
				summary := executor.Run(context.Background(), client, p, root, executor.RunOptions{}, nil)
				if summary.Succeeded != 1 {
					t.Fatalf("summary = %#v", summary)
				}
				assertCount(t, client, table, 2)
			})

			t.Run("transaction_rollback", func(t *testing.T) {
				table := uniqueTable("rollback")
				mustExec(t, client, "CREATE TABLE "+table+"(id INT PRIMARY KEY)")
				defer client.DB.Exec("DROP TABLE IF EXISTS " + table)
				root := t.TempDir()
				script := writeScript(t, root, "rollback", fmt.Sprintf("INSERT INTO %s(id) VALUES (1); INSERT INTO runner_missing_table(id) VALUES (2);", table))
				script.Order = 10
				p := executionProfile(server, profile.OnErrorContinue, profile.TransactionRunnerManaged, script)
				summary := executor.Run(context.Background(), client, p, root, executor.RunOptions{}, nil)
				if summary.Failed != 1 {
					t.Fatalf("summary = %#v", summary)
				}
				assertCount(t, client, table, 0)
			})

			t.Run("ddl_warning", func(t *testing.T) {
				table := uniqueTable("ddl")
				defer client.DB.Exec("DROP TABLE IF EXISTS " + table)
				root := t.TempDir()
				script := writeScript(t, root, "ddl", "CREATE TABLE "+table+"(id INT PRIMARY KEY);")
				script.Order = 10
				p := executionProfile(server, profile.OnErrorContinue, profile.TransactionRunnerManaged, script)
				warned := false
				executor.Run(context.Background(), client, p, root, executor.RunOptions{}, executor.SinkFunc(func(event executor.Event) {
					if event.Level == executor.LevelWarn && strings.Contains(event.Message, "implicit commit") {
						warned = true
					}
				}))
				if !warned {
					t.Fatal("expected implicit-commit warning")
				}
			})

			t.Run("continue_and_stop", func(t *testing.T) {
				for _, policy := range []profile.OnError{profile.OnErrorContinue, profile.OnErrorStop} {
					policy := policy
					t.Run(string(policy), func(t *testing.T) {
						table := uniqueTable("policy")
						defer client.DB.Exec("DROP TABLE IF EXISTS " + table)
						root := t.TempDir()
						bad := writeScript(t, root, "bad", "INSERT INTO runner_missing_table(id) VALUES (1);")
						bad.Order = 10
						good := writeScript(t, root, "good", "CREATE TABLE "+table+"(id INT PRIMARY KEY);")
						good.Order = 20
						p := executionProfile(server, policy, profile.TransactionAutoCommit, bad, good)
						summary := executor.Run(context.Background(), client, p, root, executor.RunOptions{}, nil)
						exists := tableExists(t, client, table)
						if policy == profile.OnErrorContinue && (!exists || summary.Failed != 1 || summary.Succeeded != 1 || summary.Aborted) {
							t.Fatalf("continue: exists=%v summary=%#v", exists, summary)
						}
						if policy == profile.OnErrorStop && (exists || summary.Failed != 1 || summary.Succeeded != 0 || !summary.Aborted) {
							t.Fatalf("stop: exists=%v summary=%#v", exists, summary)
						}
					})
				}
			})
		})
	}
}

func executionProfile(server serverCase, onError profile.OnError, mode profile.TransactionMode, scripts ...profile.Script) profile.Profile {
	return profile.Profile{
		ID: "integration", Name: "Integration", Version: 1,
		Connection: connectionFor(server),
		Execution: profile.Execution{OnError: onError, TransactionMode: mode},
		Scripts: scripts,
	}
}

func mustExec(t *testing.T, client *database.Client, query string) {
	t.Helper()
	if _, err := client.DB.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertCount(t *testing.T, client *database.Client, table string, want int) {
	t.Helper()
	var got int
	if err := client.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("count %s = %d, want %d", table, got, want)
	}
}

func tableExists(t *testing.T, client *database.Client, table string) bool {
	t.Helper()
	var count int
	if err := client.DB.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&count); err != nil {
		t.Fatalf("tableExists(%s): %v", table, err)
	}
	return count == 1
}
