# Later (not Phase 1)

## Windows Explorer drag-and-drop

Windows GUI is click-to-pick only. Explorer drop into the WebView2 window
fought Wails `AllowExternalDrag`, OLE `IDropTarget` on nested Chrome HWNDs,
and unsigned-binary Defender heuristics (window subclassing looked like a
hook). Revisit without `SetWindowLongPtr` / `SetWindowSubclass`: either a
future Wails version that does not call `AllowExternalDrag(false)` when
`EnableFileDrop` is on, or a signed build plus a drop target that does not
replace the window procedure.

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

## Signed / notarized GUI installers

Unsigned packages are built on each OS with `desktop/pack-macos.sh` (`.dmg`),
`pack-windows.ps1` (`.exe`), `pack-linux.sh` (`.tar.gz`). Apple notarization,
Authenticode, and `.msi` / `.deb` are still later. CLI binaries already ship
on `v*` tags.
