# Core and CLI Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the reusable Go core and working CLI for creating local profiles, importing SQL into AppData, detecting MySQL 5.6/5.7/8, and executing ordered scripts with configurable failure and transaction policies.

**Architecture:** The Go domain and application services are independent of Wails/React. Profiles are persisted under an injectable AppData root, database access is isolated behind a MySQL adapter, and both CLI and later GUI code consume the same application service. A SQL file is the execution unit and is sent as a multi-statement query; failure policy controls whether the next file is executed.

**Tech Stack:** Go 1.26.5, Wails v2.13.0 scaffold, `database/sql`, `github.com/go-sql-driver/mysql` v1.10.0, `github.com/spf13/cobra` v1.10.2, `go.yaml.in/yaml/v3` v3.0.4, `log/slog`, React + TypeScript scaffold, Tailwind CSS v4 via `@tailwindcss/vite`.

## Global Constraints

- Windows 10/11 only for v1.
- One distributed `.exe` must eventually expose GUI with no arguments and CLI with arguments.
- Real company SQL, hosts, IPs, usernames, passwords, schema names, profiles, logs, and AppData contents must never enter Git.
- Runtime SQL and YAML live under `%APPDATA%\\GoScriptSQLRunner` through `os.UserConfigDir()`; tests inject temporary roots.
- MySQL 5.6, 5.7, and 8.x must be detected automatically.
- The current MySQL driver formally supports MySQL 5.7+; MySQL 5.6 acceptance therefore requires integration tests against a real 5.6 server image.
- No SQL templating or `${variable}` substitution.
- Export/import ZIP behavior belongs to the next plan.
- SQL examples committed to tests must be synthetic only.

## File Map

```text
main.go                                  # final CLI/GUI dispatch entry point, GUI added in plan 3
wails.json                               # Wails v2 project metadata
.gitignore                               # blocks runtime/internal artifacts
frontend/package.json                    # React/Vite/Tailwind dependencies
frontend/vite.config.ts                  # Tailwind Vite plugin
frontend/src/style.css                   # Tailwind import
internal/id/id.go                        # random stable IDs without another dependency
internal/profile/model.go                # profile/script/connection domain types
internal/profile/validate.go             # strict profile validation
internal/profile/yaml.go                 # YAML encode/decode with KnownFields
internal/storage/paths.go                # AppData path resolution
internal/storage/repository.go           # profile CRUD and SQL copy/remove/reorder persistence
internal/database/types.go               # vendor/version/capability types
internal/database/version.go             # VERSION() parsing
internal/database/mysql.go               # DSN creation, server probe, DB connection
internal/executor/types.go               # run options/results/events
internal/executor/analyze.go             # DDL/client-directive preflight
internal/executor/runner.go               # ordered script execution
internal/executor/transaction.go          # auto_commit/transaction/script_managed execution
internal/logging/redact.go                # password redaction helpers
internal/app/service.go                   # shared use cases for CLI and later GUI
internal/cli/root.go                      # Cobra root and dependency wiring
internal/cli/profile.go                   # create/list/show commands
internal/cli/script.go                    # add/remove commands
internal/cli/connection.go                # connection test command
internal/cli/run.go                       # run command and concise/verbose output
test/integration/docker-compose.yml       # synthetic MySQL 5.6/5.7/8 services
test/integration/database_test.go         # version and connection compatibility
test/integration/executor_test.go         # execution/transaction compatibility
```

---

### Task 1: Scaffold Wails, Go module, Tailwind, and repository safety

**Files:**
- Create/replace: `main.go`
- Create: `wails.json`
- Create: `go.mod`
- Create: `go.sum`
- Create/modify: `.gitignore`
- Create: `frontend/package.json`
- Create: `frontend/package-lock.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/src/style.css`
- Keep: `docs/superpowers/**`

