# Desktop UI and Packaging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the Wails desktop GUI, bind it to the existing Go services, add persistent logging, and package the same Windows executable for both GUI and CLI use.

**Architecture:** A thin Wails backend adapts native dialogs and execution events to the shared `app.Service`; React never touches MySQL or AppData directly. The frontend uses a small typed API adapter plus React state/hooks and Tailwind utilities. The Windows build uses the console subsystem so CLI output works, then hides a console created only for Explorer-launched GUI sessions; the release build embeds the WebView2 bootstrapper.

**Tech Stack:** Go 1.26, Wails v2.13.0, React + TypeScript, Vite, Tailwind CSS v4, Wails Go runtime dialogs/events, Vitest + React Testing Library, `golang.org/x/sys/windows` for minimal Windows console handling.

## Global Constraints

- Initial window approximately 680×540 px; minimum approximately 560×420 px; resizing stays enabled.
- Main screen must expose profile, connection/version, ordered scripts, run control, failure policy, transaction mode, import/export, and compact bottom logs without requiring a large window.
- UI is modern and dense, but remains a native framed Windows application; no custom frameless title bar.
- SQL and profile edits always flow through Go application services; frontend never writes AppData directly.
- Native Wails v2 Go dialogs are used for `.sql` selection and profile ZIP open/save because Wails v2 does not expose those dialogs in its JS runtime.
- Execution events flow from Go to React using Wails `EventsEmit` / `EventsOn`.
- Passwords may be edited in the profile dialog but must never appear in logs, connection summary text, or run diagnostics.
- Same imported profile ID requires an overwrite warning before the already-defined replacement operation.
- Release build uses `-webview2 embed` and one `.exe` artifact.

## File Map

```text
main.go                                      # choose CLI vs GUI
 desktop.go                                  # Wails app options/assets/run
internal/bootstrap/bootstrap.go              # production wiring/cleanup
internal/logging/file_sink.go                # persistent slog-backed execution event sink
internal/logging/file_sink_test.go
internal/platform/console_policy.go           # pure hide/no-hide policy
internal/platform/console_windows.go          # Windows console syscall adapter
internal/platform/console_other.go            # test/non-Windows no-op
internal/platform/console_policy_test.go
internal/app/profile_editing.go               # GUI-required profile/script update use cases
internal/app/profile_editing_test.go
internal/desktop/backend.go                   # Wails-bound methods/native dialogs
internal/desktop/backend_test.go              # backend behavior without rendering WebView
internal/desktop/event_sink.go                # executor event -> Wails event bridge
frontend/src/api/types.ts                     # frontend DTO types
frontend/src/api/runner.ts                    # generated-binding/runtime adapter
frontend/src/state/useRunnerController.ts     # screen orchestration/state
frontend/src/components/Toolbar.tsx
frontend/src/components/ConnectionBar.tsx
frontend/src/components/ExecutionControls.tsx
frontend/src/components/ScriptList.tsx
frontend/src/components/LogPanel.tsx
frontend/src/components/ProfileDialog.tsx
frontend/src/components/ImportConflictDialog.tsx
frontend/src/App.tsx
frontend/src/style.css
frontend/src/test/setup.ts
frontend/src/**/*.test.tsx
scripts/build.ps1                             # reproducible Windows release build
README.md                                     # public/sanitized usage documentation
```

---

### Task 1: Persistent logs and production dependency bootstrap

**Files:**
- Create: `internal/logging/file_sink.go`
- Create: `internal/logging/file_sink_test.go`
- Create: `internal/bootstrap/bootstrap.go`
- Modify: `internal/app/service.go`
- Modify: `main.go`

**Interfaces:**
- Produces `logging.NewFileSink(logDir string) (*FileSink, error)` implementing `executor.Sink` and `io.Closer`.
- Produces `bootstrap.NewDefault() (*Runtime, error)` where `Runtime` owns `Service` and closers.
- Ensures both CLI and GUI runs persist already-redacted execution events under AppData.

- [ ] **Step 1: Write failing persistent sink tests**

Use `t.TempDir()` and emit three synthetic events. Assert a log file named with the local date exists under the supplied directory and contains level/script/message but not a supplied synthetic password.

Required filename shape:

```text
runner-YYYY-MM-DD.log
```

- [ ] **Step 2: Implement a `slog`-backed file sink**

