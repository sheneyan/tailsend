# Changelog

## Unreleased

### Phase 1 — Desktop GUI

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