**Interfaces:**
- Produces module path `github.com/vitorhugo-dotnet/go-script-sql-runner`.
- Produces a buildable Wails v2.13 React/TypeScript skeleton used by plan 3.
- Produces Tailwind v4 build configuration but no final UI yet.

- [ ] **Step 1: Generate the official React/TypeScript Wails v2.13 scaffold in a temporary directory**

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
$tmp = Join-Path $env:TEMP "go-script-sql-runner-wails"
Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
wails init -n go-script-sql-runner -d $tmp -t react-ts
Copy-Item "$tmp\*" . -Recurse -Force
Remove-Item $tmp -Recurse -Force
```

Do not use `-g`; the repository already exists.

- [ ] **Step 2: Pin the supported Go/tool dependencies**

Run:

```powershell
go mod edit -module github.com/vitorhugo-dotnet/go-script-sql-runner
go mod edit -go=1.26.0
go get github.com/wailsapp/wails/v2@v2.13.0
go get github.com/go-sql-driver/mysql@v1.10.0
go get github.com/spf13/cobra@v1.10.2
go get go.yaml.in/yaml/v3@v3.0.4
go mod tidy
```

Expected: `go.mod` contains only project dependencies, never local replace directives or company modules.

- [ ] **Step 3: Install Tailwind v4 through the official Vite plugin**

Run from `frontend`:

```powershell
npm install tailwindcss @tailwindcss/vite
```

Set `frontend/vite.config.ts` to:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
})
```

Set the first line of `frontend/src/style.css` to:

```css
@import "tailwindcss";
```

Remove Wails template demo styling that is no longer referenced.

- [ ] **Step 4: Add a defensive `.gitignore`**

Ensure it contains at least:

```gitignore
# Build/dependencies
build/bin/
frontend/node_modules/
frontend/dist/

# Local runtime/profile artifacts
GoScriptSQLRunner/
appdata/
.local/
*.log
*.profile.zip
*.runner.zip
profiles/
scripts-local/

# Environment/credentials
.env
.env.*
!.env.example
*.local.yaml
*.local.yml

# IDE/OS
.idea/
.vscode/
.DS_Store
Thumbs.db
```

Do not add any user-provided `.sql` to the repository while doing this task.

- [ ] **Step 5: Verify the scaffold builds before domain work**

Run:

```powershell
go test ./...
cd frontend
npm run build
cd ..
wails build -clean -webview2 embed
```

Expected: all three commands succeed and `build/bin/go-script-sql-runner.exe` exists.

- [ ] **Step 6: Commit**

```powershell
git add .gitignore go.mod go.sum main.go wails.json build frontend
git commit -m "chore: scaffold Wails runner"
```

---

### Task 2: Profile domain, strict YAML, IDs, and AppData repository

**Files:**
- Create: `internal/id/id.go`
- Create: `internal/id/id_test.go`
- Create: `internal/profile/model.go`
- Create: `internal/profile/validate.go`
- Create: `internal/profile/validate_test.go`
- Create: `internal/profile/yaml.go`
- Create: `internal/profile/yaml_test.go`
- Create: `internal/storage/paths.go`
- Create: `internal/storage/repository.go`
- Create: `internal/storage/repository_test.go`

**Interfaces:**
- Produces `profile.Profile`, `profile.Script`, `profile.Connection`, `profile.Execution`.
- Produces `profile.Decode(io.Reader) (Profile, error)` and `profile.Encode(io.Writer, Profile) error`.
- Produces `storage.NewRepository(root string) *Repository` and CRUD/script-copy methods used by all later tasks.

- [ ] **Step 1: Write failing tests for domain validation and strict YAML**

Define the domain shape in tests before implementation:

```go
p := profile.Profile{
    ID: "a1b2c3",
    Name: "Local Dev",
    Version: 1,
    Connection: profile.Connection{
        Host: "127.0.0.1", Port: 3306, Database: "example",
        Username: "dev", Password: "secret",
    },
    Execution: profile.Execution{
        OnError: profile.OnErrorContinue,
        TransactionMode: profile.TransactionAutoCommit,
    },
}
```

