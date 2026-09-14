# Architecture

Tailsend is a **parasite client**: it uses the Tailscale node you already have.
It never embeds `tsnet` / libtailscale and never performs Tailscale login —
CLI, desktop GUI, and later Android/iOS senders alike.

```
CLI `tailsend`  ─┐
Wails GUI        ├─►  internal/tsdrop  ──►  tailscaled LocalAPI
                                              │
                                              │  PeerAPI PUT /v0/put/<name>
                                              ▼
                                       peer's Tailscale inbox / Downloads
```

## Why not embed a node?

Auth keys usually create **tagged** nodes. Taildrop rejects tagged peers unless
the tailnet adds file-sharing grants. Reusing the logged-in GUI/CLI identity
keeps sends interoperable with Finder Share, the iOS/Android Tailscale share
sheet, and `tailscale file cp`.

Missing official clients in some App Store regions (mainland China, Russia,
Belarus) do not change this. See
[taildrop.md](taildrop.md#official-tailscale-only).

## LocalAPI used by Phase 0

| Method | Role |
|---|---|
| `Status` | Backend state, self, peer list, Taildrop eligibility |
| `FileTargets` | Detect “Send Files” disabled; PeerAPI URLs for JSON export |
| `PushFile` | Send (daemon does the PeerAPI hop, including netstack mode) |
| `WaitingFiles` / `GetWaitingFile` / `DeleteWaitingFile` | Linux-style inbox |
| `AwaitWaitingFiles` | `recv --watch` |

Desktop code must not PUT to PeerAPI itself. `file-put` is the supported path
when tailscaled is in userspace/netstack mode.

## Packages

| Path | Responsibility |
|---|---|
| `internal/tsdrop` | Probe, target list, send (files + zip dirs), inbox receive, error catalog |
| `internal/tsdrop/tsdroptest` | In-memory LocalAPI for unit tests |
| `cmd/tailsend` | CLI |
| `desktop` | Wails v2 + Vue GUI (build notes: [desktop.md](desktop.md)) |
| `android` | Kotlin sender: pair JSON + PeerAPI PUT (no LocalAPI, no embedded node) |

`Client.LC` is injectable so tests never talk to a real `tailscaled`.

## Send path

1. `Lookup` matches a name (hostname, MagicDNS label, StableID, or 100.x) to a
   sendable target.
2. Regular files: `PushFile` with `Content-Length`.
3. Directories: streamed zip (`docs/` → `docs.zip`) with unknown size (`-1`).
4. Multiple arguments are sequential puts. Taildrop has no multi-file transaction.

## Receive path

macOS/Windows typically write straight to Downloads (direct file mode). Linux
stages files in the daemon inbox; `tailsend recv` copies them out and deletes
the inbox entry. Conflict policy: `skip` | `overwrite` | `rename`.

## Discovery (not LocalSend multicast)

LocalSend finds peers with **UDP multicast** to `224.0.0.167:53317` (JSON
announce; HTTP `POST /register` back to the sender). That is not IPv4
broadcast (`255.255.255.255`) and not mDNS. Guest Wi‑Fi / AP isolation falls
back to scanning the LAN subnet on TCP 53317.

A tailnet is WireGuard unicast (or DERP). Multicast and LAN subnet scans do
not cross it. Tailsend therefore does **not** search for “online Tailsend
apps,” does not listen on a Tailsend-specific port, and does not embed a
second node to get a netmap.

| Surface | How the named-device grid is filled |
|---|---|
| CLI / desktop GUI | LocalAPI `Status` + `FileTargets` (true netmap) |
| Android | Pair JSON from a computer that *can* read LocalAPI (QR, or hostname refresh) |

Official Tailscale on the phone already has the netmap; a third-party app
cannot read it. VPN up only means the phone can send packets to a `100.x`
it already knows. MagicDNS can resolve a short name (`abc` → `100.x`) when
Tailscale DNS is on; it does **not** yield PeerAPI ports or the rest of the
grid. PeerAPI listens on a high port derived from the IP
(`(32<<10) | crc32` of the last three address bytes, a few retries, then a
random port).

So the phone still needs a snapshot of names + `peerAPIURL`. **Pair phone**
encodes `ExportTargetsQRJSON`: minified `stableID` / `hostname` / `os` /
`peerAPIURL` only, Low ECC, 1024px, capped at 500 bytes so a phone camera
can read it from a screen. Overflow peers are omitted; **Refresh from
hostname** fills the rest. Pretty `ExportTargetsJSON` is Debug copy / CLI
`tailsend pair` / the `tailsend-pair.json` file, not the QR. Pasting JSON
through another app is not a user path. `tskey-api` login is out of scope.

**First import is still Scan QR.** After that, Pair → **Refresh from hostname**:
type a MagicDNS short name (`abc`) of a computer whose **Tailsend GUI is
open**. The phone PUTs `tailsend-list-please` (`{"v":1,"from":["100.x"]}`)
to that machine’s existing PeerAPI (hashed ports, then give up — random
bind is why QR remains the fallback). The GUI’s Snapshot poll (~2s) reads
that name from the Linux inbox or `~/Downloads`, matches `from` to a
sendable peer, PushFile’s `tailsend-pair.json`, and deletes the command
file. Official Tailscale on the phone notifies; open that file in Tailsend
(VIEW/SEND intent). Unknown `from` IPs are dropped, not blasted. Tailscale
alone on the PC is not enough — Tailsend must be running. The command file
may flash as a received file before it is deleted.

## Later phases

Desktop GUI is Phase 1 (`desktop/`). Pairing payload is `tailsend pair` /
`list --json` / the GUI **Pair phone** button.

- **Android** (`android/`): cannot call LocalAPI. On foreground, if there is
  no Tailscale address, Tailsend may ask to **Start** official Tailscale
  (`com.tailscale.ipn.CONNECT_VPN`) and stay in Tailsend; first login and
  the VPN permission still happen in that app. Send with HTTP PUT to
  `{peerAPIURL}/v0/put/<file>` while the VPN is up. Pair by scanning the
  Pair phone QR, or refresh the list from a hostname whose Tailsend GUI is
  open. Receive stays in the official Tailscale app.
- **iOS:** same send model, not started.
- Receive-side progress, tailcat, signed installers: [TODO.md](../TODO.md).
