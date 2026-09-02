#!/bin/sh
# Regenerates rsrc_windows_*.syso (exe icon + GUI manifest) from build/appicon.png.
# Needs: go install github.com/tc-hib/go-winres@latest
set -e
cd "$(dirname "$0")"
go-winres simply \
	--arch amd64,arm64 \
	--icon build/appicon.png \
	--manifest gui \
	--product-name Tailsend \
	--file-description "Send files over Tailscale Taildrop" \
	--original-filename Tailsend.exe \
	--product-version 0.1.0 \
	--file-version 0.1.0 \
	--copyright MIT \
	--out rsrc
ls -l rsrc_windows_*.syso