Tests must assert:

```go
require.NoError(t, profile.Validate(p))
require.Error(t, profile.Validate(profile.Profile{}))
require.Error(t, profile.Validate(profileWithPort(0)))
require.Error(t, profile.Decode(strings.NewReader("name: x\nunknown: true\n")))
```

Run:

```powershell
go test ./internal/profile ./internal/id
```

Expected: FAIL because types/functions do not exist.

- [ ] **Step 2: Implement stable random IDs using `crypto/rand`**

`internal/id/id.go`:

```go
package id

import (
    "crypto/rand"
    "encoding/hex"
)

func New() (string, error) {
    var b [16]byte
    if _, err := rand.Read(b[:]); err != nil {
        return "", err
    }
    return hex.EncodeToString(b[:]), nil
}
```

Test length is 32 hex characters and two generated IDs differ.

- [ ] **Step 3: Implement the profile model and validation**

Use exact enums:

```go
type OnError string
const (
    OnErrorContinue OnError = "continue"
    OnErrorStop     OnError = "stop"
)

type TransactionMode string
const (
    TransactionAutoCommit    TransactionMode = "auto_commit"
    TransactionRunnerManaged TransactionMode = "transaction"
    TransactionScriptManaged TransactionMode = "script_managed"
)
```

`Script.TransactionMode` is optional (`""` means inherit profile default). Validation rejects duplicate script IDs, duplicate order values, paths outside `scripts/`, invalid enums, blank host/name/ID, ports outside `1..65535`, and profile versions other than `1`.

- [ ] **Step 4: Implement strict YAML encode/decode**

Use `yaml.NewDecoder`, call `KnownFields(true)`, decode one document, then require EOF so trailing YAML documents are rejected. Encode with 2-space indentation.

Run:

```powershell
go test ./internal/profile ./internal/id
```

Expected: PASS.

- [ ] **Step 5: Write failing AppData repository tests**

Use `t.TempDir()` and assert this physical layout:

```text
<root>/profiles/<profile-id>/profile.yaml
<root>/profiles/<profile-id>/scripts/<script-id>.sql
<root>/logs/
```

Test `Save`, `Get`, `List`, `Delete`, `AddScript`, `RemoveScript`, and `ReorderScripts`. `AddScript` must copy source bytes and never persist the original source path.

- [ ] **Step 6: Implement path resolution and repository persistence**

`DefaultRoot()` must call `os.UserConfigDir()` then append `GoScriptSQLRunner`. `NewRepository(root)` allows tests to inject another root.

Writes use temp-file + `os.Rename` for `profile.yaml` so interrupted writes do not leave half YAML files. Script files are named by internal ID, not source filename.

- [ ] **Step 7: Verify repository behavior**

Run:

```powershell
go test ./internal/id ./internal/profile ./internal/storage
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add internal/id internal/profile internal/storage
git commit -m "feat: add AppData profile storage"
```

---

### Task 3: MySQL connection, version detection, and capability classification

**Files:**
- Create: `internal/database/types.go`
- Create: `internal/database/version.go`
- Create: `internal/database/version_test.go`
- Create: `internal/database/mysql.go`
- Create: `internal/database/mysql_test.go`

**Interfaces:**
- Consumes `profile.Connection`.
- Produces `database.ServerCapabilities`.
- Produces `database.Client` with `DB *sql.DB`, `Capabilities ServerCapabilities`, and `Close() error`.
- Produces `database.Connect(ctx context.Context, c profile.Connection) (*Client, error)`.

- [ ] **Step 1: Write version parser tests**

Required cases:

```go
{"5.6.51", VendorMySQL, 5, 6, 51},
{"5.7.44-log", VendorMySQL, 5, 7, 44},
{"8.0.43", VendorMySQL, 8, 0, 43},
{"10.11.8-MariaDB", VendorMariaDB, 10, 11, 8},
```

