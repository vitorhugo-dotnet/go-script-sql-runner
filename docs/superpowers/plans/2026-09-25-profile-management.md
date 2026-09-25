# Profile Delete and Clone Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users delete a selected profile with confirmation and clone a fully independent copy of a profile.

**Architecture:** Keep profile lifecycle operations in `internal/app.Service` and implement complete-directory behavior in `internal/storage.Repository`. Expose typed operations through `ui.Bridge` and `wails.DesktopApp`, then add controller state actions and confirmation dialogs in React.

**Tech Stack:** Go, Wails v2, React 19, TypeScript, Vitest, React Testing Library.

**Spec:** [2026-09-25-profile-script-editor-and-windows-installer-design.md](../specs/2026-09-25-profile-script-editor-and-windows-installer-design.md)

## Global Constraints

- Copy connection settings including the password, execution settings, all script metadata, and the SQL file contents into the new profile's directory.
- Editing or deleting the clone must not change the source profile.
- Confirmation precedes recursive profile removal.
- The frontend does not copy or remove filesystem paths.
- Application data remains per-user in AppData.

## Review Focus

- Invalid profile IDs, including path traversal strings, must be rejected before path resolution; pin in the repository deletion and clone tests.
- A clone with a missing or non-regular SQL source must fail without leaving a partial destination or changing the source; pin in the repository clone tests.
- Duplicate profile names must not overwrite a profile because IDs, not names, identify profiles; pin in the service clone test.
- Cancelling deletion must preserve the current profile and make no API call; pin in the main-view deletion test.
- Deleting the selected profile when it is the only one must close its active database connection and clear selection, while deleting one of several selects a remaining profile; pin connection behavior in the service test and selection behavior in the controller/UI test.

---

### Task 1: Add safe repository and service profile lifecycle operations

**Files:**
- Modify: `internal/storage/repository.go`
- Modify: `internal/storage/repository_test.go`
- Create: `internal/app/profile_lifecycle.go`
- Create: `internal/app/profile_lifecycle_test.go`

**Interfaces:**
- Produces `func (r *Repository) Clone(sourceID string, destination profile.Profile) error`.
- Produces `func (s *Service) DeleteProfile(ctx context.Context, profileID string) error`.
- Produces `func (s *Service) CloneProfile(ctx context.Context, profileID string) (profile.Profile, error)`; the service assigns a generated ID and the name `<source name> (copy)`.

- [ ] **Step 1: Add repository deletion tests for invalid IDs and profile isolation**

Extend `TestRepositoryCRUDAndLayout` with a second saved profile and assert deleting the first removes its profile directory while the second remains readable. Add table cases for `../outside`, `.`, and an empty ID; assert each returns an error and does not remove any directory outside `profiles`.

- [ ] **Step 2: Run the focused repository test**

Run: `go test ./internal/storage -run 'TestRepositoryCRUDAndLayout|TestDeleteRejectsInvalidIDs' -count=1`

Expected: PASS; invalid IDs are rejected and the other profile remains readable.

- [ ] **Step 3: Add repository clone tests**

Create source and destination candidates with two SQL scripts. Call `Clone(source.ID, clone)` and assert the destination YAML retains connection/password/execution/script metadata, each destination script has identical bytes, and subsequent writes/deletion in the destination do not affect source files. Assert an existing destination ID is rejected. Add missing-file and directory-valued SQL source cases and assert the destination path does not remain.

- [ ] **Step 4: Run the focused clone test before implementation**

Run: `go test ./internal/storage -run 'Repository.*Clone|CloneProfile' -count=1`

Expected: FAIL because Repository.Clone does not exist yet; the missing-source test should not fail during fixture setup.

- [ ] **Step 5: Implement transactional repository cloning**

Validate both IDs and the destination profile. Create a temporary staging directory under the profiles root, copy only regular SQL files named by source profile metadata after checking each path remains under that source profile, encode the destination profile into staging, then rename the complete staging directory to the destination. Remove staging on every error. Do not call `Save` on the final directory until all files are copied.

- [ ] **Step 6: Add service deletion and clone tests**

Add tests that `DeleteProfile` removes a profile and rejects invalid IDs, and `CloneProfile` creates a generated ID distinct from the source, appends ` (copy)` to the name, retains the password/settings/scripts, and does not collide when two profiles share the same name. Configure an active recording client for the profile being deleted and assert successful deletion closes and clears that connection; deleting another profile must leave the active connection open. Assert a cancelled context returns before mutation.

- [ ] **Step 7: Implement the service operations**

`CloneProfile` must check `ctx.Err()`, load the source, copy its value, clear the clone ID, set the derived name, call `id.New()`, preserve version and all other profile/script fields, then pass the completed clone to `Repository.Clone`. `DeleteProfile` checks context, takes `connectionMu` so an active run finishes first, deletes through the repository, then closes and clears the active client only when its profile ID matches the deleted profile. Preserve the connection when repository deletion fails. Wrap errors with operation and profile ID while preserving them with `%w`.

- [ ] **Step 8: Run backend lifecycle tests**

Run: `go test ./internal/storage ./internal/app -run 'Delete|Clone' -count=1`

Expected: PASS; source and destination directories remain independent under the failure and mutation cases.

- [ ] **Step 9: Commit the backend lifecycle operations**

```powershell
git add internal/storage/repository.go internal/storage/repository_test.go internal/app/profile_lifecycle.go internal/app/profile_lifecycle_test.go
git commit -m "feat: add profile clone and delete operations"
```

### Task 2: Expose profile lifecycle operations through the desktop API and controller

