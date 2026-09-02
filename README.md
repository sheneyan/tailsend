# Tailsend

LocalSend-style file transfer over **Tailscale Taildrop**.

Tailsend is a CLI and desktop GUI that talks to the official Tailscale daemon
already running on your machine. Android/iOS senders come later. It does
**not** create a second Tailscale node and has **no login of its own**. If
Tailscale is not running, Tailsend tells you to open the Tailscale app.

```bash
tailsend list
tailsend send ./photo.jpg pixel:
```

[中文说明](README.zh.md) · [Architecture](docs/architecture.md) · [Taildrop limits](docs/taildrop.md) · [Desktop GUI](docs/desktop.md)

> **Unofficial.** Not affiliated with [Tailscale](https://tailscale.com) or
> [LocalSend](https://localsend.org). Taildrop is a Tailscale feature
> (currently alpha). Tailsend is a client around it.

## Status

| Surface | State |
|---|---|
| CLI on macOS, Linux, Windows | **Phase 0 — usable** |
| Desktop GUI (Wails) | **Phase 1 — usable** |
| Android / iOS senders | Planned |

Requires [Tailscale](https://tailscale.com/download) installed and signed in.

## Prerequisites

1. Install and sign in to Tailscale on every device you want to send between.
2. In the [admin console](https://login.tailscale.com/admin/settings/general),
   turn on **Send Files**. Taildrop is an alpha opt-in per tailnet.
3. By default Taildrop only works between **the same user's untagged devices**.
   Tagged nodes (typical when you join with an auth key) need extra ACL
   grants. See [docs/taildrop.md](docs/taildrop.md).

There is no `tailsend login`.

## Install

### CLI (all platforms)

The CLI has **no CGO**. Build all six with `make cli-all` →
`dist/tailsend-cli-<os>-<arch>` (`darwin`/`linux`/`windows` × `amd64`/`arm64`).
To attach them to a GitHub Release, copy `scripts/github-release-cli.yml` to
`.github/workflows/` and push a `v*` tag (the first copy needs
`gh auth refresh -s workflow`). Rename a binary to `tailsend` (or
`tailsend.exe`) and put it on your `PATH`.

```bash
# macOS Apple Silicon
chmod +x tailsend-cli-darwin-arm64 && mv tailsend-cli-darwin-arm64 /usr/local/bin/tailsend

# Linux x86_64
chmod +x tailsend-cli-linux-amd64 && sudo mv tailsend-cli-linux-amd64 /usr/local/bin/tailsend
```

Windows: `tailsend-cli-windows-amd64.exe` → `tailsend.exe`.

From source, or `go install` (needs **Go 1.26+**; Ubuntu apt `golang-go` is too
old):

```bash
go install github.com/sheneyan/tailsend/cmd/tailsend@latest

git clone https://github.com/sheneyan/tailsend.git
cd tailsend
make cli                          # this machine
make cli-all                      # dist/tailsend-cli-* for six OS/arch pairs
```

The GUI (`Tailsend` / `Tailsend.exe`) is a separate CGO binary and **cannot**
be cross-compiled this way — see [docs/desktop.md](docs/desktop.md).

### Desktop GUI

Needs Go with CGO and the platform WebView. **Always** use `-tags production`.
Ubuntu 24 also needs `webkit2_41` (Wails defaults to webkit2gtk-4.0; 24.04
ships 4.1). Do **not** `sudo go build`. Do **not** use apt `golang-go`.

```bash
git clone https://github.com/sheneyan/tailsend.git
cd tailsend/desktop
# macOS / Windows:
go build -tags production -o Tailsend .          # Windows: Tailsend.exe
# Ubuntu 24:
# go build -tags production,webkit2_41 -o Tailsend .
./Tailsend
```

Without `-tags production` the binary starts then exits:
`will not build without the correct build tags`.

Full checklist (Go tarball, PATH, winget, GOPROXY, WebView2, GTK, error table):
**[docs/desktop.md](docs/desktop.md)**.

The window lists named devices (no IP field). Add files (click, or drop onto
the window), then click a device. After a send, the UI says where the file
should land (Downloads vs Linux inbox commands). **Inbox** drains the
Tailscale inbox. **Pair phone** (GUI) and `tailsend pair [--qr]` (CLI) export sendable
targets for a later mobile app.

## Usage

```
tailsend status
tailsend list                          # named devices; no IP required
tailsend list --json                   # sendable targets as JSON
tailsend pair [--qr]                   # same JSON as GUI Pair phone; --qr draws it
tailsend send <file-or-dir...> <device>:
tailsend inbox                         # waiting files (Linux / inbox mode)
tailsend recv [dir] [--watch] [--conflict=skip|overwrite|rename]
```

The last argument of `send` must end with `:` (same idea as `scp` and
`tailscale file cp`):

```bash
tailsend send README.md pixel:
tailsend send ./docs ./notes.txt macbook:
```

- Device names come from `tailsend list` (hostname, MagicDNS label, or
  Tailscale IP). You do not type IPs in the normal flow.
- Directories are zipped on the fly (`docs/` → `docs.zip`).
- Default `--conflict` for `recv` is `rename` (`note.txt` → `note-1.txt`).

### Where received files go

| Receiver OS | What happens |
|---|---|
| macOS | Usually `~/Downloads` (Tailscale direct-file mode) |
| Windows | Usually `Downloads` (Tailscale v1.34+) |
| Linux | Daemon **inbox** until you run `tailsend recv .` (same as `tailscale file get`) |
| iOS / Android | Official Tailscale app notification / Files |

On Linux the inbox is often owned by root because `tailscaled` runs as root.
You may need `sudo tailsend recv .`.

## How it differs from LocalSend

| | LocalSend | Tailsend |
|---|---|---|
| Network | LAN (mDNS) | Your tailnet (WireGuard / DERP) |
| Account | None | Reuses Tailscale |
| Internet | No | Yes, if Tailscale can reach the peer |
| Discovery | Nearby devices | `tailsend list` from the netmap |
| Folders | Native tree | Zip, then send |
| Receive confirm | In-app | Whatever Tailscale already does |

Tailsend is interoperable with official Taildrop: a file you send lands in
the same place as **Share → Tailscale** or `tailscale file cp`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `tailscale daemon is not running` | Open / start the Tailscale app |
| `file sharing is not enabled` | Admin console → Send Files |
| `owned by a different user` | Taildrop is same-user by default |
| device missing or “not a Taildrop target” | Tagged node, offline, or no PeerAPI — see [docs/taildrop.md](docs/taildrop.md) |
| `destination must end with ':'` | Use `pixel:` not `pixel` |
| Linux `recv` permission denied | Run as the same user as `tailscaled` (often `sudo`) |
| `proxy.golang.org` timeout | `go env -w GOPROXY=https://goproxy.cn,direct` — [docs/desktop.md](docs/desktop.md) |
| Win: drop shows a corner hint, tray empty | Rebuild a build that disables WebView2 drop; use `git pull` |
| Win: WebView2 `.cab` has no “Install” | That cab is Fixed Version (extract only). Use Evergreen `.exe` or the OS copy |
| GUI: `Package webkit2gtk-4.0 was not found` | Ubuntu 24: `-tags production,webkit2_41` — [docs/desktop.md](docs/desktop.md) |
| `invalid go version … must match format 1.23` | apt Go is too old; install https://go.dev/dl and put `/usr/local/go/bin` first |

Exit codes: `0` ok, `2` daemon/login, `3` policy/target, `4` I/O or other.

## Development

```bash
go test ./...
go build -o tailsend ./cmd/tailsend
```

Unit tests fake LocalAPI over HTTP. They do not need a live tailnet.

Layout:

```
cmd/tailsend/          CLI
desktop/               Wails v2 GUI (Vue)
internal/tsdrop/       LocalAPI client (send, inbox, zip)
internal/tsdrop/tsdroptest/   fake daemon for tests
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/architecture.md](docs/architecture.md).

## Roadmap

1. **Phase 0 (done)** — Go library + CLI
2. **Phase 1 (done)** — Desktop GUI (Wails) with a named-device grid
3. **Phase 2** — Android sender
4. **Phase 3** — iOS sender
5. **Phase 4** — Share-sheet polish, resume UX

Open items that are *not* Phase 1 (receive progress, tailcat, signed
installers) live in [TODO.md](TODO.md).

## License

[MIT](LICENSE). Uses the official Go LocalAPI client from
[`tailscale.com`](https://github.com/tailscale/tailscale) (BSD-3-Clause).

Tailscale, Taildrop, and LocalSend are trademarks of their respective owners.
