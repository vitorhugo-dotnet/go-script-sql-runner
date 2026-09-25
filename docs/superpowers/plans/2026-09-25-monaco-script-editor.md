# Monaco SQL Script Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use Markdown task checkboxes.

**Goal:** Add an in-app Monaco editor that safely reads and updates SQL text for an existing profile script.

**Architecture:** The Go repository reads and atomically replaces a script file based on validated profile metadata. The application service, bridge, and Wails desktop facade expose content-only methods; React opens a dirty-aware Monaco dialog and saves through the typed RunnerApi.

**Tech Stack:** Go, Wails v2, React 19, TypeScript, Monaco Editor, Vite, Vitest, React Testing Library.

**Spec:** [2026-09-25-profile-script-editor-and-windows-installer-design.md](../specs/2026-09-25-profile-script-editor-and-windows-installer-design.md)

## Global Constraints

- Monaco Editor is configured for SQL syntax highlighting.
- The editor loads content through the Wails API and saves it through the Go application service to the existing profile-owned SQL file.
- The frontend will not write files directly.
- Saving changes only the script content; its profile, ID, name, order, enabled state, transaction mode, and file reference remain unchanged.
- Closing a modified editor without saving asks the user to confirm discarding the changes.
- Editing is unavailable while a profile run is active.

## Review Focus

- Invalid IDs and ../-style script file metadata must not read or overwrite files outside a profile; pin in the storage path-validation tests.
- Missing profile/script and non-regular/symlink SQL files must return errors without creating files; pin in repository and service tests.
- Saving empty text and non-ASCII/multiline SQL must preserve exact bytes and UTF-8 text; pin in repository tests.
- A failed save must leave the original SQL intact and preserve script metadata; pin in atomic write and service tests.
- Dirty close via Cancel/Escape must preserve the draft until the user explicitly discards; pin in editor dialog tests.

---

### Task 1: Add safe SQL content operations in storage and application service

**Files:**
- Modify: internal/storage/repository.go
- Modify: internal/storage/repository_test.go
- Create: internal/app/script_content.go
- Create: internal/app/script_content_test.go

**Interfaces:**
- Repository.ReadScriptContent(profileID, scriptID) returns (string, error).
- Repository.WriteScriptContent(profileID, scriptID, content) returns error.
- Service.GetScriptContent(ctx, profileID, scriptID) returns (string, error).
- Service.SaveScriptContent(ctx, profileID, scriptID, content) returns error.

- [ ] **Step 1: Add repository read/write tests**

Use a temporary repository with a profile containing a real SQL script. Assert read returns exact text; writing the UTF-8 multiline text "-- Café\nSELECT 2;\n" changes only the SQL bytes and preserves all metadata. Also test saving empty text, missing profile, missing script ID, unknown script, malformed file reference ../escape.sql, and a symlink to an outside file where the platform permits symlinks.

- [ ] **Step 2: Run the focused repository tests**

Run: go test ./internal/storage -run ScriptContent -count=1

Expected: current implementation lacks these methods and the new tests fail to compile.

- [ ] **Step 3: Implement a validated path resolver and atomic writes**

Resolve the profile through Get, find the exact script ID, validate its relative file reference with the profile path rules, and join it below the profile's scripts directory. Reject any symlink component and any non-regular final file before reading or writing. Reads use os.ReadFile; writes call existing atomicWrite(path, []byte(content), 0o600). Never accept a frontend-supplied path.

- [ ] **Step 4: Add application service tests and methods**

Use editingService(t) in the existing profile editing test fixture or a local fixture. Assert GetScriptContent and SaveScriptContent return exact content and that saving leaves every profile.Script field unchanged. Assert cancelled contexts and unknown IDs fail without changing content. Implement the context checks and wrap repository errors with %w.

- [ ] **Step 5: Run the backend script content tests**

Run: go test ./internal/storage ./internal/app -run 'ScriptContent|Script.*Content' -count=1

Expected: PASS, including exact empty and UTF-8 content round trips.

- [ ] **Step 6: Commit safe SQL content operations**

```powershell
git add internal/storage/repository.go internal/storage/repository_test.go internal/app/script_content.go internal/app/script_content_test.go
git commit -m "feat: add safe SQL script content editing"
```

### Task 2: Expose script content calls to the Wails frontend

**Files:**
- Modify: internal/ui/bridge.go
- Modify: internal/ui/bridge_test.go
- Modify: internal/ui/wails/desktop_app.go
- Modify: frontend/src/api/types.ts
- Modify: frontend/src/api/runner.ts
- Modify: frontend/src/state/useRunnerController.ts
- Modify: frontend/src/state/useRunnerController.test.tsx

**Interfaces:**
- ServicePort consumes GetScriptContent(context.Context, string, string) (string, error) and SaveScriptContent(context.Context, string, string, string) error.
- Bridge exposes GetScriptContent(ctx, profileID, scriptID) (string, error) and SaveScriptContent(ctx, profileID, scriptID, content) error.
- RunnerApi exposes getScriptContent(profileID, scriptID): Promise<string> and saveScriptContent(profileID, scriptID, content): Promise<void>.
- The controller exposes loadScriptContent(scriptID): Promise<string | null> and saveScriptContent(scriptID, content): Promise<boolean>.

- [ ] **Step 1: Add bridge forwarding and error tests**

