[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])\.(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])\.(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])$')]
    [string]$AppVersion,

    [Parameter(Mandatory)]
    [string]$ExecutablePath,

    [Parameter(Mandatory)]
    [string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'
$wixVersion = '7.0.0'
$uiExtensionId = 'WixToolset.UI.wixext'
$uiExtensionVersion = '7.0.0'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$packageSource = Join-Path $repositoryRoot 'build\windows\installer\Package.wxs'
$scopeDialogSource = Join-Path $repositoryRoot 'build\windows\installer\ScopeSelectionDlg.wxs'

if (-not (Test-Path -LiteralPath $ExecutablePath -PathType Leaf)) {
    throw "Portable executable does not exist: $ExecutablePath"
}

$resolvedExecutable = (Resolve-Path -LiteralPath $ExecutablePath).Path
$resolvedOutputDirectory = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $resolvedOutputDirectory -Force | Out-Null
$installerPath = Join-Path $resolvedOutputDirectory 'go-script-sql-runner-setup.msi'

$toolDirectory = Join-Path $env:TEMP "go-script-sql-runner-tools\wix\$wixVersion"
$wixPath = Join-Path $toolDirectory 'wix.exe'
if (-not (Test-Path -LiteralPath $wixPath -PathType Leaf)) {
    New-Item -ItemType Directory -Path $toolDirectory -Force | Out-Null
    & dotnet tool install wix --version $wixVersion --tool-path $toolDirectory --verbosity quiet
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to install pinned WiX Toolset $wixVersion."
    }
}

$reportedWixVersion = (& $wixPath --version).Trim()
if ($LASTEXITCODE -ne 0 -or $reportedWixVersion -notmatch "^$([regex]::Escape($wixVersion))(\+|$)") {
    throw "Expected WiX Toolset $wixVersion, found '$reportedWixVersion'."
}

$extensionRoot = Join-Path $env:TEMP "go-script-sql-runner-wix-extensions-$wixVersion"
$previousExtensionRoot = $env:WIX_EXTENSION
try {
    # WIX_EXTENSION redirects WiX's global cache to this temporary task-specific path.
    $env:WIX_EXTENSION = $extensionRoot
    $extensionList = (& $wixPath extension list --global 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to inspect the temporary WiX extension cache: $($extensionList.Trim())"
    }
    if ($extensionList -notmatch [regex]::Escape("$uiExtensionId/$uiExtensionVersion")) {
        & $wixPath extension add --global "$uiExtensionId/$uiExtensionVersion" -acceptEula wix7
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to install pinned WiX extension $uiExtensionId $uiExtensionVersion."
        }
    }

    & $wixPath build $packageSource $scopeDialogSource `
        --arch x64 `
        --ext "$uiExtensionId/$uiExtensionVersion" `
        -d "AppVersion=$AppVersion" `
        -d "PortableExecutable=$resolvedExecutable" `
        -acceptEula wix7 `
        -o $installerPath
    if ($LASTEXITCODE -ne 0) {
        throw "WiX failed to build the installer at $installerPath."
    }
}
finally {
    if ($null -eq $previousExtensionRoot) {
        Remove-Item Env:\WIX_EXTENSION -ErrorAction SilentlyContinue
    }
    else {
        $env:WIX_EXTENSION = $previousExtensionRoot
    }
}

if (-not (Test-Path -LiteralPath $installerPath -PathType Leaf)) {
    throw "WiX completed without producing the expected MSI: $installerPath"
}
if ((Get-Item -LiteralPath $installerPath).Length -le 0) {
    throw "WiX produced an empty MSI: $installerPath"
}

Write-Host "Installer build complete: $installerPath"
