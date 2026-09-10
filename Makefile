.PHONY: help cli cli-all gui android android-install test all

# One command to remember: make help   or   ./scripts/build.sh
help:
	@sh scripts/build.sh help

# Current machine only (fast). Output: ./tailsend or ./tailsend.exe
cli:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o tailsend ./cmd/tailsend

# All desktop OS/arch pairs → dist/tailsend-cli-*
# GUI cannot be cross-compiled (CGO / WebView); see docs/desktop.md.
cli-all:
	sh scripts/build-cli.sh

gui:
	sh scripts/build.sh gui

android:
	sh scripts/build.sh android

android-install:
	sh scripts/build.sh android-install

test:
	go test ./...
	go test ./desktop

all:
	sh scripts/build.sh all
