# Changelog

## Unreleased

### Windows Explorer drop

- Do not set `DisableWebViewDrop` on Windows (Wails v2.15 would call
  `AllowExternalDrag(false)` and Explorer drops never reach `OnFileDrop`).
  File picker was unaffected.

### Windows GUI icon

- Embed `rsrc_windows_amd64.syso` / `rsrc_windows_arm64.syso` so `Tailsend.exe`
  uses the paper-plane icon in the title bar, taskbar, and Explorer.

### CLI packages

- `make cli-all` / `scripts/build-cli.sh` cross-compiles
  `dist/tailsend-cli-{darwin,linux,windows}-{amd64,arm64}` (Windows gets `.exe`)
- Tag `v*` can attach those files to a GitHub Release. Copy
  `scripts/github-release-cli.yml` to `.github/workflows/` (needs a token
  with the `workflow` scope). The GUI is still built per-OS (CGO).

### Phase 1 — Desktop GUI

- `tailsend pair [--qr]`: same pairing JSON as the GUI Pair phone button
- Device cards use distinct per-OS icons (macOS / Windows / Linux / Android / iOS)
- App icon (`desktop/build/appicon.png`); macOS `make app` builds `Tailsend.app`
- Windows: hide the extra console when the GUI is double-clicked
- Linux: `Tailsend.desktop` + GTK window icon
- Wails v2 + Vue window: named-device grid, file drop/picker, send progress
- Inbox drawer (save to Downloads / chosen folder)
- Pair-phone QR of sendable targets
- Setup copy when Tailscale is down or needs login
- After send: landing hint (Downloads vs `tailsend recv .` / `tailscale file get .`)
- macOS: AppleScript file picker; link UniformTypeIdentifiers
- Windows: disable WebView2 drop overlay; Wails `OnFileDrop` for real paths
- Docs: PATH, winget, GOPROXY, WebView2 Evergreen vs Fixed Version cab

### Phase 0 — CLI

- `tailsend status|list|send|inbox|recv` on macOS, Linux, and Windows
- Talks to official `tailscaled` LocalAPI (no embedded node, no extra login)
- Device list by hostname; send destination uses `name:` like `scp`
- Directories sent as a zip
- Inbox drain with `--conflict=skip|overwrite|rename` and `--watch`
- Unit tests against a fake LocalAPI