Create the directory with `0700`, open the daily file with append/create/write, and construct:

```go
logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

Map executor levels to `logger.Info`, `logger.Warn`, and `logger.Error`. Store `script_id`, `message`, and `detail` as structured attributes. The sink assumes incoming event detail has already been redacted; tests must still ensure known secrets are never passed when bootstrap wires the service.

- [ ] **Step 3: Extend `app.Service` to fan out run events**

Add a persistent sink dependency to the service constructor. `RunProfile` combines the persistent sink with the caller-provided sink using a private fan-out sink; nil caller sinks are allowed.

Do not duplicate execution logging in CLI or GUI.

- [ ] **Step 4: Add production bootstrap**

`bootstrap.NewDefault()` must:

```text
storage.DefaultRoot()
-> create AppData root/profiles/logs directories
-> storage.NewRepository(root)
-> logging.NewFileSink(<root>/logs)
-> app.NewService(repository, persistentSink, database connector)
```

`Runtime.Close()` closes the file sink and returns `errors.Join` of cleanup errors.

- [ ] **Step 5: Verify logging/bootstrap tests**

```powershell
go test ./internal/logging ./internal/bootstrap ./internal/app
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/logging internal/bootstrap internal/app main.go
git commit -m "feat: persist runner execution logs"
```

---

### Task 2: GUI-required profile editing use cases

**Files:**
- Create: `internal/app/profile_editing.go`
- Create: `internal/app/profile_editing_test.go`

**Interfaces:**
- Adds update/reorder/enable operations without exposing raw filesystem mutation to React.

- [ ] **Step 1: Write failing service tests**

Add exact methods:

```go
func (s *Service) UpdateProfile(ctx context.Context, p profile.Profile) (profile.Profile, error)
func (s *Service) ReorderScripts(ctx context.Context, profileID string, orderedIDs []string) (profile.Profile, error)
func (s *Service) SetScriptEnabled(ctx context.Context, profileID, scriptID string, enabled bool) (profile.Profile, error)
func (s *Service) SetScriptTransactionMode(ctx context.Context, profileID, scriptID string, mode profile.TransactionMode) (profile.Profile, error)
```

Tests require `UpdateProfile` to preserve the existing profile ID and script file references, `ReorderScripts` to reject missing/duplicate IDs, and script setters to reject unknown IDs.

- [ ] **Step 2: Implement edits through the repository only**

`ReorderScripts` assigns deterministic `Order` values `10, 20, 30...` in the submitted ID order. This avoids order collisions while leaving insertion space for future changes.

`SetScriptTransactionMode(..., "")` restores profile-default inheritance; non-empty modes use existing validation enums.

- [ ] **Step 3: Run tests**

```powershell
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add internal/app/profile_editing.go internal/app/profile_editing_test.go
git commit -m "feat: add profile editing use cases"
```

---

### Task 3: Wails backend facade, native file dialogs, and execution event bridge

**Files:**
- Create: `internal/desktop/backend.go`
- Create: `internal/desktop/backend_test.go`
- Create: `internal/desktop/event_sink.go`
- Create: `desktop.go`

**Interfaces:**
- Produces a Wails-bound `desktop.Backend` that adapts frontend calls to `app.Service`.
- Emits `runner:execution-event` for every executor event.

- [ ] **Step 1: Define backend methods and write service-delegation tests**

Expose these methods to generated Wails bindings:

```go
func (b *Backend) ListProfiles() ([]profile.Profile, error)
func (b *Backend) GetProfile(id string) (profile.Profile, error)
func (b *Backend) CreateProfile(p profile.Profile) (profile.Profile, error)
func (b *Backend) UpdateProfile(p profile.Profile) (profile.Profile, error)
func (b *Backend) RemoveScript(profileID, scriptID string) error
func (b *Backend) ReorderScripts(profileID string, orderedIDs []string) (profile.Profile, error)
func (b *Backend) SetScriptEnabled(profileID, scriptID string, enabled bool) (profile.Profile, error)
func (b *Backend) SetScriptTransactionMode(profileID, scriptID string, mode profile.TransactionMode) (profile.Profile, error)
func (b *Backend) TestConnection(profileID string) (database.ServerCapabilities, error)
func (b *Backend) RunProfile(profileID string, opts executor.RunOptions) (executor.Summary, error)
func (b *Backend) InspectProfileArchive(path string) (profile.ArchiveInspection, error)
func (b *Backend) ImportProfile(path string, overwrite bool) (profile.Profile, error)
func (b *Backend) ExportProfile(profileID, destination string) error
```

Store the Wails runtime context in:

```go
func (b *Backend) Startup(ctx context.Context) { b.ctx = ctx }
```

Use an interface for the application service in backend unit tests so no WebView is needed.

- [ ] **Step 2: Add native picker methods using Wails v2 Go runtime**

Exact methods:

```go
func (b *Backend) PickSQLFiles() ([]string, error)
func (b *Backend) PickProfileArchive() (string, error)
func (b *Backend) PickExportDestination(defaultName string) (string, error)
```

Use:

```go
runtime.OpenMultipleFilesDialog(b.ctx, runtime.OpenDialogOptions{
    Title: "Add SQL scripts",
    Filters: []runtime.FileFilter{{DisplayName: "SQL (*.sql)", Pattern: "*.sql"}},
})
```

Use `OpenFileDialog` with `*.zip` for import and `SaveFileDialog` with a default `.zip` filename for export.

Add a convenience backend method:

```go
func (b *Backend) AddSQLFiles(profileID string, paths []string) ([]profile.Script, error)
```

that calls the existing application `AddScript` for each selected path sequentially and returns successfully added scripts or the first error.

- [ ] **Step 3: Bridge execution events to Wails**

`event_sink.go`:

```go
type eventSink struct{ ctx context.Context }

