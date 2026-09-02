#!/bin/sh
# Linux GUI tarball: Tailsend binary + .desktop + icon.
# Ubuntu 24: TAGS=production,webkit2_41 (default). Do not sudo.
set -e
cd "$(dirname "$0")"
TAGS=${TAGS:-production,webkit2_41}
ARCH=$(uname -m)
case "$ARCH" in
x86_64) ARCH=amd64 ;;
aarch64|arm64) ARCH=arm64 ;;
esac

go build -tags "$TAGS" -o Tailsend .

NAME="tailsend-gui-linux-${ARCH}"
DEST="dist-release/$NAME"
rm -rf "$DEST"
mkdir -p "$DEST"
cp Tailsend "$DEST/"
cp Tailsend.desktop "$DEST/"
cp build/appicon.png "$DEST/tailsend.png"
cat > "$DEST/README.txt" <<EOF
Tailsend GUI (Linux)

  chmod +x Tailsend
  ./Tailsend

Optional menu entry:

  mkdir -p ~/.local/share/applications ~/.local/share/icons/hicolor/512x512/apps
  cp Tailsend.desktop ~/.local/share/applications/
  cp tailsend.png ~/.local/share/icons/hicolor/512x512/apps/tailsend.png

Click the left pane to pick files (Ubuntu drop is supported in this build).
EOF

mkdir -p dist-release
tar -C dist-release -czf "dist-release/${NAME}.tar.gz" "$NAME"
echo "built $PWD/dist-release/${NAME}.tar.gz"
