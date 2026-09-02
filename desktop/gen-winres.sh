#!/bin/sh
# Regenerates rsrc_windows_*.syso (exe icon + GUI manifest) from build/appicon.png.
# Needs: go install github.com/tc-hib/go-winres@latest
set -e
cd "$(dirname "$0")"
go-winres make --in winres/winres.json --arch amd64,arm64 --out rsrc
ls -l rsrc_windows_*.syso
