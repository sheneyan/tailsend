# Tailsend Android (Phase 2)

Sender only. The app does **not** embed Tailscale. Official Tailscale VPN
must be connected; receive stays in that app.

The phone cannot read Tailscale’s netmap (that lives in the official app).
VPN + MagicDNS can resolve a hostname to `100.x`; they do not list devices
or PeerAPI ports. LocalSend-style UDP multicast (`224.0.0.167:53317`) does
not cross a tailnet. First import is a **compact Pair QR** (`ExportTargetsQRJSON`)
from a computer that talks to LocalAPI. If the code is a black square that no
camera can read, rebuild that computer’s GUI (`./scripts/build.sh gui`). Why:
[docs/architecture.md](../docs/architecture.md#discovery-not-localsend-multicast).

1. Pair: on a computer, Tailsend → **Pair phone**. On the phone, Pair →
   **Scan QR**. Do not shuttle the JSON through another app. (Debug paste is
   hidden behind a disclosure.)
2. Later refresh: type that computer’s MagicDNS short name while its
   Tailsend window is open → **Refresh from hostname**. Wait for official
   Tailscale to notify `tailsend-pair.json`, then open it in Tailsend.
   If the GUI is closed, Scan QR.
3. Pick files (SAF), or **Share → Tailsend** from Gallery / Files (single or
   multiple). Then tap a device.
4. HTTP PUT `{peerAPIURL}/v0/put/<file>` on the tailnet.

The share sheet lists Tailsend for any MIME (`SEND` / `SEND_MULTIPLE`). A
`tailsend-pair.json` share still imports pairing; other files go to the send
tray. After installing, open Tailsend once if it does not appear yet.

OS marks are PNG files (same art as the desktop GUI), not emoji.

## Build

Set `ANDROID_HOME` or `android/local.properties` `sdk.dir` (gitignored).

From the repo root (reads `ANDROID_HOME` or `android/local.properties`):

```bash
./scripts/build.sh android
./scripts/build.sh android-install   # adb device already connected
```

Or by hand:

```bash
export ANDROID_HOME=/Users/sheneyan/Documents/Work/Sources/AI/android-sdk
cd android
./gradlew :app:test :app:assembleDebug
```

APK: `android/app/build/outputs/apk/debug/app-debug.apk`

No login, no inbox, no directory zip. Camera is the pairing path.
