# Build Tailsend.exe without a console, then stamp the paper-plane icon
# into the PE (CGO/gcc often drops .syso). Requires gcc on PATH.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .
if (-not (Get-Command go-winres -ErrorAction SilentlyContinue)) {
    go install github.com/tc-hib/go-winres@latest
}
# go-winres requires PNG <= 256x256 (appicon-256.png). A 1024 PNG prints
# "image: unknown format" / "must fit in 256x256" and leaves the default icon.
go-winres patch --in winres/winres.json --delete --no-backup .\Tailsend.exe
Write-Host "built $PWD\Tailsend.exe"
