# Go Script SQL Runner

## Purpose

Go Script SQL Runner is a compact Windows desktop application for executing ordered SQL scripts against MySQL-compatible databases with reusable local profiles. The same executable also exposes the CLI when started with arguments.

## Requirements

For end users:

- Windows 10 or Windows 11
- Network access to the target MySQL/MariaDB server

For development/builds:

- Go 1.26+
- Node.js 24+
- Wails v2.13.0

## Build

Use the reproducible build script:

```powershell
.\scripts\build.ps1
```

It runs Go tests, installs the locked frontend dependencies with `npm ci`, runs frontend tests/build, and builds the Wails executable with the embedded WebView2 bootstrapper.

Output:

```text
build/bin/go-script-sql-runner.exe
```

## AppData location

Application profiles and logs are stored under the current Windows user's configuration directory:

```text
%AppData%\GoScriptSQLRunner\
├── profiles\
└── logs\
```

Each profile owns its copied SQL scripts under its profile directory. The frontend does not write these files directly; storage is managed by the Go application layer.

## GUI usage

Start the executable without arguments:

```powershell
.\go-script-sql-runner.exe
```

The default desktop window exposes the essential workflow without navigating to another screen:

1. Create or select a profile.
2. Configure host, port, database/schema, username and password.
3. Add one or more `.sql` files. Multiple files can be selected in the native picker.
4. Reorder, enable/disable or set a transaction override per script when needed.
5. Test the connection and review the detected server version.
6. Choose the run-only failure and transaction policies.
7. Run the scripts and follow execution events in the bottom log panel.

Run-level overrides do not silently modify the saved profile. A script-level transaction override takes precedence over the run/profile default for that script.

## CLI usage

Supplying arguments routes the same executable to the CLI:

```powershell
.\go-script-sql-runner.exe profile list
.\go-script-sql-runner.exe profile show example
.\go-script-sql-runner.exe run example
```

For the complete command list:

```powershell
.\go-script-sql-runner.exe --help
```

## Profile import/export warning

Profile ZIP exports intentionally contain the configured database host, username and password together with the profile's SQL scripts. The ZIP is **not encrypted**.

The application displays a warning before export. Importing a profile whose ID already exists also requires explicit confirmation before the existing configuration and SQL files are completely replaced.

Passwords are not shown in the connection summary or execution logs.

## MySQL compatibility

The repository includes integration coverage against MySQL 5.6, MySQL 5.7 and MySQL 8.0. The connection test reports the detected server family/version to the desktop UI.

MariaDB connections are supported by the database layer, with server capabilities detected at runtime.

## Transaction and DDL caveat

Available transaction modes are:

- `auto_commit`: execute using normal database autocommit behavior.
- `transaction`: the runner starts and finishes the transaction.
- `script_managed`: transaction statements are controlled by the SQL script itself.

Some MySQL DDL statements cause implicit commits. When runner-managed transaction mode is used, such statements can make a complete rollback impossible. The runner analyzes scripts and emits a warning when this condition is detected.

## Synthetic profile example

Profiles are application-managed. This example is intentionally synthetic:

```yaml
id: example
name: example
version: 1
connection:
  host: 127.0.0.1
  port: 3306
  database: example
  username: dev
  password: dev
execution:
  on_error: continue
  transaction_mode: auto_commit
scripts:
  - id: example
    name: example.sql
    file: scripts/example.sql
    enabled: true
    order: 10
```

## Repository security rules

- Never commit real database credentials, internal hosts, production schemas or exported profile ZIPs.
- Production/local SQL files are runtime data and are ignored; only explicitly whitelisted synthetic test fixtures belong in Git.
- Logs and local AppData/profile directories are ignored.
- Keep examples synthetic using `127.0.0.1`, `example`, `dev` and disposable test data only.

## Local development

Frontend:

```powershell
cd frontend
npm ci
npm test
npm run build
```

Go:

```powershell
go test ./...
```
