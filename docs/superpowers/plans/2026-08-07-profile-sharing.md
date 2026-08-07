# Profile Sharing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add self-contained ZIP profile export/import containing YAML, SQL files, and database credentials, with safe validation and explicit overwrite behavior.

**Architecture:** Export reads only the canonical AppData profile directory and writes a deterministic archive containing `manifest.yaml`, `profile.yaml`, and `scripts/`. Import is two-phase: inspect/validate into a staging directory, then commit into AppData. Existing profile IDs are never merged; confirmed imports replace the complete directory.

**Tech Stack:** Go 1.26, standard-library `archive/zip`, `io`, `os`, `path`, `filepath`, existing profile/storage/app packages, Cobra CLI.

## Global Constraints

- Exported archives intentionally contain plaintext host, port, database/schema, username, password, YAML configuration, and SQL files.
- Exported archives have no ZIP password or application-level encryption by explicit requirement.
- CLI and GUI must warn that archives contain database credentials.
- Same profile ID means overwrite after warning/confirmation; never merge scripts.
- Import must validate the complete archive before touching the existing profile.
- Import must reject ZIP path traversal, absolute/drive paths, backslash-based Windows traversal, symlinks, invalid YAML, missing referenced SQL files, unexpected profile format versions, and duplicate required entries.
- No internal/company archive or real credentials may be committed to Git or used in tests.

## File Map

```text
internal/profile/archive.go              # manifest/archive domain and inspection validation
internal/profile/archive_test.go         # archive structure/security tests
internal/profile/export.go               # ZIP writer
internal/profile/export_test.go          # deterministic self-contained export tests
internal/profile/import.go               # staging extraction and validated import
internal/profile/import_test.go          # traversal/corruption/round-trip tests
internal/storage/replace.go              # safe directory replacement with restoration
internal/storage/replace_test.go         # replacement failure/overwrite tests
internal/app/sharing.go                   # application-level inspect/export/import use cases
internal/app/sharing_test.go              # service behavior and conflict contract
internal/cli/profile_sharing.go           # profile import/export commands and confirmation
internal/cli/profile_sharing_test.go      # CLI warnings/--force tests
```

---

### Task 1: Archive manifest and strict archive inspection

**Files:**
- Create: `internal/profile/archive.go`
- Create: `internal/profile/archive_test.go`

**Interfaces:**
- Produces `profile.ArchiveManifest`.
- Produces `profile.ArchiveInspection`.
- Produces `profile.InspectArchive(path string) (ArchiveInspection, error)`.

- [ ] **Step 1: Write failing manifest/structure tests**

Use this exact manifest schema:

```go
type ArchiveManifest struct {
    FormatVersion int    `yaml:"format_version"`
    ProfileID     string `yaml:"profile_id"`
    ProfileName   string `yaml:"profile_name"`
}
```

Valid archive layout:

```text
manifest.yaml
profile.yaml
scripts/<script-id>.sql
```

Tests must reject archives with missing `manifest.yaml`, missing `profile.yaml`, duplicate `manifest.yaml`, manifest `format_version != 1`, or manifest/profile ID mismatch.

Run:

```powershell
go test ./internal/profile
```

Expected: FAIL because archive inspection does not exist.

- [ ] **Step 2: Add malicious ZIP path tests**

Create archives in tests entirely in `t.TempDir()`. Require rejection for each entry name:

```text
../outside.sql
scripts/../../outside.sql
/absolute.sql
C:/outside.sql
scripts\..\outside.sql
```

Also create an archive entry whose mode is `os.ModeSymlink` and require rejection.

- [ ] **Step 3: Implement archive inspection without extraction**

Validation rules:

```go
func safeZipName(name string) bool {
    if name == "" || strings.Contains(name, "\\") || strings.Contains(name, ":") {
        return false
    }
    if path.IsAbs(name) {
        return false
    }
    clean := path.Clean(name)
    return clean != ".." && !strings.HasPrefix(clean, "../")
}
```

Use `archive/zip.OpenReader`, inspect `zip.File` metadata, reject symlinks, decode manifest/profile with the existing strict YAML decoder, validate `profile.Profile`, verify every enabled/disabled script path equals `scripts/<script-id>.sql`, and verify each referenced script exists exactly once.

`ArchiveInspection`:

```go
type ArchiveInspection struct {
    Manifest ArchiveManifest
    Profile  Profile
}
```

