#!/bin/sh
# Omarchy pacman package. Run on that machine (Arch + makepkg). Do not sudo.
# Writes dist-release/tailsend-0.1.0-1-<arch>.pkg.tar.zst
set -eu
cd "$(dirname "$0")"
ROOT=$(CDPATH= cd -- .. && pwd)

if ! command -v makepkg >/dev/null 2>&1; then
	echo "makepkg not found. Run this on Omarchy." >&2
	exit 1
fi
if ! command -v go >/dev/null 2>&1; then
	echo "go not on PATH (need 1.26+)" >&2
	exit 1
fi

case "$(uname -m)" in
x86_64|aarch64) ;;
*)
	echo "unsupported machine: $(uname -m)" >&2
	exit 1
	;;
esac

echo "→ GUI (webkit2gtk-4.1)"
go build -tags production,webkit2_41 -o Tailsend .
echo "→ CLI"
here=$PWD
(cd "$ROOT" && CGO_ENABLED=0 go build -o "$here/tailsend-cli" ./cmd/tailsend)

STAGE=$(mktemp -d)
cleanup() { rm -rf "$STAGE"; rm -f tailsend-cli; }
trap cleanup EXIT

cp packaging/omarchy/PKGBUILD packaging/omarchy/tailsend.desktop "$STAGE/"
cp Tailsend tailsend-cli build/appicon.png "$STAGE/"
(
	cd "$STAGE"
	makepkg -f --noconfirm
)
mkdir -p dist-release
cp "$STAGE"/*.pkg.tar.* dist-release/
echo "built:"
ls -1 dist-release/*.pkg.tar.*
echo "install: sudo pacman -U dist-release/tailsend-0.1.0-1-*.pkg.tar.zst"
