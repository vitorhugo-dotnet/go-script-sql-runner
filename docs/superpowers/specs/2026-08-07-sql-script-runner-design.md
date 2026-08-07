# Go Script SQL Runner — Design Specification

**Date:** 2026-08-07  
**Status:** Approved design, pending final user review  
**Target:** Windows 10/11

## 1. Goal

Build a modern SQL script runner for developers who need to prepare or repair local MySQL databases when database dumps or legacy setup tools are incomplete or unreliable.

The application must be distributed as a single Windows `.exe`, provide both a CLI and a small modern desktop GUI, and keep all real SQL scripts, database infrastructure details, profiles, and runtime configuration outside the Git repository.

The SQL files supplied during design are reference material only and must never be committed to the repository.

## 2. Primary requirements

- Go backend/core.
- Windows 10/11 only for v1.
- Wails v2.13 desktop application.
- React + TypeScript frontend.
- Tailwind CSS v4 for styling.
- One distributed `.exe` exposes both CLI and GUI behavior.
- CLI and GUI are backed by the same Go application services.
- MySQL 5.6, MySQL 5.7, and MySQL 8.x support.
- Detect server version automatically after connecting.
- SQL scripts are configured/imported through the GUI or CLI.
- Imported SQL scripts and YAML configuration are persisted in the current user's AppData directory.
- Support reusable script profiles.
- Profiles can be exported and imported as ZIP archives.
- Exported profiles include SQL files, YAML configuration, host, port, database/schema, username, and password.
- Exported ZIP files are intentionally not password protected or encrypted because profiles are intended for controlled internal use.
- Importing a profile with the same profile ID overwrites the local profile after an explicit warning/confirmation.
- Failure behavior is configurable: stop execution on failure or continue after failure.
- Support transaction-aware execution, including commit and rollback behavior.
- GUI shows a compact log panel.
- CLI is concise by default and supports detailed logging with `--verbose` / `-v`.
- GUI starts small but remains responsive and usable when resized.

## 3. Non-goals for v1

- PostgreSQL support.
- SQL Server support.
- Linux/macOS distribution.
- SQL template variables or `${variable}` substitution.
- Cloud profile synchronization.
- User accounts or authentication inside the runner.
- Profile ZIP encryption/password protection.
- Automatic modification of SQL to make it compatible with another MySQL version.

## 4. Technology choices

### Backend

- Go.
- `database/sql` for database access and transaction lifecycle.
- MySQL driver isolated behind the database adapter.
- `log/slog` for structured application logging.
- Standard-library `archive/zip` for profile import/export.
- Standard-library filesystem/path APIs for local storage.

### Desktop UI

- Wails v2.13.
- React.
- TypeScript.
- Vite-based frontend build.
- Tailwind CSS v4.

Wails embeds the built frontend assets into the Windows application and exposes Go services to the TypeScript frontend. The application core must not depend on React or Wails-specific UI behavior.

### Windows packaging

The distributed artifact is a single application `.exe`. Wails uses the Microsoft WebView2 runtime on Windows. To preserve the single-file distribution experience on Windows 10 machines where WebView2 may not already be installed, release builds must use Wails' embedded WebView2 bootstrapper strategy (`-webview2 embed`). If a suitable runtime is missing, the embedded Microsoft bootstrapper can install it without requiring the runner to ship a second companion file.

The actual WebView2 runtime remains a Windows runtime dependency; it is not statically linked into the Go executable.

## 5. Application architecture

```text
go-script-sql-runner/
├── cmd/
│   └── runner/                 # CLI/application entry point
├── internal/
│   ├── app/                    # use cases/orchestration
│   ├── database/
│   │   ├── connection.go
│   │   ├── detection.go
│   │   └── capabilities.go
│   ├── executor/
│   │   ├── runner.go
│   │   ├── transaction.go
│   │   └── result.go
│   ├── profile/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── import.go
│   │   └── export.go
│   ├── storage/
│   │   └── appdata.go
│   └── logging/
├── frontend/
│   └── ... React + TypeScript + Tailwind
└── docs/
```

### Boundary rules

- `database` owns connection creation and server capability detection.
- `executor` owns script execution and transaction/failure policies.
- `profile` owns profile metadata and import/export.
- `storage` owns AppData paths and filesystem persistence.
- `app` coordinates these units and is consumed by both CLI and GUI.
- Frontend code never connects directly to MySQL and never executes SQL itself.

## 6. AppData storage

Runtime data must live under the current Windows user's AppData directory.

Logical structure:

```text
%APPDATA%\GoScriptSQLRunner\
├── settings.yaml
├── profiles\
│   └── <profile-id>\
│       ├── profile.yaml
│       └── scripts\
│           ├── <script-id>.sql
│           └── ...
└── logs\
    └── ...
```

Rules:

- SQL files selected by the user are copied into AppData.
- The application must not depend on the source file remaining at its original path.
- Script IDs are stable internal IDs; display names are metadata.
- YAML and SQL runtime content must never be generated inside the repository working tree.
- Real infrastructure values are forbidden in tracked repository files.

## 7. Profile model

