param(
    [Parameter()]
    [ValidatePattern('^(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])\.(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])\.(?:0|[1-9]\d?|1\d\d|2[0-4]\d|25[0-5])$')]
    [string]$AppVersion = '0.1.0'
)

$ErrorActionPreference = 'Stop'

Push-Location "$PSScriptRoot\.."
try {
    go test ./...

    Push-Location frontend
    try {
        npm ci
        npm test
        npm run build
    }
    finally {
        Pop-Location
    }

    if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
    }

    wails build -clean -trimpath -webview2 embed -windowsconsole -o go-script-sql-runner-portable.exe

    $outputDirectory = Join-Path (Get-Location) 'build\bin'
    $output = Join-Path $outputDirectory 'go-script-sql-runner-portable.exe'
    if (-not (Test-Path $output)) {
        throw "Expected executable was not produced at $output"
    }

    & (Join-Path $PSScriptRoot 'build-windows-installer.ps1') `
        -AppVersion $AppVersion `
        -ExecutablePath $output `
        -OutputDirectory $outputDirectory

    $installer = Join-Path $outputDirectory 'go-script-sql-runner-setup.msi'
    if (-not (Test-Path $installer)) {
        throw "Expected installer was not produced at $installer"
    }

    Write-Host "Build complete: $output and $installer"
}
finally {
    Pop-Location
}