func (s eventSink) Emit(event executor.Event) {
    runtime.EventsEmit(s.ctx, "runner:execution-event", event)
}
```

`Backend.RunProfile` uses this sink. Protect runs with `sync.Mutex.TryLock`; if a run is already active return a typed `ErrRunInProgress` instead of launching two concurrent profile executions against the same application instance.

- [ ] **Step 4: Configure the actual Wails window**

`desktop.go` embeds `frontend/dist` and calls `wails.Run` with:

```go
&options.App{
    Title:         "Go Script SQL Runner",
    Width:         680,
    Height:        540,
    MinWidth:      560,
    MinHeight:     420,
    DisableResize: false,
    Frameless:     false,
    AssetServer:   &assetserver.Options{Assets: assets},
    OnStartup:     backend.Startup,
    Bind:          []interface{}{backend},
}
```

Do not maximize by default.

- [ ] **Step 5: Generate bindings and verify backend build**

Run:

```powershell
wails generate module
go test ./internal/desktop
wails build -clean -webview2 embed
```

If the installed Wails CLI does not provide `generate module` for this scaffold, run `wails dev` once to regenerate `frontend/wailsjs`, then stop it after bindings exist. Do not hand-write generated bindings.

- [ ] **Step 6: Commit**

```powershell
git add internal/desktop desktop.go frontend/wailsjs
git commit -m "feat: bind runner services to Wails"
```

---

### Task 4: Frontend typed adapter, test harness, and screen state controller

**Files:**
- Create: `frontend/src/api/types.ts`
- Create: `frontend/src/api/runner.ts`
- Create: `frontend/src/state/useRunnerController.ts`
- Create: `frontend/src/state/useRunnerController.test.tsx`
- Create: `frontend/src/test/setup.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/vite.config.ts`

**Interfaces:**
- Produces a `RunnerApi` interface that components can fake in tests.
- Produces a controller with profile/run/log state and actions; components remain mostly presentational.

- [ ] **Step 1: Install frontend test dependencies**

From `frontend`:

```powershell
npm install -D vitest jsdom @testing-library/react @testing-library/jest-dom @testing-library/user-event
```

Add scripts:

```json
{
  "test": "vitest run",
  "test:watch": "vitest"
}
```

Configure Vitest in `vite.config.ts` with `environment: 'jsdom'` and `setupFiles: './src/test/setup.ts'`.

- [ ] **Step 2: Define frontend domain/API types**

`RunnerApi` must expose Promise-based versions of the backend methods plus:

```ts
onExecutionEvent(handler: (event: ExecutionEvent) => void): () => void
```

The production adapter imports generated Go bindings and `EventsOn` from `frontend/wailsjs/runtime/runtime`. Return the cleanup function from `EventsOn`.

Never build MySQL DSNs or profile filesystem paths in TypeScript.

- [ ] **Step 3: Write failing controller tests**

With a fake `RunnerApi`, require:

1. initial load selects the first profile;
2. selecting a profile loads its full data;
3. `testConnection` stores detected `VersionLabel`;
4. execution events append to a bounded in-memory log list;
5. `run` sets `running=true`, waits for result, then resets it even on failure;
6. profile refresh after import selects the imported profile;
7. changing stop/continue or transaction mode for a run changes runtime options without silently mutating saved profile until the user saves profile settings.

Bound in-memory logs to the newest 1000 events so a long script run cannot grow the WebView indefinitely.

- [ ] **Step 4: Implement `useRunnerController`**

Use React hooks/reducer only; do not add Redux/Zustand for this small application. Subscribe to `runner:execution-event` on mount and unsubscribe on cleanup.

Expose action names:

```ts
loadProfiles
selectProfile
saveProfile
addSQLFiles
removeScript
moveScript
setScriptEnabled
setScriptTransactionMode
testConnection
run
importProfile
exportProfile
clearLogs
```

- [ ] **Step 5: Run controller tests**

```powershell
cd frontend
npm test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/src/api frontend/src/state frontend/src/test
git commit -m "feat: add typed desktop UI state"
```

---

### Task 5: Compact responsive main window and log panel

**Files:**
- Create: `frontend/src/components/Toolbar.tsx`
- Create: `frontend/src/components/ConnectionBar.tsx`
- Create: `frontend/src/components/ExecutionControls.tsx`
- Create: `frontend/src/components/ScriptList.tsx`
- Create: `frontend/src/components/LogPanel.tsx`
- Create: `frontend/src/components/MainView.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/style.css`

**Interfaces:**
- Consumes the controller from Task 4.
- Produces the required default 680×540 information-dense main screen.

- [ ] **Step 1: Write failing rendering/interaction tests**

Render with a fake API/profile and assert these controls are visible without navigating to another page:

```text
Profile selector
New/Edit
Import
Export
Connection summary
Test Connection
Detected version/status
Script list
Add SQL
Failure policy
Transaction mode
Run
Log panel
```

Test script enable checkbox, move up/down actions, remove action, and `Run` disabled while `running=true`.

- [ ] **Step 2: Implement the compact application shell**

Use a full-height CSS grid:

```text
row 1: compact toolbar
row 2: connection/execution controls
row 3: flexible script list
row 4: compact log panel
```

The main content must use `min-h-0` / scroll containers so the 560×420 minimum does not push essential controls outside the window.

Use native Windows-friendly font stack in `style.css`:

```css
@import "tailwindcss";