- [ ] **Step 4: Run archive validation tests**

```powershell
go test ./internal/profile -run Archive -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/profile/archive.go internal/profile/archive_test.go
git commit -m "feat: validate profile archives"
```

---

### Task 2: Self-contained profile ZIP export

**Files:**
- Create: `internal/profile/export.go`
- Create: `internal/profile/export_test.go`

**Interfaces:**
- Consumes canonical AppData profile directory.
- Produces `profile.ExportArchive(profileDir, destination string) error`.

- [ ] **Step 1: Write failing self-contained export test**

Create a synthetic AppData profile:

```text
profiles/<id>/profile.yaml
profiles/<id>/scripts/<script-a>.sql
profiles/<id>/scripts/<script-b>.sql
```

Use a password value such as `synthetic-password-123`. Export and open the ZIP with `archive/zip`; assert:

- `manifest.yaml`, `profile.yaml`, and both scripts exist;
- decoded `profile.yaml` still contains the synthetic host/user/password/database;
- SQL content matches source bytes;
- no absolute AppData/source paths are stored in entry names or YAML.

Run:

```powershell
go test ./internal/profile -run Export -v
```

Expected: FAIL.

- [ ] **Step 2: Implement deterministic export**

`ExportArchive` must:

1. load and validate canonical `profile.yaml`;
2. verify every referenced script exists;
3. create destination parent directory when necessary;
4. write to `<destination>.tmp` first;
5. write entries in deterministic order: `manifest.yaml`, `profile.yaml`, scripts sorted by profile `Order` then ID;
6. close the ZIP writer and file;
7. rename temp file to final destination;
8. remove temp file on any error.

Use standard `archive/zip`; do not introduce encryption libraries.

- [ ] **Step 3: Add destination replacement behavior test**

If a destination ZIP already exists, export must replace it only after a complete new temp archive has been successfully written. A failed write must leave the previous destination intact.

- [ ] **Step 4: Verify export tests**

```powershell
go test ./internal/profile -run Export -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/profile/export.go internal/profile/export_test.go
git commit -m "feat: export complete runner profiles"
```

---

### Task 3: Staged import and complete directory replacement

**Files:**
- Create: `internal/storage/replace.go`
- Create: `internal/storage/replace_test.go`
- Create: `internal/profile/import.go`
- Create: `internal/profile/import_test.go`

**Interfaces:**
- Produces `storage.ReplaceDir(staged, target string) error`.
- Produces `profile.StageArchive(archivePath, stagingRoot string) (stagedProfileDir string, inspection ArchiveInspection, err error)`.

- [ ] **Step 1: Write failing storage replacement tests**

Test this sequence using temporary directories:

```text
target has old profile -> staged has new profile -> ReplaceDir -> target contains only new profile
```

Also inject a failure seam around rename and assert the old target is restored if moving staged content into place fails after the target has been backed up.

Define a private rename function variable in `replace.go`:

```go
var rename = os.Rename
```

Tests may temporarily replace it and restore it with `t.Cleanup`.

- [ ] **Step 2: Implement safe same-volume replacement**

Algorithm:

```text
if target absent: rename staged -> target
if target exists:
  rename target -> sibling backup
  rename staged -> target
  if second rename fails: rename backup -> target and return joined error
  remove backup
```

Use a random backup suffix generated with `internal/id.New()`.

- [ ] **Step 3: Write failing staged extraction tests**

`StageArchive` must call `InspectArchive` first, create a unique staging directory under the supplied `stagingRoot`, then extract only inspected valid entries. On error it removes the staging directory.

Assert no file is created outside staging for every malicious path test from Task 1.

- [ ] **Step 4: Implement extraction with exact destination containment check**

For each accepted entry:

```go
dst := filepath.Join(stagedProfileDir, filepath.FromSlash(name))
rel, err := filepath.Rel(stagedProfileDir, dst)
if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
    return "", inspection, ErrUnsafeArchivePath
}
```

Create directories with `0755` and files with `0600` permissions. The Windows ACL remains governed by the current user's AppData directory; no cross-user sharing mechanism is added.

- [ ] **Step 5: Add full export -> stage -> validate round-trip test**

Create a synthetic profile with two SQL files and credentials, export it, stage it in a second temp root, then decode the staged `profile.yaml` and compare all domain fields and SQL bytes.

