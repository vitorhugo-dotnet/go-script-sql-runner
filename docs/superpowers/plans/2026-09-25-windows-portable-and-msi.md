# Windows Portable and MSI Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use Markdown task checkboxes.

**Goal:** Publish a portable Windows executable and a dual-scope guided MSI installer from the existing CI release workflow.

**Architecture:** Keep Wails responsible for the application executable and add a WiX-authored MSI that installs that executable with a current-user or all-users scope choice. CI verifies and uploads both artifacts; the update checker points to the renamed portable asset.

**Tech Stack:** Go, Wails v2, PowerShell, WiX Toolset, GitHub Actions, GitHub CLI.

**Spec:** [2026-09-25-profile-script-editor-and-windows-installer-design.md](../specs/2026-09-25-profile-script-editor-and-windows-installer-design.md)

## Global Constraints

- Name the portable release asset go-script-sql-runner-portable.exe.
- Name the installer release asset go-script-sql-runner-setup.msi.
- Current-user installation goes under %LOCALAPPDATA%\Programs\Go Script SQL Runner without administrator rights.
- All-users installation goes under %ProgramFiles%\Go Script SQL Runner and requests elevation.
- Regardless of installation scope, application data remains per-user in AppData.
- CI builds and verifies both artifacts and publishes both assets on the existing automated GitHub Release.
- The update checker searches for go-script-sql-runner-portable.exe.

## Review Focus

- A typo or missing CI artifact must fail the job before attempting release publication; pin with workflow checks for both exact file names.
- Per-user MSI installation must never write files or registry entries into machine locations; verify with a Windows install/uninstall smoke test and inspect authored directory/registry scopes.
- Per-machine MSI installation must target Program Files and use Windows Installer elevation behavior; verify the selected MSI properties and manual UAC install.
- Upgrade must replace the installed app while preserving user profiles; validate stable UpgradeCode/major upgrade behavior and inspect the user AppData directory after upgrade.
- Missing portable Release asset must not break update checking or return an installer URL as the portable download; pin in update-checker tests.

---

### Task 1: Author and locally build the dual-scope MSI

**Files:**
- Create: build/windows/installer/Package.wxs
- Create: build/windows/installer/ScopeSelectionDlg.wxs
- Create: scripts/build-windows-installer.ps1
- Modify: scripts/build.ps1

**Interfaces:**
- The installer consumes build/bin/go-script-sql-runner-portable.exe.
- The installer build produces build/bin/go-script-sql-runner-setup.msi.
- The MSI exposes a wizard choice for current user versus all users.

- [ ] **Step 1: Add a WiX source validation/build script**

Create a PowerShell build helper that accepts an application version and executable path, fails if the EXE is missing, invokes the pinned WiX Toolset CLI version 7.0.0 and UI extension version 7.0.0, and writes the MSI to the specified output directory. Keep the WiX and extension versions fixed in the repository and CI setup so the same source uses the same compiler.

- [ ] **Step 2: Author MSI package identity and file layout**

Define the product name, manufacturer, fixed UpgradeCode, package version, x64 architecture, and major-upgrade rule. Install the one portable executable under a scope-dependent install directory. Add a Start Menu shortcut and Add/Remove Programs metadata. Do not author profile/log directories under Program Files.

- [ ] **Step 3: Add dual-scope selection and wizard navigation**

Author the package as perUserOrMachine with ALLUSERS=2 and MSIINSTALLPERUSER=1 as the default. Add a wizard dialog with a radio choice that keeps MSIINSTALLPERUSER=1 for current user and clears MSIINSTALLPERUSER for all users. Current-user install directory is LocalAppDataFolder\Programs\Go Script SQL Runner; all-users install directory is ProgramFiles64Folder\Go Script SQL Runner. Confirm the selection occurs before the execute sequence so Windows Installer can request elevation when the user chooses all users.

- [ ] **Step 4: Add MSI source-level checks**

Add a PowerShell validation step to the helper that runs WiX compilation/linking and checks the output file exists and is non-empty. Include a test invocation using version 0.1.123; inspect the built MSI properties with Orca and assert package scope supports either context and UpgradeCode is stable.

- [ ] **Step 5: Update the local build script to emit both artifacts**

