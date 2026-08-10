# Go Script SQL Runner — Runtime Schema Selection

**Date:** 2026-08-10  
**Status:** Approved  
**Scope:** Desktop runtime schema selection, shared application/database layer, and CLI compatibility

## 1. Goal

Make the MySQL database/schema a runtime choice instead of persistent profile configuration.

A profile must be creatable and editable without a database/schema. After a successful server connection, the application loads the schemas visible to that MySQL user and requires the user to select one before executing SQL scripts.

The selected schema exists only for the current application session and must be selected again after restarting the application.

## 2. Superseded behavior

This specification supersedes the parts of `2026-08-07-sql-script-runner-design.md` that require or persist `connection.database` / database/schema in a profile or exported ZIP.

From this change onward:

- schema is not required to create or save a profile;
- schema is not persisted as the active execution target;
- schema is not exported as the active execution target in new profile archives;
- reopening the desktop application never automatically restores the previous runtime schema selection;
- changing profiles clears the runtime schema selection.

For backward compatibility, existing YAML/profile archives that contain a legacy database field may still be accepted during import/load, but the value must not become the active runtime schema automatically and must not be relied on for execution.

## 3. Profile configuration

Persistent connection configuration contains:

- profile name;
- host;
- port;
- username;
- password;
- execution defaults;
- configured SQL scripts.

Database/schema is not required profile configuration.

The profile editor must remove the current `Database / Schema` required field. Validation must require only name, host, valid port, and username for connection identity. Password remains optional according to existing behavior.

## 4. Connection and schema discovery

`Test Connection` connects to the MySQL server without selecting a database.

After a successful connection the backend must:

1. detect the MySQL server version/capabilities as it does today;
2. execute `SHOW DATABASES` on the same server credentials;
3. return the server capabilities plus the schema names visible to the connected user.

No system-schema filtering is applied. The UI displays the list returned by MySQL because server privileges already determine which databases are visible.

If schema discovery fails, the connection test is considered unsuccessful for runtime execution and the UI must show the returned error.

## 5. Desktop runtime state

The frontend controller owns runtime-only state:

- `availableSchemas: string[]`;
- `selectedSchema: string | null`;
- connection/capability status already maintained by the controller.

State rules:

- application startup: `selectedSchema = null`;
- profile change: clear capabilities, available schemas, and selected schema;
- profile edit/save affecting connection data: clear capabilities, available schemas, and selected schema;
- successful connection test: populate available schemas but do not auto-select a schema;
- failed connection test: clear available schemas and selected schema;
- application restart: no schema is restored.

## 6. Schema selector UI

After a profile is selected, the main connection toolbar exposes a `Schema` combobox/search selector.

Behavior:

- disabled until `Test Connection` succeeds and schemas are loaded;
- supports typing/filtering locally by schema name;
- filtering is case-insensitive;
- selecting an item sets only `selectedSchema` in frontend runtime state;
- the selected value is never written back into the profile;
- changing the selected schema does not require reconnecting just to update the UI state.

The implementation should use a small local React component instead of adding a third-party combobox dependency. The current frontend has only React/ReactDOM as runtime dependencies, and this feature does not justify introducing another package.

## 7. Execution flow

`Run` requires all of the following:

- selected profile;
- successful connection/schema discovery for that profile;
- non-empty `selectedSchema`;
- no execution already running.

The runtime schema is sent through execution options rather than mutating `Profile.Connection`.

Conceptually:

```text
Profile credentials
      ↓
Test Connection (no DB)
      ↓
SHOW DATABASES
      ↓
availableSchemas
      ↓
user selects selectedSchema
      ↓
RunOptions.Schema
      ↓
backend opens execution connection with selected DB
      ↓
executor runs configured SQL scripts
```

The database adapter should distinguish two operations:

- server connection/probe with no database selected;
- execution connection with an explicit runtime database/schema.

An empty runtime schema passed to execution must return a validation error before any script is executed.

## 8. CLI compatibility

Because schema is no longer persistent execution configuration, CLI execution cannot silently rely on the profile database value.

The CLI `run` flow must receive the target schema explicitly for each invocation, for example with `--schema <name>`. The schema option is required when executing a profile.

Connection testing remains database-independent. If the CLI exposes schema discovery output, it should use the same shared application service as the desktop UI rather than duplicating SQL/database logic.

Legacy profiles containing a database value must not cause CLI execution to bypass the explicit runtime schema requirement.

## 9. Backend/API boundaries

The shared application layer should expose schema discovery without leaking `database/sql` details to Wails or React.

Recommended contract shape:

```text
TestConnection(profileID) -> ConnectionResult
ConnectionResult:
  capabilities
  schemas[]

RunProfile(profileID, RunOptions{ Schema, OnError, TransactionMode })
```

The exact DTO names may follow existing project conventions, but schema discovery and runtime selection must remain separate from profile persistence.

## 10. Error handling

Expected errors include:

- server connection failure;
- authentication failure;
- `SHOW DATABASES` permission/server restriction failure;
- selected schema removed or permission revoked between discovery and execution;
- execution attempted without a runtime schema.

Errors must be surfaced through the existing controller/log error handling. The application must not fall back to an arbitrary/default schema when the selected schema becomes invalid.

## 11. Testing

Backend tests must cover:

- profile validation accepts an empty database/schema;
- server probe succeeds without selecting a database;
- schema discovery returns database names from `SHOW DATABASES`;
- execution rejects an empty runtime schema;
- execution uses the explicitly supplied runtime schema;
- legacy persisted database values do not automatically become runtime execution targets.

Frontend tests must cover:

- profile dialog saves without database/schema;
- successful connection populates schema choices;
- selector filtering is case-insensitive;
- schema is not auto-selected after connection;
- changing profile clears selected schema;
- failed connection clears selected schema;
- `Run` is disabled until a schema is selected;
- selected schema is passed to `runProfile` but not persisted through `saveProfile`;
- remount/restart state begins with no selected schema.

CLI tests must cover the explicit `--schema` execution requirement.

## 12. Acceptance criteria

The change is complete when:

1. a profile can be created without filling a database/schema;
2. connection testing works without a database selected;
3. successful connection testing loads schemas visible to the MySQL user;
4. the desktop UI provides a searchable schema selector;
5. no schema is auto-selected;
6. SQL cannot run until the user selects a schema;
7. execution uses exactly the schema selected for that run/session;
8. schema selection is cleared on profile change and application restart;
9. the runtime schema is not persisted as the execution target in profile storage or new exports;
10. CLI execution requires an explicit runtime schema;
11. existing tests plus the new backend/frontend/CLI tests pass.
