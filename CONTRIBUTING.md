# Contributing

## Setup

- Go 1.24+ (CI/dev: 1.27)
- Official Tailscale is only needed for manual checks, not for `go test`

```bash
git clone https://github.com/sheneyan/tailsend.git
cd tailsend
go test ./...
go test ./desktop
go build -o tailsend ./cmd/tailsend
```

GUI (`desktop/`) is a nested module (`replace` to the parent). From repo root,
`go.work` includes both. GUI tests still use the fake LocalAPI.

## Tests

Unit tests stand up an in-memory LocalAPI (`internal/tsdrop/tsdroptest`).
They must not require a logged-in tailnet, API keys, or network to
`api.tailscale.com`.

```bash
go test ./...
go test ./desktop
gofmt -w .
go vet ./...
```

If `proxy.golang.org` times out (common in China):

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```

If you change LocalAPI usage, extend the fake daemon and add a test that
fails without your production change.

## Scope

Phase 0–1 are CLI + `internal/tsdrop` + Wails GUI. Please do not land Android
or iOS in the same PR unless that phase is underway.

Keep send/receive on Taildrop / LocalAPI. Do not add a parallel custom
transfer protocol.

## Commit style

Short imperative subject, optional body:

```
feat: zip directories on send
fix: map 403 owned-by-other-user to ErrOtherUser
docs: document Linux inbox permissions
```

## Manual check (optional)

With Tailscale Running:

```bash
./tailsend status
./tailsend list
./tailsend send ./README.md <device>:
```

Do not commit binaries, API keys, or tailnet dumps.