- [ ] **Step 6: Run tests**

```powershell
go test ./internal/profile ./internal/storage
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/profile/import.go internal/profile/import_test.go internal/storage/replace.go internal/storage/replace_test.go
git commit -m "feat: import and replace runner profiles"
```

---

### Task 4: Application import/export contract and overwrite conflict

**Files:**
- Create: `internal/app/sharing.go`
- Create: `internal/app/sharing_test.go`

**Interfaces:**
- Extends `app.Service` with inspect/export/import operations used identically by CLI and GUI.

- [ ] **Step 1: Write failing application service tests**

Add exact methods:

```go
func (s *Service) InspectProfileArchive(ctx context.Context, archivePath string) (profile.ArchiveInspection, error)
func (s *Service) ExportProfile(ctx context.Context, profileID, destination string) error
func (s *Service) ImportProfile(ctx context.Context, archivePath string, overwrite bool) (profile.Profile, error)
```

Define typed conflict:

```go
type ProfileConflictError struct {
    Existing profile.Profile
    Incoming profile.Profile
}
```

`ImportProfile(..., false)` returns `*ProfileConflictError` when incoming ID exists. It must not alter current files.

- [ ] **Step 2: Implement two-phase import in the service**

Flow:

```text
InspectArchive
-> check existing ID
-> if conflict && !overwrite: return ProfileConflictError
-> StageArchive under <AppData>/.imports
-> storage.ReplaceDir(staged, profiles/<id>)
-> remove empty staging parent when practical
-> repository.Get(id)
```

Never merge script directories.

- [ ] **Step 3: Ensure export surfaces credential warning metadata without logging the password**

The service itself returns no password warning string. Warning copy belongs to CLI/GUI presentation. Add a test proving service errors/events never interpolate the full profile struct or password.

- [ ] **Step 4: Run tests**

```powershell
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/sharing.go internal/app/sharing_test.go
git commit -m "feat: add profile sharing service"
```

---

### Task 5: CLI profile import/export with warning and overwrite confirmation

**Files:**
- Create: `internal/cli/profile_sharing.go`
- Create: `internal/cli/profile_sharing_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Adds `runner profile export <profile-id> <file.zip>`.
- Adds `runner profile import <file.zip> [--force]`.

- [ ] **Step 1: Write failing CLI tests**

Test export output includes this warning before success:

```text
WARNING: exported profile contains database credentials in plaintext.
```

Test import conflict without `--force` using an injected input reader containing `n\n` and `y\n`.

Prompt text:

```text
Profile "<name>" already exists and will be completely overwritten. Continue? [y/N]:
```

Test `--force` performs overwrite without reading stdin.

- [ ] **Step 2: Make CLI dependencies accept stdin**

Extend CLI wiring to carry `io.Reader` in addition to stdout/stderr so confirmation is testable and not tied directly to `os.Stdin`.

Public entry becomes:

```go
func Execute(ctx context.Context, service *app.Service, args []string, stdin io.Reader, stdout, stderr io.Writer) int
```

Update existing CLI tests/callers from the previous plan.

- [ ] **Step 3: Implement export command**

Before calling `ExportProfile`, print the plaintext-credentials warning to stderr. After success print the destination path. Never print host/user/password values themselves.

- [ ] **Step 4: Implement import command and confirmation**

Call `InspectProfileArchive` first so the CLI can show incoming profile name/ID. Call `ImportProfile(..., false)`; on `ProfileConflictError`, either:

- with `--force`: call `ImportProfile(..., true)` immediately;
- without `--force`: prompt once, accept only case-insensitive `y` or `yes`, otherwise exit without modifying files.

- [ ] **Step 5: Run CLI/profile round-trip tests**

```powershell
go test ./internal/cli ./internal/app ./internal/profile ./internal/storage
```

Expected: PASS.

- [ ] **Step 6: Manual synthetic smoke test**

Using only a fabricated local profile:

```powershell
go run . profile export <synthetic-profile-id> .\synthetic.runner.zip
go run . profile import .\synthetic.runner.zip --force
Remove-Item .\synthetic.runner.zip
```

Expected: warning appears, profile survives round trip, and the temporary ZIP is deleted afterward rather than committed.

- [ ] **Step 7: Commit**

```powershell
git add internal/cli internal/app internal/profile internal/storage
git commit -m "feat: share runner profiles as zip"
```