Extend the fake service and assert profile ID, script ID, and exact text are passed unchanged for read/write. Assert service errors are propagated. Update existing bridge service fakes for the expanded ServicePort interface.

- [ ] **Step 2: Add controller tests**

Assert loading returns the selected script text. Assert save calls the selected profile ID and script ID, refreshes the selected profile on success, returns true, and reports errors while returning false on failure. Assert no selected profile prevents an API call.

- [ ] **Step 3: Run API tests before implementation**

Run: go test ./internal/ui -run ScriptContent -count=1

Run in frontend: npm test -- --run src/state/useRunnerController.test.tsx

Expected: FAIL because the new service methods and RunnerApi/controller actions have not been added.

- [ ] **Step 4: Implement bridge, Wails, and frontend adapters**

Add DesktopApp.GetScriptContent and DesktopApp.SaveScriptContent, each retrieving the startup context and delegating through Bridge. Add both methods to the desktop binding interface and Wails adapter. Implement the controller methods using existing error handling and refreshSelectedProfile patterns.

- [ ] **Step 5: Run Go bridge tests and frontend controller tests**

Run: go test ./internal/ui ./internal/ui/wails -count=1

Run in frontend: npm test -- --run src/state/useRunnerController.test.tsx

Expected: PASS; the script is refreshed after save and backend errors remain visible.

- [ ] **Step 6: Commit script content API wiring**

```powershell
git add internal/ui/bridge.go internal/ui/bridge_test.go internal/ui/wails/desktop_app.go frontend/src/api/types.ts frontend/src/api/runner.ts frontend/src/state/useRunnerController.ts frontend/src/state/useRunnerController.test.tsx
git commit -m "feat: expose SQL content editing through Wails"
```

### Task 3: Build and test the Monaco editor dialog

**Files:**
- Modify: frontend/package.json
- Modify: frontend/package-lock.json
- Modify: frontend/vite.config.ts
- Create: frontend/src/components/ScriptEditorDialog.tsx
- Create: frontend/src/components/ScriptEditorDialog.test.tsx
- Modify: frontend/src/App.tsx
- Modify: frontend/src/components/MainView.test.tsx

**Interfaces:**
- ScriptEditorDialog receives open, scriptName, initialContent, saving, onCancel, and onSave(content): Promise<boolean> props; true means saved and false keeps the editor open with its draft.
- App stores the script being edited, loads content through the controller, and saves through the controller.

- [ ] **Step 1: Install and lock Monaco dependencies**

Run in frontend: npm install @monaco-editor/react monaco-editor

Expected: package.json and package-lock.json pin resolved dependencies for reproducible npm ci builds.

- [ ] **Step 2: Add dialog behavior tests with a Monaco test double**

Mock @monaco-editor/react as an accessible textarea-like editor that forwards value and onChange. Test loading initial SQL, editing multiline text, Save calling onSave with exact content, closing only when onSave resolves true, keeping the editor and draft open when onSave resolves false, disabled Save while saving, Cancel without edits, and dirty close prompting to discard. If the user declines discard, keep the editor and draft open; if confirmed, close it.

- [ ] **Step 3: Add main-view editor lifecycle tests**

In MainView.test.tsx, assert each row has an Edit action naming the script, clicking loads that script's SQL into the dialog, successful save calls saveScriptContent and closes the dialog, and the action is disabled during a run. Assert backend load/save errors appear in the existing error area and preserve the dialog draft on save failure.

- [ ] **Step 4: Run the editor tests before implementation**

Run in frontend: npm test -- --run src/components/ScriptEditorDialog.test.tsx src/components/MainView.test.tsx

Expected: FAIL because ScriptEditorDialog and the script-row action have not been implemented yet; failures should identify missing behavior, not test setup errors.

- [ ] **Step 5: Implement Monaco dialog and local bundle configuration**

Render Monaco Editor with language sql, the current draft value, and an onChange handler in a labeled dialog with a minimum height of 420px, dark theme matching the app, and accessible Save/Cancel/close controls. Configure Monaco workers or Vite asset handling so editor assets are bundled locally and no CDN/network is required at runtime. Keep the original value to detect dirty state. When the draft is dirty, show an in-app alert dialog with Keep editing and Discard changes actions; do not use window.confirm. Close after save only when onSave resolves true.

- [ ] **Step 6: Implement the script row action and editor lifecycle**

In App.tsx, add an Edit action per script beside the ordering controls. Disable it during a run. Load selected SQL via the controller before opening the dialog, save through saveScriptContent, close only after successful save, and keep the draft/dialog open on save failure. Backend errors appear in the existing error area.

- [ ] **Step 7: Run frontend editor tests, typecheck, and production build**

Run in frontend: npm test -- --run src/components/ScriptEditorDialog.test.tsx src/components/MainView.test.tsx src/state/useRunnerController.test.tsx

Run in frontend: npx tsc -b

Run in frontend: npm run build

Expected: PASS; the build emits the Monaco editor and worker assets into frontend/dist without remote loading.

- [ ] **Step 8: Commit Monaco editor integration**

```powershell
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/src/components/ScriptEditorDialog.tsx frontend/src/components/ScriptEditorDialog.test.tsx frontend/src/App.tsx frontend/src/components/MainView.test.tsx
git commit -m "feat: add Monaco SQL script editor"
```