html, body, #root { height: 100%; }
body {
  margin: 0;
  font-family: "Segoe UI Variable", "Segoe UI", system-ui, sans-serif;
}
```

Use Tailwind utilities for the rest. Avoid animation beyond short hover/focus transitions.

- [ ] **Step 3: Implement `Toolbar` and `ConnectionBar`**

Toolbar layout:

```text
[Profile ▼] [New] [Edit]                         [Import] [Export]
```

Connection bar must display only safe summary fields:

```text
host:port / database     [Disconnected|MySQL 5.7|MySQL 8.x] [Test]
```

Never display password.

- [ ] **Step 4: Implement execution controls and script list**

Execution controls use compact `<select>` elements for:

```text
On failure: Continue | Stop
Transaction: Auto commit | Transaction | Script managed
```

Script rows include checkbox, name, per-script transaction override, up/down buttons, status, and remove. Use up/down buttons rather than a drag-and-drop dependency; the result is deterministic, keyboard-friendly, and cheaper to maintain.

- [ ] **Step 5: Implement bottom log panel**

Default height approximately 120–140px with internal scrolling. Each row shows compact timestamp, level, script/name context, and message. Add:

```text
[Detailed logs checkbox] [Clear]
```

When detailed logging is off, hide `Detail`; do not discard it from the controller so toggling on reveals existing diagnostics.

- [ ] **Step 6: Run frontend tests/build**

```powershell
cd frontend
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add frontend/src/components frontend/src/App.tsx frontend/src/style.css
git commit -m "feat: build compact runner workspace"
```

---

### Task 6: Profile editor and profile import/export dialogs

**Files:**
- Create: `frontend/src/components/ProfileDialog.tsx`
- Create: `frontend/src/components/ProfileDialog.test.tsx`
- Create: `frontend/src/components/ImportConflictDialog.tsx`
- Create: `frontend/src/components/ImportConflictDialog.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/state/useRunnerController.ts`

**Interfaces:**
- Allows complete connection/profile configuration from the GUI.
- Implements explicit overwrite and plaintext-credential export warnings.

- [ ] **Step 1: Write failing profile editor tests**

Use the native HTML `<dialog>` element. Required fields:

```text
Name
Host
Port
Database / Schema
Username
Password
Default failure policy
Default transaction mode
```

Test create and edit submission. Password input must use `type="password"` by default.

- [ ] **Step 2: Implement profile dialog with local draft state**

Do not save on every keystroke. Submit one validated draft to `CreateProfile` or `UpdateProfile`; keep the dialog open and show the backend error when save fails.

Use explicit labels, `aria-describedby` for validation messages, autofocus on name when supported, and Escape/cancel behavior from `<dialog>`.

- [ ] **Step 3: Write failing import conflict/export warning tests**

Import flow test:

```text
Pick ZIP -> Inspect -> ID conflict -> warning dialog -> confirm -> Import(overwrite=true)
```

Warning copy must clearly state that existing SQL/configuration for that profile ID will be completely replaced.

Export flow must show this warning before opening the destination picker:

```text
This profile contains database credentials. The exported ZIP is not encrypted.
```

- [ ] **Step 4: Implement import/export flows**

Import:

```text
PickProfileArchive
-> InspectProfileArchive
-> compare profile IDs with current list
-> conflict dialog if needed
-> ImportProfile(path, confirmedOverwrite)
-> refresh/select imported profile
```

Export:

```text
show credential warning
-> PickExportDestination(<safe-name>.zip)
-> ExportProfile(profileID, destination)
-> show non-blocking success status in the toolbar/log area
```

Do not add ZIP encryption/password UI.

- [ ] **Step 5: Verify all frontend tests**

```powershell
cd frontend
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add frontend/src/components frontend/src/state frontend/src/App.tsx
git commit -m "feat: add profile management dialogs"
```

---

### Task 7: Same-executable CLI/GUI Windows behavior

**Files:**
- Create: `internal/platform/console_policy.go`
- Create: `internal/platform/console_policy_test.go`
- Create: `internal/platform/console_windows.go`
- Create: `internal/platform/console_other.go`
- Modify: `main.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- No arguments -> Wails GUI.
- One or more arguments -> existing Cobra CLI in the same executable.
- Explorer-launched GUI does not leave a console window visible.

