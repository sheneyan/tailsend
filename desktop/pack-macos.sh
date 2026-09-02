#!/bin/sh
# Wrap the GUI binary in Tailsend.app so double-click does not open Terminal
# and the Dock uses build/appicon.png.
set -e
cd "$(dirname "$0")"
TAGS=${TAGS:-production}
go build -tags "$TAGS" -o Tailsend .

APP=Tailsend.app
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp Tailsend "$APP/Contents/MacOS/Tailsend"
cp Info.plist "$APP/Contents/Info.plist"

ICONSET=build/Tailsend.iconset
rm -rf "$ICONSET"
mkdir -p "$ICONSET"
sips -z 16 16     build/appicon.png --out "$ICONSET/icon_16x16.png" >/dev/null
sips -z 32 32     build/appicon.png --out "$ICONSET/icon_16x16@2x.png" >/dev/null
sips -z 32 32     build/appicon.png --out "$ICONSET/icon_32x32.png" >/dev/null
sips -z 64 64     build/appicon.png --out "$ICONSET/icon_32x32@2x.png" >/dev/null
sips -z 128 128   build/appicon.png --out "$ICONSET/icon_128x128.png" >/dev/null
sips -z 256 256   build/appicon.png --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256   build/appicon.png --out "$ICONSET/icon_256x256.png" >/dev/null
sips -z 512 512   build/appicon.png --out "$ICONSET/icon_256x256@2x.png" >/dev/null
sips -z 512 512   build/appicon.png --out "$ICONSET/icon_512x512.png" >/dev/null
sips -z 1024 1024 build/appicon.png --out "$ICONSET/icon_512x512@2x.png" >/dev/null
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/appicon.icns"
rm -rf "$ICONSET"

echo "built $PWD/$APP"
echo "open with: open $APP"
