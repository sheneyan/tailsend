#!/bin/sh
# Cross-compile tailsend CLI (no CGO). GUI cannot be cross-compiled this way.
set -e
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
OUT=${OUT:-"$ROOT/dist"}
mkdir -p "$OUT"

build_one() {
	os=$1
	arch=$2
	name="tailsend-cli-${os}-${arch}"
	if [ "$os" = windows ]; then
		name="${name}.exe"
	fi
	echo "→ $name"
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
		go build -C "$ROOT" -trimpath -ldflags "-s -w" \
		-o "$OUT/$name" ./cmd/tailsend
}

build_one darwin arm64
build_one darwin amd64
build_one linux amd64
build_one linux arm64
build_one windows amd64
build_one windows arm64

echo "wrote:"
ls -lh "$OUT"/tailsend-cli-*
