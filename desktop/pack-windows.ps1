# Build Tailsend.exe without a console. Icon comes from rsrc_windows_*.syso
# in this directory (resource ID 3). go-winres is optional: Defender often
# flags `go install github.com/tc-hib/go-winres`.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .

$winres = Get-Command go-winres -ErrorAction SilentlyContinue
if ($winres) {
    go-winres patch --in winres/winres.json --delete --no-backup .\Tailsend.exe
    Write-Host "stamped PE icon with go-winres"
} else {
    Write-Host "go-winres not on PATH (skipping PE patch; using rsrc_windows_*.syso + runtime icon)"
}
New-Item -ItemType Directory -Force -Path dist-release | Out-Null
Copy-Item -Force Tailsend.exe dist-release\Tailsend.exe
Write-Host "built $PWD\Tailsend.exe"
Write-Host "copied $PWD\dist-release\Tailsend.exe"
