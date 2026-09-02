.PHONY: cli cli-all test

# Current machine only (fast). Output: ./tailsend or ./tailsend.exe
cli:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o tailsend ./cmd/tailsend

# All desktop OS/arch pairs → dist/tailsend-cli-*
# GUI cannot be cross-compiled (CGO / WebView); see docs/desktop.md.
cli-all:
	sh scripts/build-cli.sh

test:
	go test ./...
	go test ./desktop