**Files:**
- Modify: `internal/ui/bridge.go`
- Modify: `internal/ui/bridge_test.go`
- Modify: `internal/ui/wails/desktop_app.go`
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/runner.ts`
- Modify: `frontend/src/state/useRunnerController.ts`
- Modify: `frontend/src/state/useRunnerController.test.tsx`

**Interfaces:**
- `ServicePort` consumes `DeleteProfile(context.Context, string) error` and `CloneProfile(context.Context, string) (profile.Profile, error)`.
- `Bridge` exposes `DeleteProfile(ctx, profileID) error` and `CloneProfile(ctx, profileID) (profile.Profile, error)`.
- `RunnerApi` exposes `deleteProfile(profileID): Promise<void>` and `cloneProfile(profileID): Promise<Profile>`.
- The controller exposes `deleteSelectedProfile(): Promise<boolean>` and `cloneSelectedProfile(): Promise<Profile | null>`.

- [ ] **Step 1: Add bridge tests and extend service test doubles**

Add bridge tests proving lifecycle calls forward the selected ID, result, and error unchanged. Update every `ServicePort` fake to implement the two new methods.

- [ ] **Step 2: Run bridge tests before implementation**

Run: `go test ./internal/ui -run 'Profile.*(Delete|Clone)|DeleteProfile|CloneProfile' -count=1`

Expected: FAIL with missing ServicePort methods or forwarding methods, not an unrelated test failure.

- [ ] **Step 3: Add controller tests for clone selection and delete fallback**

With two fake profiles, assert cloning calls `api.cloneProfile` with the selected ID, reloads the profile list, loads the returned clone by ID, and selects it. Assert deletion reloads profiles and selects the first remaining profile; when the API returns an empty list, assert selection becomes `null`. Assert API errors set the controller error and do not fake a successful selection change.

- [ ] **Step 4: Run controller lifecycle tests before implementation**

Run in `frontend`: `npm test -- --run src/state/useRunnerController.test.tsx`

Expected: FAIL because RunnerApi and controller lifecycle methods have not been added; failures identify missing API methods/actions.

- [ ] **Step 5: Implement backend and frontend API bindings**

Add `DesktopApp.DeleteProfile` and `DesktopApp.CloneProfile` methods that obtain the Wails context and delegate to `Bridge`. Add the two typed RunnerApi functions and Wails binding adapters. Update all typed RunnerApi test doubles with clone/delete functions. Implement controller actions using existing `listProfiles`, `getProfile`, `applySelectedProfile`, and error-state patterns; make `deleteSelectedProfile` return true only after deletion and profile-list refresh succeed, and false after an error.

- [ ] **Step 6: Run bridge tests, controller tests, and typecheck**

Run: `go test ./internal/ui ./internal/ui/wails -count=1`

Run in `frontend`: `npm test -- --run src/state/useRunnerController.test.tsx`

Run in `frontend`: `npx tsc -b`

Expected: PASS with the new methods present in all fake APIs.

- [ ] **Step 7: Commit desktop API and controller integration**

```powershell
git add internal/ui/bridge.go internal/ui/bridge_test.go internal/ui/wails/desktop_app.go frontend/src/api/types.ts frontend/src/api/runner.ts frontend/src/state/useRunnerController.ts frontend/src/state/useRunnerController.test.tsx
git commit -m "feat: expose profile lifecycle actions to desktop"
```

### Task 3: Add profile clone and delete actions to the main UI

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/components/ProfileDeleteDialog.tsx`
- Create: `frontend/src/components/ProfileDeleteDialog.test.tsx`
- Modify: `frontend/src/components/MainView.test.tsx`

**Interfaces:**
- `ProfileDeleteDialog` receives `open`, `profileName`, `onCancel`, and async `onConfirm` props.
- `App` uses controller lifecycle actions and closes dialogs only after a successful operation.

- [ ] **Step 1: Add UI tests for delete confirmation and clone action**

Assert the selected profile name appears in the delete confirmation, Cancel performs no delete call, and Confirm calls the controller/API. Assert Clone calls the selected profile API and the cloned name becomes selected. Assert lifecycle actions are disabled while a run is active or while no profile is selected.

- [ ] **Step 2: Run the focused UI tests before implementation**

Run in `frontend`: `npm test -- --run src/components/ProfileDeleteDialog.test.tsx src/components/MainView.test.tsx`

Expected: FAIL because the delete confirmation, profile actions, and controller integration do not exist yet.

- [ ] **Step 3: Implement accessible profile actions and confirmation**

Add **Clone** and **Delete** buttons beside profile actions. Use a confirmation dialog with `role="alertdialog"`, the exact profile name, and explicit Cancel/Delete buttons. Keep the dialog open on deletion error and surface the controller error in the existing error area. Invoke clone and select the returned profile; close confirmation and let the controller refresh selection after a successful delete.

- [ ] **Step 4: Run frontend lifecycle tests and typecheck**

Run in `frontend`: `npm test -- --run src/components/ProfileDeleteDialog.test.tsx src/components/MainView.test.tsx src/state/useRunnerController.test.tsx`

Run in `frontend`: `npx tsc -b`

Expected: PASS; cancel/error do not report success and deleting the last profile clears selection.

- [ ] **Step 5: Commit the profile UI**

```powershell
git add frontend/src/App.tsx frontend/src/components/ProfileDeleteDialog.tsx frontend/src/components/ProfileDeleteDialog.test.tsx frontend/src/components/MainView.test.tsx
git commit -m "feat: add profile clone and delete controls"
```