- [ ] **Step 1: Write pure console policy tests**

Define:

```go
func shouldHideStandaloneConsole(hasCLIArgs bool, attachedProcessCount uint32) bool {
    return !hasCLIArgs && attachedProcessCount == 1
}
```

Tests:

```go
require.True(t, shouldHideStandaloneConsole(false, 1))
require.False(t, shouldHideStandaloneConsole(false, 2))
require.False(t, shouldHideStandaloneConsole(true, 1))
```

This prevents hiding a terminal window shared with PowerShell/cmd.

- [ ] **Step 2: Add the Go Windows syscall package**

```powershell
go get golang.org/x/sys@v0.47.0
go mod tidy
```

- [ ] **Step 3: Implement Windows standalone-console detection/hide**

`console_windows.go` uses `windows.NewLazySystemDLL` for:

```text
kernel32!GetConsoleProcessList
kernel32!GetConsoleWindow
user32!ShowWindow
```

Call `GetConsoleProcessList` with a 2-element PID buffer. Only when it reports exactly one attached process and there are no CLI args, call `ShowWindow(hwnd, SW_HIDE)`.

Keep the unsafe/Win32 code entirely inside this build-tagged file:

```go
//go:build windows
```

`console_other.go` is a no-op under `//go:build !windows` so core tests remain runnable outside Windows.

- [ ] **Step 4: Make `main.go` perform final dispatch**

Required shape:

```go
func main() {
    rt, err := bootstrap.NewDefault()
    if err != nil { /* print fatal error and exit */ }
    defer rt.Close()

    if len(os.Args) > 1 {
        code := cli.Execute(context.Background(), rt.Service, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
        os.Exit(code)
    }

    platform.HideStandaloneConsole(false)
    if err := runDesktop(rt.Service); err != nil {
        // report fatal startup error without credentials
        os.Exit(1)
    }
}
```