Also require errors for blank/unparseable versions and MySQL majors not in 5 or 8.

Run:

```powershell
go test ./internal/database
```

Expected: FAIL.

- [ ] **Step 2: Implement centralized parsing**

`ServerCapabilities`:

```go
type Vendor string
const (
    VendorMySQL   Vendor = "mysql"
    VendorMariaDB Vendor = "mariadb"
)

type ServerCapabilities struct {
    Vendor       Vendor
    Major        int
    Minor        int
    Patch        int
    RawVersion   string
    VersionLabel string
}
```

`VersionLabel` must produce `MySQL 5.6`, `MySQL 5.7`, `MySQL 8.x`, or `MariaDB`.

- [ ] **Step 3: Write database connection tests using `sqlmock`-free seams**

Do not add a mocking dependency. Extract `buildConfig(c profile.Connection, dbName string) mysql.Config` and test its fields directly:

```go
cfg := buildConfig(conn, "example")
require.True(t, cfg.MultiStatements)
require.Equal(t, "tcp", cfg.Net)
require.Equal(t, "127.0.0.1:3306", cfg.Addr)
require.Equal(t, "example", cfg.DBName)
```

Require non-zero dial/read/write timeouts.

- [ ] **Step 4: Implement server-first probe then database connection**

Use `mysql.Config` instead of manually concatenating DSNs.

Flow:

```text
open DSN without DB -> PingContext -> SELECT VERSION(), @@version_comment
-> classify capabilities -> close probe connection
-> open DSN with configured DB -> PingContext -> return Client
```

This separates authentication/network/version errors from database/schema selection errors.

Set `MultiStatements: true` because a `.sql` file is executed as one multi-statement payload. Use finite connect/read/write timeouts (10s each) and `context.Context` for caller cancellation.

- [ ] **Step 5: Verify unit tests**

Run:

```powershell
go test ./internal/database
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/database
git commit -m "feat: detect MySQL server capabilities"
```

---

### Task 4: Script executor, transaction modes, failure policy, and redacted events

**Files:**
- Create: `internal/executor/types.go`
- Create: `internal/executor/analyze.go`
- Create: `internal/executor/analyze_test.go`
- Create: `internal/executor/transaction.go`
- Create: `internal/executor/runner.go`
- Create: `internal/executor/runner_test.go`
- Create: `internal/logging/redact.go`
- Create: `internal/logging/redact_test.go`

**Interfaces:**
- Consumes `database.Client`, `profile.Profile`, and AppData script paths.
- Produces `executor.Run(ctx, client, profile, scriptRoot, options, sink) Summary`.
- Produces real-time `executor.Event` values consumed by CLI and later Wails.

- [ ] **Step 1: Define event/result contracts and write failing policy tests**

Use these exact public shapes:

```go
type Level string
const (
    LevelInfo Level = "INFO"
    LevelWarn Level = "WARN"
    LevelError Level = "ERROR"
)

type Event struct {
    Time     time.Time
    Level    Level
    ScriptID string
    Message  string
    Detail   string
}

type Sink interface { Emit(Event) }

type ScriptResult struct {
    ScriptID string
    Name string
    StartedAt time.Time
    EndedAt time.Time
    Success bool
    Error string
}

type Summary struct {
    Results []ScriptResult
    Succeeded int
    Failed int
    Aborted bool
}
```

Test that disabled scripts are skipped, scripts run by ascending `Order`, `continue` executes the next file after failure, and `stop` does not.

- [ ] **Step 2: Add SQL preflight tests**

`Analyze(sql string)` returns warnings and hard validation errors. Require:

- DDL beginning with `CREATE`, `ALTER`, `DROP`, `TRUNCATE`, or `RENAME` emits an implicit-commit warning when runner-managed transaction mode is selected.
- A line beginning with MySQL client directive `DELIMITER` returns a clear unsupported-client-directive error instead of sending it blindly to the server.
- Ordinary DML has no warning.

