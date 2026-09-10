# One command to remember on Windows. GUI cannot be cross-compiled.
# Usage (repo root):
#   powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
#   powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 gui
param(
    [Parameter(Position = 0)]
    [string]$Target = "help"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Show-Help {
    @"
Tailsend build (Windows)

  .\scripts\build.ps1                 print this help
  .\scripts\build.ps1 cli             CLI for this machine -> .\tailsend.exe
  .\scripts\build.ps1 cli-all         CLI for six OS/arch -> dist\tailsend-cli-*
  .\scripts\build.ps1 gui             Tailsend.exe (no extra console window)
  .\scripts\build.ps1 test            Go tests (including desktop)
  .\scripts\build.ps1 all             cli-all + gui

Underlying commands
  CLI   go build -o tailsend.exe .\cmd\tailsend
        or Git Bash / make cli-all
  GUI   cd desktop; powershell -ExecutionPolicy Bypass -File .\pack-windows.ps1
  test  go test ./... ; go test ./desktop

Always use -tags production. Do not use old apt/msys Go. Proxy:
  go env -w GOPROXY=https://goproxy.cn,direct
"@
}

function Need-Go {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go not on PATH (need 1.26+)"
    }
}

switch ($Target) {
    { $_ -in "help", "-h", "--help" } { Show-Help }
    "cli" {
        Need-Go
        Write-Host "→ CLI (this machine)"
        go build -trimpath -ldflags "-s -w" -o tailsend.exe .\cmd\tailsend
        Write-Host "wrote $Root\tailsend.exe"
    }
    "cli-all" {
        Need-Go
        Write-Host "→ CLI all platforms"
        sh "$Root\scripts\build-cli.sh"
    }
    "gui" {
        Need-Go
        Write-Host "→ Windows GUI"
        & "$Root\desktop\pack-windows.ps1"
    }
    "test" {
        Need-Go
        go test ./...
        go test ./desktop
    }
    "all" {
        Need-Go
        sh "$Root\scripts\build-cli.sh"
        & "$Root\desktop\pack-windows.ps1"
    }
    default {
        Write-Error "unknown target: $Target"
        Show-Help
        exit 2
    }
}