A profile represents a complete runnable database setup.

Example conceptual shape:

```yaml
id: local-dev-profile
name: Local Dev Profile
version: 1

connection:
  host: 127.0.0.1
  port: 3306
  database: example
  username: example
  password: example

execution:
  on_error: continue
  transaction_mode: auto_commit

scripts:
  - id: script-001
    name: Base structure
    file: scripts/script-001.sql
    enabled: true
    order: 10
```

Repository examples must use only fictitious values such as those above.

### Script ordering

- Profiles have an explicit deterministic script order.
- The UI must allow scripts to be reordered.
- Disabled scripts remain in the profile but are skipped during execution.

## 8. Profile import/export

### Export

The application exports a self-contained `.zip` containing everything another developer needs to import and execute the profile.

```text
profile.zip
├── manifest.yaml
├── profile.yaml
└── scripts/
    ├── <script-id>.sql
    └── ...
```

`profile.yaml` includes the database connection details, including the password.

By explicit requirement, the ZIP is not encrypted and does not require a password. The GUI/CLI should warn that the exported profile contains database credentials.

### Import

- Validate ZIP structure before modifying local state.
- Validate YAML before modifying local state.
- Reject unsupported profile format versions.
- Extract to a temporary directory first.
- Only move the profile into AppData after complete validation.
- Prevent ZIP path traversal (`../`, absolute paths, drive paths).
- If the profile ID already exists, warn that the existing profile will be replaced.
- After confirmation, replace the existing profile atomically as far as practical.
- Do not merge old and imported SQL files.

CLI import may use an explicit overwrite confirmation unless a non-interactive `--force` option is supplied.

## 9. MySQL detection and compatibility

The runner must not ask the user to choose a MySQL version.

After connection succeeds, it queries server version information and builds a capability model.

Required recognized families:

- MySQL 5.6
- MySQL 5.7
- MySQL 8.x
- MariaDB should be detected as a distinct vendor when encountered, but MariaDB-specific compatibility is not a guaranteed v1 target.

Conceptual model:

```go
type ServerCapabilities struct {
    Vendor     Vendor
    Major      int
    Minor      int
    Patch      int
    RawVersion string
}
```

Version checks must remain centralized. Application code must not accumulate scattered string comparisons such as `strings.HasPrefix(version, "8")`.

Because current Go MySQL driver support guarantees may not formally include MySQL 5.6, v1 acceptance requires integration testing against actual MySQL 5.6, 5.7, and 8.x instances rather than assuming compatibility from the driver API alone.

## 10. SQL execution

### Execution unit

Scripts execute sequentially according to the profile order.

Each script produces a result containing at minimum:

- script ID/name;
- start/end time;
- success/failure;
- duration;
- error information when applicable.

### Failure policy

Profile setting:

```yaml
execution:
  on_error: continue # continue | stop
```

- `continue`: record the failure and continue with the next script.
- `stop`: record the failure and abort the remaining script queue.

This policy can be overridden temporarily by CLI/UI execution controls without rewriting the stored profile.

### Statement failure inside one SQL file

A SQL file is treated as the current execution unit. If execution of that file fails, the file is marked failed and the profile's `on_error` policy determines whether the runner proceeds to the next file.

The runner must not silently suppress database errors.

## 11. Transaction modes

The runner supports three explicit modes:

### `auto_commit`

Statements execute using normal database autocommit behavior.

### `transaction`

The runner starts a Go `sql.Tx` for the script. On successful completion it calls `Commit`; on failure it attempts `Rollback`.

The UI/log output must warn that MySQL DDL statements can cause implicit commits, so rollback cannot be guaranteed for scripts containing DDL such as `CREATE`, `ALTER`, `DROP`, or `TRUNCATE`.

### `script_managed`

The SQL file is responsible for transaction commands such as `START TRANSACTION`, `COMMIT`, `ROLLBACK`, or autocommit configuration. The runner must not wrap the script in a second transaction.

The profile has a default transaction mode. Individual scripts may override the profile default when needed.

## 12. Logging

Use structured logging in the Go core.

### GUI

- Compact log panel attached to the bottom of the main window.
- Show timestamp, level, script/context, and message.
- Support at least `INFO`, `WARN`, and `ERROR` levels.
- Detailed mode can display database/statement diagnostics useful to developers.
- Log panel must not make the default window excessively large.

### CLI

Default output stays concise, for example:

```text
✓ Connected — MySQL 5.7.44
✓ Base structure
✗ Optional setup — table already exists
✓ Permissions

2 succeeded, 1 failed
```

`--verbose` / `-v` emits detailed execution logs.

Persistent logs are written under AppData.

Passwords must be redacted from logs, errors, and diagnostic dumps.

## 13. CLI design

The exact command library is an implementation choice, but v1 must expose these capabilities:

```text
runner                         # open GUI
runner profile list
runner profile import <file.zip>
runner profile export <profile-id> <file.zip>
runner profile show <profile-id>
runner script add <profile-id> <file.sql>
runner script remove <profile-id> <script-id>
runner connection test <profile-id>
runner run <profile-id>
runner run <profile-id> --verbose
runner run <profile-id> --stop-on-error
runner run <profile-id> --continue-on-error
```

