# Changelog

## Unreleased

### Omarchy

- `desktop/pack-omarchy.sh` builds a pacman package (`tailsend-gui` in the
  menu, `tailsend` on `PATH`). Depends on Arch `webkit2gtk-4.1`.

### Desktop LocalSend

- The open GUI is a LocalSend v2 peer on the LAN (announce, register,
  prepare-upload, upload, cancel). Tailscale discovery is unchanged.
- Incoming files prompt by default. Auto-save is a checkbox.
- Port 53317 already in use is shown. The other process is left alone.

### Phase 2 — Android sender

- `android/`: paste pairing JSON, pick files, PUT `{peerAPIURL}/v0/put/<file>`
- Pairing JSON includes the pairing computer first (`ExportTargetsJSON`)
- Android can scan the Pair phone QR (CameraX + ZXing); paste is debug-only
- Android appears in the system share sheet (`SEND` / `SEND_MULTIPLE`, `*/*`)
- On foreground, if there is no Tailscale address, ask to Start official
  Tailscale via `CONNECT_VPN` (stay in Tailsend). Open Tailscale UI is
  the fallback.
- Pair → Refresh from hostname: PUT `tailsend-list-please` to that PC’s
  PeerAPI; open GUI PushFile’s `tailsend-pair.json` (open-with on the phone).
  First import and closed-GUI fallback remain Scan QR.
- Pair QR is a compact minified snapshot (Low ECC, 1024px, ≤500 bytes) so
  phone cameras can read it from a screen; overflow devices use hostname refresh.
- OS device marks are PNG (`scripts/mkosicons.py`) on desktop and Android
- Docs: LocalSend discovery is UDP multicast (not broadcast/mDNS); Android
  has no netmap, so Pair QR is required for the first list. README /
  `docs/architecture.md` / `android/README.md` cover Scan QR, hostname
  refresh, compact QR, share sheet, Start via `CONNECT_VPN`, and
  `./scripts/build.sh`.

### Signing hooks

- Optional Apple notarization (`CODESIGN_IDENTITY`, `NOTARY_PROFILE`) and
  Authenticode (`SIGN_THUMBPRINT`). See docs/signing.md.

### Build cheat sheet

- `./scripts/build.sh` / `scripts/build.ps1` / `make help`: CLI, native GUI,
  Android APK, and the underlying commands.

### GUI release scripts

- macOS: `make dmg` writes unsigned `dist-release/Tailsend.dmg` (app + Applications link)
- Linux: `desktop/pack-linux.sh` writes `dist-release/tailsend-gui-linux-*.tar.gz`
- Windows: `pack-windows.ps1` also copies `Tailsend.exe` to `dist-release/`

### Windows Explorer drop

- Removed from the Windows GUI. Click the left pane to pick files. Drop is
  tracked in TODO.md (WebView2 + Defender false positives on window hooks).

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
