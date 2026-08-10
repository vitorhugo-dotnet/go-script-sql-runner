# Runtime Schema Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make MySQL schema selection a runtime-only choice loaded after connection testing and required for every SQL execution.

**Architecture:** Persistent profiles keep only server credentials and execution/script configuration. The application service probes MySQL without a default database, discovers visible schemas with `SHOW DATABASES`, and returns them with server capabilities. The selected schema lives only in frontend/CLI runtime state and is passed in `executor.RunOptions` so execution opens a database-specific connection without mutating or persisting the profile.

**Tech Stack:** Go, `database/sql`, `go-sql-driver/mysql`, Cobra, Wails v2.13, React 19, TypeScript 6, Tailwind CSS 4, Vitest.

## Global Constraints

- Windows 10/11 remains the desktop target.
- MySQL 5.6, 5.7 and 8.x compatibility must be preserved.
- Do not add a third-party React combobox dependency.
- Runtime schema selection must never be persisted or restored automatically.
- Existing profiles containing legacy `connection.database` remain readable, but that value must not become the runtime execution target.
- `Run` must fail before executing scripts when no runtime schema is supplied.

---

### Task 1: Profile validation and database connection boundaries

**Files:**
- Modify: `internal/profile/validate.go`
- Modify: `internal/database/mysql.go`
- Modify/Test: `internal/profile/validate_test.go`
- Modify/Test: `internal/database/mysql_test.go`

**Interfaces:**
- Produces: `database.Probe(ctx, connection) (*Client, error)` for server-only connection.
- Produces: `database.ConnectToDatabase(ctx, connection, schema) (*Client, error)` for execution.
- Produces: `(*database.Client).Schemas(ctx) ([]string, error)` using `SHOW DATABASES`.

- [ ] **Step 1: Add failing profile validation test**

Add a test that builds an otherwise valid profile with `Connection.Database == ""` and asserts `Validate(profile) == nil`.

- [ ] **Step 2: Run profile tests and verify failure**

Run: `go test ./internal/profile`

Expected before implementation: FAIL because validation reports `connection database is required`.

- [ ] **Step 3: Remove database-required validation**

Delete only the `Connection.Database` required check from `Validate`; keep host, port and username validation unchanged.

- [ ] **Step 4: Add database adapter tests**

Cover these behaviors:

```go
// probe config has an empty DBName
cfg := buildConfig(connection, "")
if cfg.DBName != "" { t.Fatalf(...) }

// execution config uses runtime schema rather than legacy profile value
cfg := buildConfig(connection, "runtime_schema")
if cfg.DBName != "runtime_schema" { t.Fatalf(...) }
```

Add a schema discovery test using a SQL mock/controlled DB seam already used by the package; expected query is exactly `SHOW DATABASES` and returned values preserve server order.

- [ ] **Step 5: Split server probe from execution connection**

Refactor `internal/database/mysql.go` so server capability detection is performed on a connection with empty DB name. Add an explicit database-specific open path taking `schema string`. `Schemas` queries `SHOW DATABASES`, scans one string per row, closes rows, checks `rows.Err()`, and wraps errors with context.

- [ ] **Step 6: Run package tests**

Run: `go test ./internal/profile ./internal/database`

Expected: PASS.

---

### Task 2: Application service, executor options and bridge DTO

**Files:**
- Modify: `internal/executor/types.go`
- Modify: `internal/app/service.go`
- Modify: `internal/ui/bridge.go`
- Modify: `internal/ui/wails/desktop_app.go`
- Modify/Test: `internal/app/service_test.go`
- Modify/Test: `internal/ui/bridge_test.go`

**Interfaces:**
- Produces: `database.ConnectionResult{Capabilities database.ServerCapabilities, Schemas []string}`.
- Produces: `executor.RunOptions.Schema string` serialized as `schema`.
- Changes: `TestConnection(context.Context, string) (database.ConnectionResult, error)`.

- [ ] **Step 1: Add failing service tests**

Test that connection testing returns both capabilities and schemas, and that `RunProfile` rejects `RunOptions{Schema: ""}` before invoking the executor/connecting to a database.

- [ ] **Step 2: Run service tests and verify failure**

Run: `go test ./internal/app ./internal/ui`

Expected before implementation: compile/test failure because `ConnectionResult` and `RunOptions.Schema` do not exist.

- [ ] **Step 3: Add runtime schema DTO/options**

Add:

```go
type ConnectionResult struct {
    Capabilities ServerCapabilities `json:"capabilities"`
    Schemas      []string           `json:"schemas"`
}
```

and:

```go
type RunOptions struct {
    Schema          string                  `json:"schema"`
    OnError         profile.OnError         `json:"onError,omitempty"`
    TransactionMode profile.TransactionMode `json:"transactionMode,omitempty"`
}
```

- [ ] **Step 4: Update application service**

`TestConnection` loads the profile, probes the server without a DB, loads schemas on that connection, and returns `ConnectionResult`. `RunProfile` trims `opts.Schema`; if empty return `schema is required`; otherwise open execution connection with exactly that schema and then call `executor.Run`.

Legacy `p.Connection.Database` must not be consulted to select the execution database.

- [ ] **Step 5: Update bridge and Wails surface**

Change `ServicePort`, `Bridge.TestConnection`, and `DesktopApp.TestConnection` to return `database.ConnectionResult`. Keep `RunProfile` signature but it now accepts the schema-bearing options DTO.