Change scripts/build.ps1 to build Wails output as go-script-sql-runner-portable.exe, verify it exists, then invoke the WiX build helper with the same output directory and app version. Keep existing frontend and Go build steps intact.

- [ ] **Step 6: Build both artifacts locally**

Run: .\\scripts\\build.ps1

Expected: both build/bin/go-script-sql-runner-portable.exe and build/bin/go-script-sql-runner-setup.msi exist. The MSI wizard offers both scopes and the current-user selection completes without elevation on a standard account.

- [ ] **Step 7: Commit installer authoring and local build integration**

```powershell
git add build/windows/installer scripts/build.ps1
git commit -m "build: add dual-scope Windows MSI installer"
```

### Task 2: Publish both artifacts and update the portable download lookup

**Files:**
- Modify: .github/workflows/ci.yml
- Modify: internal/updatecheck/checker.go
- Modify: internal/updatecheck/checker_test.go

**Interfaces:**
- Release assets are go-script-sql-runner-portable.exe and go-script-sql-runner-setup.msi.
- The checker returns DownloadURL for the portable EXE only.

- [ ] **Step 1: Update checker tests for the renamed asset**

Change fixture releases to include both portable and MSI assets. Assert the portable asset URL is selected, MSI URL is never selected, and absent portable asset leaves DownloadURL empty while the rest of release metadata is returned.

- [ ] **Step 2: Change the checker asset constant**

Set the expected asset name to go-script-sql-runner-portable.exe and keep the exact name match behavior. Do not silently fall back to the MSI asset.

- [ ] **Step 3: Run focused checker tests**

Run: go test ./internal/updatecheck -count=1

Expected: PASS for portable selection, MSI exclusion, and missing asset behavior.

- [ ] **Step 4: Update CI build, artifact upload, and release steps**

Install the pinned WiX CLI 7.0.0 and WixToolset.UI.wixext 7.0.0 in the Windows build job. Build the portable EXE with Wails and the MSI with the repository helper using MSI version 0.1.<github.run_number>. Verify both exact filenames, upload both in the same named artifact, download them in the release job, verify both again, and pass both paths to gh release upload/create. Set user-facing asset labels to “Windows portable executable” and “Windows installer (MSI)”.

- [ ] **Step 5: Validate workflow syntax and release asset references**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml`

Search the workflow for stale go-script-sql-runner.exe references and confirm none remain in upload, verify, and publish commands.

- [ ] **Step 6: Commit CI and update-check integration**

```powershell
git add .github/workflows/ci.yml internal/updatecheck/checker.go internal/updatecheck/checker_test.go
git commit -m "ci: publish portable exe and Windows MSI"
```

### Task 3: Document distribution and perform Windows install matrix verification

**Files:**
- Modify: README.md
- Modify: scripts/build.ps1 if smoke-test findings require a build correction
- Modify: build/windows/installer sources if install-matrix findings require an MSI correction

**Interfaces:**
- README documents how to download/use the portable executable and how MSI scope affects the install destination.

- [ ] **Step 1: Update download and local build documentation**

Document both GitHub Release assets. Explain that the portable EXE does not install the application, the MSI offers current-user and all-users choices, the former installs under LocalAppData without administrator rights, and the latter installs under Program Files and requests elevation. Clarify that profiles and logs remain in each Windows user's AppData in both cases.

- [ ] **Step 2: Run full Go/frontend build validation**

Run: go test ./...

Run in frontend: npm ci

Run in frontend: npm test

Run in frontend: npm run build

Run: .\\scripts\\build.ps1

Expected: PASS and both release artifacts are created with exact names.

- [ ] **Step 3: Exercise the MSI context and upgrade matrix on Windows**

On a standard user account, install current-user scope and verify files/shortcuts are under the user context, the app launches, and profiles/logs remain under that user's AppData. Upgrade that per-user install with a higher MSI version and confirm its profile remains. Uninstall, then install all-users scope and confirm Windows requests administrator approval and installs under Program Files. Create a test profile, upgrade that per-machine install with a higher MSI version and the same UpgradeCode, and verify the profile data remains unchanged. Uninstall and confirm profile data remains present.

- [ ] **Step 4: Commit distribution documentation and any verified packaging fixes**

```powershell
git add README.md scripts/build.ps1 build/windows/installer
git commit -m "docs: document portable and MSI distribution"
```
