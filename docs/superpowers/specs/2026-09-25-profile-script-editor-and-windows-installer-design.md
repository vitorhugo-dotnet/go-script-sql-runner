# Profile Management, SQL Script Editing, and Windows Distribution Design

**Date:** 2026-09-25  
**Status:** Design approved in conversation; awaiting written specification review.

## Goal

Extend the existing Wails desktop app so users can edit stored SQL scripts, delete and clone profiles, and choose between a portable executable and a guided Windows installer. The GitHub Release produced by CI will contain both Windows distribution options.

## Current context

- The React UI already lists, enables, removes, and reorders scripts, and edits profile connection and execution settings.
- SQL files live under each profile directory. React calls Go application services through Wails bindings; it does not access profile files directly.
- The storage repository already supports deleting a profile directory, but the service/backend/UI do not expose profile deletion.
- The service supports updating a profile while retaining its ID and script references. It does not yet clone profiles or edit stored SQL content.
- CI currently builds and publishes a single Wails executable named `go-script-sql-runner.exe`.
- The update checker currently searches GitHub Release assets for that fixed executable name.

## Approved design

### Script editing with Monaco

Add an **Edit** action for every script beside the existing ordering controls. It opens a larger in-app dialog containing Monaco Editor configured for SQL syntax highlighting. The editor loads content through the Wails API and saves it through the Go application service to the existing profile-owned SQL file. The frontend will not write files directly.

Saving changes only the script content; its profile, ID, name, order, enabled state, transaction mode, and file reference remain unchanged. Closing a modified editor without saving asks the user to confirm discarding the changes. Editing is unavailable while a profile run is active.

### Profile deletion

Add a **Delete profile** action for the selected profile. Before deletion, show a confirmation that identifies the profile by name and explains that its stored scripts are removed with it. The service deletes the profile through the repository. On success, refresh the profile list and select another profile if one remains; on failure, preserve the selection and show the error.

### Profile cloning

Add a **Clone profile** action. Create a separate profile with a new generated ID and a derived copy name. Copy connection settings including the password, execution settings, all script metadata, and the SQL file contents into the new profile's directory. Editing or deleting the clone must not change the source profile. The cloned profile can be renamed later using the existing profile editing dialog.

Cloning and deletion run through the Go application service and storage repository. The frontend does not copy or remove filesystem paths.

### Portable executable and installer

Keep a standalone, non-installing executable and name its release asset `go-script-sql-runner-portable.exe`. It continues to store profiles and logs in the current per-user AppData location.

Add a WiX-authored MSI named `go-script-sql-runner-setup.msi` with a guided installation flow and a scope choice:

- **Current user:** install under `%LOCALAPPDATA%\Programs\Go Script SQL Runner` without administrator rights.
- **All users:** install under `%ProgramFiles%\Go Script SQL Runner`; Windows Installer requests elevation for this machine-wide installation.

Regardless of installation scope, application data remains per-user in AppData. The installer supplies normal application shortcuts and an uninstall registration, and uses stable package identity/versioning so later releases can upgrade an earlier MSI installation.

CI builds and verifies both artifacts and publishes both assets on the existing automated GitHub Release. The update checker switches its fixed asset lookup to `go-script-sql-runner-portable.exe`; opening an update continues to download the portable build.

### Alternatives considered

- **Editing through the system's default SQL editor:** avoids embedding an editor, but requires launching an external application and complicates the save/reload path. Monaco was selected for an in-app editing flow.
- **Separate per-user and per-machine MSI files:** duplicates installer maintenance and makes release choice less clear. A single dual-purpose MSI with a scope choice was selected.
- **Single executable only:** keeps the current release simple but does not provide the requested guided installation or machine-wide option.

## Components and interfaces

The implementation will follow existing boundaries:

- `internal/storage`: safe read/write access to a script file, profile deletion, and cloning/copying profile-owned files.
- `internal/app`: context-aware use cases for script content read/write, profile deletion, and profile cloning, with validation and clear errors.
- `internal/ui/wails`: bind the new service operations to Wails; do not expose raw paths for frontend file access.
- `frontend/src/api`: extend the `RunnerApi` and Wails adapter with the new calls.
- `frontend/src/components`: Monaco SQL editor dialog and profile deletion confirmation.
- `frontend/src/App.tsx` and `frontend/src/state/useRunnerController.ts`: add the script and profile actions and refresh/selection behavior.
- `.github/workflows/ci.yml` and a WiX source/build script under `build/windows` or `scripts`: build, verify, upload, and publish the portable EXE and MSI.
- `internal/updatecheck`: search for the portable release asset's new filename.
- `README.md`: explain the portable executable and both MSI installation scopes.

Exact file names and helper boundaries may follow nearby repository conventions while preserving the behavior above.

## Error handling and data safety

- Validate profile and script IDs before resolving paths; resolve SQL paths only from validated profile metadata and enforce that resolved paths remain inside the profile directory.
- Use atomic replacement for SQL edits so an interrupted write does not leave a truncated script.
- Cloning should fail without modifying the source; avoid leaving an incomplete destination profile if copying or persistence fails.
- A failed deletion must be visible to the user. Confirmation precedes recursive profile removal.
- Treat installer and portable artifacts as equivalent app versions. Data remains in per-user AppData and is not moved or deleted by install/uninstall.
- Continue embedding WebView2 according to the current Wails build configuration.

## Verification and acceptance criteria

### Backend

- Unit tests cover script content reads/writes, unknown profile/script IDs, validation, preserving all script metadata during content updates, and ensuring writes stay within profile storage.
- Service/repository tests cover clone independence (new ID, retained settings/password, identical SQL bytes, independent subsequent edits/deletes), deletion of all files for a profile, and no effect on other profiles.
- Wails-facing tests or the existing bridge tests verify calls and error propagation.

### Frontend

- Component tests cover opening the editor with the selected SQL, saving content, cancelling unchanged content, confirming a dirty close, and disabled state while running.
- UI tests cover profile deletion confirmation/cancel/error/success and cloning plus profile selection/refresh.
- Frontend typecheck/build confirms Monaco and worker assets bundle correctly for Wails' local WebView.

### Windows distribution

- CI verifies both expected output files exist before upload and both named assets are published.
- Validate the MSI's wizard has a current-user and all-users choice; verify the former installs to LocalAppData without elevation and the latter targets Program Files and requests elevation.
- Verify MSI upgrade/uninstall behavior and that application profiles remain in per-user AppData.
- Update-checker tests select the new portable filename and continue to work when an expected asset is absent.
- Run the Go and frontend test suites and typecheck through CI.

## Out of scope

- SQL semantic validation, database-aware completion, formatting, or schema introspection in Monaco.
- Editing script names, order, or execution properties in the SQL editor dialog.
- Moving profile data into the all-users installation directory or sharing profiles between Windows accounts.
- Automatic in-place application of updates; the update notice continues to open the portable asset/release page behavior already supported by the app.