Adapt the exact helper signature to expose the pure policy test cleanly, but preserve the behavior above.

- [ ] **Step 5: Build with console subsystem and manually test both launch paths**

Run:

```powershell
wails build -clean -trimpath -webview2 embed -windowsconsole -o go-script-sql-runner.exe
.\build\bin\go-script-sql-runner.exe profile list
.\build\bin\go-script-sql-runner.exe
```

Expected:

- terminal invocation prints CLI output and returns an exit code;
- no-argument invocation opens the Wails window;
- when double-clicked in Explorer, any standalone console created for the console-subsystem executable is immediately hidden rather than remaining behind the GUI.

- [ ] **Step 6: Commit**

```powershell
git add internal/platform main.go go.mod go.sum
git commit -m "feat: unify Windows CLI and GUI executable"
```

---

### Task 8: Release build script, sanitized README, and final acceptance pass

**Files:**
- Create: `scripts/build.ps1`
- Create/modify: `README.md`
- Modify: `.gitignore` if acceptance scan reveals a missing local pattern

**Interfaces:**
- Produces the final reproducible Windows `.exe` build command.
- Documents only synthetic examples.

- [ ] **Step 1: Add reproducible Windows build script**

`scripts/build.ps1`:

```powershell
$ErrorActionPreference = 'Stop'

Push-Location "$PSScriptRoot\.."
try {
    go test ./internal/...

    Push-Location frontend
    try {
        npm ci
        npm test
        npm run build
    }
    finally {
        Pop-Location
    }

    wails build `
        -clean `
        -trimpath `
        -webview2 embed `
        -windowsconsole `
        -o go-script-sql-runner.exe
}
finally {
    Pop-Location
}
```

Do not use UPX by default; fewer moving pieces and easier antivirus/reproducibility behavior wins here.

- [ ] **Step 2: Write a sanitized README**

README sections:

```text
Purpose
Requirements (Windows 10/11)
Build
AppData location
GUI usage
CLI usage
Profile import/export warning
MySQL compatibility
Transaction/DDL caveat
Repository security rules
```

Every example must use `127.0.0.1`, `example`, `dev`, and clearly synthetic passwords/scripts. Do not copy text/SQL from the uploaded company examples.

- [ ] **Step 3: Run the complete automated suite**

```powershell
go test ./internal/...
cd frontend
npm ci
npm test
npm run build
cd ..
docker compose -f test/integration/docker-compose.yml up -d --wait
go test -tags=integration ./test/integration -v
docker compose -f test/integration/docker-compose.yml down -v
.\scripts\build.ps1
```

Expected: all unit/frontend/integration tests pass and `build/bin/go-script-sql-runner.exe` exists.

- [ ] **Step 4: Perform final Windows GUI acceptance**

Verify manually at both 680×540 and 560×420:

1. essential toolbar/connection/run controls remain reachable;
2. script list and log panel scroll internally rather than expanding the window;
3. resize larger uses available space;
4. connection test shows detected 5.6/5.7/8 label;
5. run progress appears in bottom logs;
6. continue and stop policies behave as selected;
7. import same ID warns and then completely replaces after confirmation;
8. export warns about plaintext credentials;
9. no password appears in visible logs.

- [ ] **Step 5: Scan tracked content before completion**

Run:

```powershell
git status --short
git ls-files "*.sql" "*.zip" "*.log" "*.yaml" "*.yml"
git grep -n -I -E "(password|passwd|jdbc:mysql|10\.[0-9]+\.|192\.168\.)" -- ':!docs/superpowers/**'
```

Review every match. Only synthetic/test configuration may remain. The user-provided SQL examples and any company host/user/password/schema must have zero tracked occurrences.

- [ ] **Step 6: Commit final docs/build tooling**

```powershell
git add scripts/build.ps1 README.md .gitignore
git commit -m "docs: finalize Windows runner build"
```

- [ ] **Step 7: Tag readiness check without creating a release**

```powershell
git status --short
git log --oneline -10
```

Expected: clean working tree and the implementation commits from all three plans visible. Release/tag creation is intentionally outside this plan until the user requests it.
