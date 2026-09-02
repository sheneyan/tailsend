# Build Tailsend.exe without a console, then stamp the paper-plane icon
# into the PE (CGO/gcc often drops .syso). Requires gcc on PATH.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .
if (-not (Get-Command go-winres -ErrorAction SilentlyContinue)) {
    go install github.com/tc-hib/go-winres@latest
}
# Use PNG, not the .ico — go-winres cannot decode PNG-compressed ICO
# ("image: unknown format") and then leaves the default Windows icon.
go-winres patch --in winres/winres.json --delete --no-backup .\Tailsend.exe
Write-Host "built $PWD\Tailsend.exe"
