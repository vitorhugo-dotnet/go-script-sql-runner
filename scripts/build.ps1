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

    wails build -clean -trimpath -webview2 embed -windowsconsole -o go-script-sql-runner.exe

    $output = Join-Path (Get-Location) 'build\bin\go-script-sql-runner.exe'
    if (-not (Test-Path $output)) {
        throw "Expected executable was not produced at $output"
    }

    Write-Host "Build complete: $output"
}
finally {
    Pop-Location
}