CLI and GUI must call the same application services rather than duplicate execution logic.

## 14. GUI design

### Window

- Windows-native desktop window through Wails.
- Initial size approximately 680×540 px.
- Minimum approximately 560×420 px.
- Resizable.
- Responsive layout must make useful use of larger window sizes.
- Essential controls remain visible at the default size.

### Main view

The compact default layout should expose:

1. Profile selector.
2. Connection summary/status.
3. Detected MySQL version after test/connect.
4. Ordered script list with enabled state and result/status.
5. Run button.
6. Stop/continue failure setting.
7. Transaction mode.
8. Import/export profile actions.
9. Compact bottom log panel.

Profile/script editing may use dialogs, drawers, or secondary views so the main screen stays small.

### Visual direction

- Modern developer-tool appearance.
- High information density without looking like a legacy administration panel.
- Tailwind utility classes for layout/theme.
- Avoid unnecessary animation.
- Clear success/warning/error states.
- Keyboard-friendly controls where practical.

## 15. Git and repository safety

This is a hard requirement.

The repository must never contain:

- company SQL scripts;
- real database hosts/IPs;
- real usernames;
- real passwords;
- real database/schema names when they expose company infrastructure;
- exported internal profiles;
- AppData contents;
- runtime logs.

`.gitignore` must cover local runtime/profile artifacts and common accidental export locations.

Any sample profile committed for tests/docs must use fabricated data and fabricated SQL.

Tests must create temporary SQL/configuration fixtures dynamically or use obviously synthetic fixtures committed specifically for tests.

## 16. Error handling

Errors presented to users must distinguish at least:

- invalid profile/configuration;
- file/import errors;
- invalid/corrupt ZIP;
- unsupported profile format;
- connection/DNS/network failure;
- authentication failure;
- database/schema selection failure;
- SQL execution failure;
- commit failure;
- rollback failure;
- unsupported or unrecognized server version.

Detailed database errors belong in detailed logs, while the normal GUI/CLI output should remain readable.

## 17. Testing strategy

### Unit tests

- profile YAML serialization/validation;
- AppData path handling through injectable/test directories;
- profile overwrite/import behavior;
- ZIP traversal protection;
- version parsing/detection;
- script ordering;
- failure policy;
- log redaction;
- transaction orchestration using test doubles where appropriate.

### Integration tests

Run the same compatibility suite against:

- MySQL 5.6;
- MySQL 5.7;
- MySQL 8.x.

Validate at least:

- connection;
- version detection;
- multi-statement SQL execution;
- normal DML transaction commit;
- rollback after DML failure;
- DDL implicit-commit warning behavior;
- continue-on-error;
- stop-on-error.

Integration infrastructure is development/CI-only and must use generic containers/configuration. No company infrastructure may be committed.

### Frontend tests

Focus on critical state and flows rather than snapshot-heavy coverage:

- profile selection;
- script ordering/enabling;
- import overwrite warning;
- run state/progress;
- connection test/version display;
- compact/resized layout behavior.

## 18. Acceptance criteria

v1 is complete when a developer on Windows 10/11 can:

1. Run one distributed `.exe` without installing Java.
2. Launch the GUI by running the executable without CLI arguments.
3. Use CLI commands through that same executable.
4. On a Windows 10 machine missing WebView2, use the embedded bootstrapper flow rather than needing a second runner file.
5. Create a profile through GUI or CLI.
6. Configure complete MySQL connection information.
7. Import SQL files and have the runner copy them into AppData.
8. Reorder and enable/disable profile scripts.
9. Connect to MySQL 5.6, 5.7, or 8.x and see the detected version.
10. Execute the profile sequentially.
11. Choose whether failures stop or continue execution.
12. Use supported transaction modes and see commit/rollback outcomes.
13. See concise status and optionally detailed logs.
14. Export a complete profile ZIP including credentials and SQL files.
15. Import that ZIP on another machine/user account.
16. Receive an overwrite warning when importing an existing profile ID.
17. Successfully overwrite the profile after confirmation.
18. Resize the GUI while keeping the layout usable.
19. Verify that no real infrastructure data or supplied company SQL exists in Git history introduced by this project.

## 19. Key design decisions

- Windows-only v1 keeps packaging and UI behavior focused.
- Wails v2.13 is preferred over Wails v3 while v3 remains outside the stable v2 line used for this project.
- Release builds use Wails' embedded WebView2 bootstrapper strategy to preserve a single distributed `.exe` experience.
- React/TypeScript/Tailwind provides a modern GUI without moving database logic out of Go.
- AppData is the single source of truth for local runtime profiles and SQL files.
- CLI and GUI share one core to prevent behavioral drift.
- MySQL version is detected, not manually configured.
- Actual MySQL 5.6 integration tests are mandatory because driver maintainers may no longer formally guarantee that server version.
- Exported profile ZIPs intentionally contain plaintext connection credentials by explicit internal-use requirement; the product warns users instead of encrypting the archive.
- No SQL templating is included in v1.
