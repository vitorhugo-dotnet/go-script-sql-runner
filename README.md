# Go Script SQL Runner

Desktop runner for executing ordered SQL scripts against MySQL/MariaDB with reusable local profiles.

## What it does

- Runs as a compact Windows desktop app when started without arguments.
- Uses the same executable for CLI commands when arguments are supplied.
- Stores reusable profiles with connection settings, execution policy and ordered SQL scripts.
- Supports per-run failure and transaction overrides without changing the saved profile.
- Supports per-script enable/disable, ordering and transaction overrides.
- Tests database connectivity and reports the detected server version.
- Streams execution events into the desktop log panel and persists execution logs locally.
- Imports and exports complete profiles as ZIP archives, including the SQL files and database connection settings.

## Security note

Profile exports intentionally contain the configured database host, username and password. The ZIP is **not encrypted**. The application displays a warning before export and should only be used through the intended internal sharing channel.

Passwords are not displayed in the connection summary or execution logs.

## Desktop usage

Start the executable without arguments:

```powershell
.\go-script-sql-runner.exe
```

Create a profile from **New**, configure the connection, add one or more SQL scripts, test the connection and run them in the displayed order.

The main execution controls allow temporary overrides for:

- **On failure:** `Continue` or `Stop`
- **Transaction:** `Auto commit`, `Transaction` or `Script managed`

A script-level transaction setting overrides the run/profile default for that specific script.

## CLI usage

The same executable routes commands with arguments to the CLI:

```powershell
.\go-script-sql-runner.exe profile list
.\go-script-sql-runner.exe profile show <profile-id>
.\go-script-sql-runner.exe run <profile-id>
```

Use the built-in help for the complete command list:

```powershell
.\go-script-sql-runner.exe --help
```

## Local development

Requirements:

- Go 1.26+
- Node.js 24+
- Wails v2.13
- Windows for the final desktop executable

Frontend development:

```powershell
cd frontend
npm install
npm test
npm run build
```

Go tests:

```powershell
go test ./...
```

## Build the Windows executable

Use the reproducible build script:

```powershell
.\scripts\build.ps1
```

The resulting executable is written to:

```text
build/bin/go-script-sql-runner.exe
```

The release build embeds the WebView2 bootstrapper and keeps the console subsystem so the same binary can service CLI usage. A standalone console opened only for GUI launch is hidden by the application.

## Synthetic profile example

Profiles are application-managed. A conceptual example looks like this:

```yaml
id: example-profile
name: Example
version: 1
connection:
  host: db.internal.example
  port: 3306
  database: sample_db
  username: runner_user
  password: replace-me
execution:
  on_error: continue
  transaction_mode: auto_commit
```

Do not commit real profile exports, SQL production data or credentials to the repository.