This is intentionally not a general SQL parser.

- [ ] **Step 3: Implement transaction execution around one whole SQL file**

Exact behavior:

```go
switch mode {
case profile.TransactionAutoCommit:
    _, err = db.ExecContext(ctx, sqlText)
case profile.TransactionRunnerManaged:
    tx, beginErr := db.BeginTx(ctx, nil)
    if beginErr != nil { return beginErr }
    if _, err = tx.ExecContext(ctx, sqlText); err != nil {
        rollbackErr := tx.Rollback()
        return errors.Join(err, rollbackErr)
    }
    err = tx.Commit()
case profile.TransactionScriptManaged:
    _, err = db.ExecContext(ctx, sqlText)
}
```

Do not issue manual `BEGIN`/`COMMIT` strings for runner-managed mode.

- [ ] **Step 4: Implement ordered orchestration and event emission**

Read each enabled file from the profile's AppData `scripts` directory, execute it, emit start/success/failure/warning events, append a `ScriptResult`, then apply effective `OnError` from runtime options or profile default.

A failed SQL file is one failed execution unit; do not attempt to continue individual statements inside that same file.

- [ ] **Step 5: Implement password redaction and tests**

`logging.Redact(text string, secrets ...string) string` replaces every non-empty secret with `***`. All code producing diagnostic `Detail` strings must pass the configured database password through redaction before emitting/logging it.

Test:

```go
out := logging.Redact("dsn user:secret@tcp(localhost)", "secret")
require.NotContains(t, out, "secret")
require.Contains(t, out, "***")
```

- [ ] **Step 6: Run executor tests**

Run:

```powershell
go test ./internal/executor ./internal/logging
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/executor internal/logging
git commit -m "feat: execute ordered SQL profiles"
```

---

### Task 5: Shared application service and CLI commands

**Files:**
- Create: `internal/app/service.go`
- Create: `internal/app/service_test.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/profile.go`
- Create: `internal/cli/script.go`
- Create: `internal/cli/connection.go`
- Create: `internal/cli/run.go`
- Create: `internal/cli/cli_test.go`
- Modify: `main.go`

**Interfaces:**
- Produces `app.Service`, the only orchestration API used by CLI and later GUI.
- Produces `cli.Execute(ctx, service, args, stdout, stderr) int`.

- [ ] **Step 1: Write application service tests**

Define these methods:

```go
func (s *Service) CreateProfile(ctx context.Context, p profile.Profile) (profile.Profile, error)
func (s *Service) ListProfiles(ctx context.Context) ([]profile.Profile, error)
func (s *Service) GetProfile(ctx context.Context, id string) (profile.Profile, error)
func (s *Service) AddScript(ctx context.Context, profileID, sourcePath string) (profile.Script, error)
func (s *Service) RemoveScript(ctx context.Context, profileID, scriptID string) error
func (s *Service) TestConnection(ctx context.Context, profileID string) (database.ServerCapabilities, error)
func (s *Service) RunProfile(ctx context.Context, profileID string, opts executor.RunOptions, sink executor.Sink) (executor.Summary, error)
```

`CreateProfile` generates IDs when missing and defaults to port `3306`, `continue`, and `auto_commit`.

- [ ] **Step 2: Implement `app.Service` as thin orchestration**

Keep filesystem, SQL, and CLI formatting logic out of this package. It coordinates repository + database + executor and closes database clients with `defer`.

Run:

```powershell
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 3: Write CLI behavior tests with in-memory writers**

At minimum test:

```text
runner profile list
runner profile create --name Dev --host 127.0.0.1 --database demo --username root --password example
runner profile show <id>
runner script add <profile-id> synthetic.sql
runner script remove <profile-id> <script-id>
runner connection test <profile-id>
runner run <profile-id>
runner run <profile-id> --verbose
runner run <profile-id> --stop-on-error
runner run <profile-id> --continue-on-error
```

Tests inject `bytes.Buffer` for stdout/stderr and a temporary storage root.

- [ ] **Step 4: Implement Cobra command tree**

`root.go` must set `SilenceUsage: true` and `SilenceErrors: true`; errors are formatted once by `Execute`.

`run` flags:

```text
-v, --verbose
--stop-on-error
--continue-on-error
```

The stop/continue flags are mutually exclusive and override only the current run, not saved YAML.

Default output stays concise; verbose mode prints event timestamps, levels, script names, and redacted diagnostic details.

- [ ] **Step 5: Route arguments from `main.go` to CLI**

For this plan, `main.go` may exit with a short message when invoked with no arguments because the final GUI wiring is plan 3. With one or more arguments it must build the default AppData repository/service and call `cli.Execute`.

- [ ] **Step 6: Verify CLI tests and manual help**

Run:

```powershell
go test ./internal/app ./internal/cli
go run . --help
go run . profile --help
go run . run --help
```

Expected: tests pass and help exposes the specified commands/flags.

- [ ] **Step 7: Commit**

```powershell
git add internal/app internal/cli main.go
git commit -m "feat: add shared runner CLI"
```

---

### Task 6: Real MySQL 5.6, 5.7, and 8 integration suite

**Files:**
- Create: `test/integration/docker-compose.yml`
- Create: `test/integration/database_test.go`
- Create: `test/integration/executor_test.go`
- Create: `test/integration/helpers_test.go`

**Interfaces:**
- Verifies Task 3/4 compatibility against real servers.
- Uses only synthetic credentials/schema names such as `runner_test` / `runner` / `runnerpass`.

- [ ] **Step 1: Add generic container services**

Use three official MySQL images:

```yaml
services:
  mysql56:
    image: mysql:5.6.51
    environment:
      MYSQL_ROOT_PASSWORD: runnerpass
      MYSQL_DATABASE: runner_test
    ports: ["3356:3306"]
  mysql57:
    image: mysql:5.7.44
    environment:
      MYSQL_ROOT_PASSWORD: runnerpass
      MYSQL_DATABASE: runner_test
    ports: ["3357:3306"]
  mysql80:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: runnerpass
      MYSQL_DATABASE: runner_test
    ports: ["3380:3306"]
```

Add healthchecks using `mysqladmin ping` so tests do not rely on sleeps.

- [ ] **Step 2: Write table-driven version/connection integration tests**

Each endpoint must connect through the production `database.Connect`, assert the expected family (`5.6`, `5.7`, `8.x`), and execute `SELECT 1` against `runner_test`.

Gate integration tests with build tag:

```go
//go:build integration
```

- [ ] **Step 3: Write execution compatibility tests**

For every server version verify:

1. multi-statement DDL+DML file succeeds in `auto_commit`;
2. DML transaction commits on success;
3. DML transaction rolls back when a later statement fails;
4. DDL in `transaction` emits implicit-commit warning;
5. profile `continue` runs a synthetic good script after a failing script;
6. profile `stop` does not run the next script.

Use unique table names per test and clean them with `DROP TABLE IF EXISTS`.

- [ ] **Step 4: Run the real compatibility suite**

```powershell
docker compose -f test/integration/docker-compose.yml up -d --wait
go test -tags=integration ./test/integration -v
docker compose -f test/integration/docker-compose.yml down -v
```

Expected: PASS on MySQL 5.6.51, 5.7.44, and current 8.0 image. If 5.6 fails due to driver incompatibility, stop implementation and document the exact protocol/auth failure before selecting a driver workaround; do not claim 5.6 support based on unit tests.

- [ ] **Step 5: Run the whole non-integration suite**

```powershell
go test ./internal/...
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add test/integration
git commit -m "test: verify MySQL 5 and 8 compatibility"
```
