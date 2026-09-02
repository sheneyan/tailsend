# Later (not Phase 1)

## Receive-side transfer progress

The sender already has a progress bar. The receiver does not, because Taildrop
only lists a file in `WaitingFiles` after the transfer finishes (Linux inbox),
or writes straight to Downloads (macOS/Windows) inside the official Tailscale
app.

Possible later work: watch LocalAPI IPN bus `PartialFile` while Tailsend is
open on the **receiving desktop**. Phones still only get the official Tailscale
notification until the mobile app exists.

## tailcat

[tailcat](https://tailscale.com/blog/tailcat) is a separate data plane
(`tc…` addresses, no Tailscale login, `tailcat recv` / `tailcat cp`).
Tailsend Phase 0–1 is a Taildrop shell on the official daemon. Do not mix
tailcat into the named-device grid. A later mode could send to a `tc` address
as an opt-in path.

## Signed GUI installers

CLI zip/binaries ship on `v*` tags (`tailsend-cli-<os>-<arch>`). Signed GUI
`.dmg` / `.msi` / `.deb` are still later. Until then, use `docs/desktop.md`
**Packaging**: macOS `make app`, Windows `-H windowsgui`, Linux `.desktop` +
icon.