- [ ] **Step 6: Run service/UI tests**

Run: `go test ./internal/app ./internal/ui/...`

Expected: PASS.

---

### Task 3: CLI explicit runtime schema

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/connection.go`
- Modify: `internal/cli/run.go`
- Modify/Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `TestConnection(...) (database.ConnectionResult, error)`.
- Consumes: `executor.RunOptions.Schema`.
- Produces: required CLI option `runner run <profile-id> --schema <name>`.

- [ ] **Step 1: Add failing CLI test**

Assert running without `--schema` exits with code 1 and reports that schema is required. Assert running with `--schema app_db` passes `RunOptions.Schema == "app_db"` to the fake service.

- [ ] **Step 2: Run CLI tests and verify failure**

Run: `go test ./internal/cli`

Expected before implementation: FAIL because the flag is not defined/required.

- [ ] **Step 3: Add schema flag and adapt connection output**

In `newRunCommand`, declare `schema string`, register:

```go
command.Flags().StringVar(&schema, "schema", "", "database/schema to execute against")
_ = command.MarkFlagRequired("schema")
```

Set `opts.Schema = strings.TrimSpace(schema)` before calling the service.

`connection test` reads server version from `result.Capabilities` and otherwise keeps its existing concise output.

- [ ] **Step 4: Update CLI service interface**

Change `TestConnection` return type in `internal/cli/root.go` to `database.ConnectionResult`.

- [ ] **Step 5: Run CLI tests**

Run: `go test ./internal/cli`

Expected: PASS.

---

### Task 4: Frontend runtime state and searchable schema selector

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/runner.ts`
- Modify: `frontend/src/state/useRunnerController.ts`
- Modify: `frontend/src/components/ProfileDialog.tsx`
- Create: `frontend/src/components/SchemaSelect.tsx`
- Modify: `frontend/src/App.tsx`
- Modify/Test: `frontend/src/components/ProfileDialog.test.tsx`
- Modify/Test: `frontend/src/state/useRunnerController.test.tsx`
- Modify/Test: `frontend/src/components/MainView.test.tsx`
- Create/Test: `frontend/src/components/SchemaSelect.test.tsx`

**Interfaces:**
- Produces: TypeScript `ConnectionResult { capabilities: ServerCapabilities; schemas: string[] }`.
- Produces controller fields `availableSchemas`, `selectedSchema`, `setSelectedSchema`.
- Consumes: `RunOptions.schema`.

- [ ] **Step 1: Add failing profile dialog test**

Assert the dialog contains no `Database / Schema` input and saves a profile when name, host, port and username are filled.

- [ ] **Step 2: Add failing controller tests**

Cover: successful connection populates schema list without auto-selecting; profile switch clears schemas/selection; failed test clears both; run sends selected schema; run is not attempted when schema is empty.

- [ ] **Step 3: Add failing SchemaSelect tests**

Render schemas `apollo`, `mysql`, `TestDB`; type `test`; assert only `TestDB` remains selectable and selection calls `onChange("TestDB")`. Filtering must be case-insensitive.

- [ ] **Step 4: Run frontend tests and verify failure**

Run: `cd frontend && npm test`

Expected before implementation: FAIL.

- [ ] **Step 5: Update frontend API types**

Make `Connection.database` optional/legacy-compatible (`database?: string`) or keep it as an optional empty string-compatible property if storage DTO generation requires the key. Add `ConnectionResult`, add required `schema: string` to `RunOptions`, and change `testConnection` to return `ConnectionResult`.

- [ ] **Step 6: Remove database input from profile editor**

`emptyProfile` initializes no meaningful database target. Submission validation becomes `Name, host and username are required.` and does not trim/validate database.

- [ ] **Step 7: Implement `SchemaSelect` without dependencies**

Use a controlled text input plus an absolutely-positioned filtered listbox. Normalize filter text with `.toLocaleLowerCase()` and compare schema names case-insensitively. Disable it until schemas are available. Selecting an option updates the visible input and calls `onChange`.

- [ ] **Step 8: Add controller runtime state**

Add `availableSchemas` and `selectedSchema`; clear them in `applySelectedProfile` and on failed connection. On successful test set capabilities from `result.capabilities`, schemas from `result.schemas`, and leave selection null. `run()` returns early and sets an error when no schema is selected; otherwise calls `api.runProfile` with `{ schema: selectedSchema, onError, transactionMode }`.

- [ ] **Step 9: Wire selector into `App.tsx`**

Change connection summary to only `host:port`. Render `SchemaSelect` in the connection toolbar after the connection status/test controls. Disable `Run` when no schema is selected. Do not write schema into the profile.

- [ ] **Step 10: Run frontend tests and typecheck**

Run:

```bash
cd frontend
npm test
npx tsc -b
```

Expected: PASS.

---

### Task 5: Full regression and release-build verification

**Files:**
- No new production files expected.

**Interfaces:**
- Verifies all previous tasks together.

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend tests and production build**

Run:

```bash
cd frontend
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 3: Run Windows/Wails build in CI-compatible environment**

Run: `wails build -clean -webview2 embed -windowsconsole`

Expected: `build/bin/go-script-sql-runner.exe` exists.

- [ ] **Step 4: Commit implementation**

Commit the production/test changes with a focused message such as:

```bash
git commit -m "feat: select schema at runtime"
```

The implementation commit must not include persisted runtime schema state or a new frontend dependency.
