# Architecture

Tailsend is a **parasite client**: it uses the Tailscale node you already have.
It never embeds `tsnet` / libtailscale and never performs Tailscale login.

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
| `desktop` | Wails v2 + Vue GUI |

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

## Later phases (not in this tree)

- **Desktop GUI:** Wails over the same `tsdrop` client. Named-device grid from
  `Targets()`, no IP field on the home screen.
- **Android / iOS:** cannot call LocalAPI (sandbox). Send with HTTP PUT to
  `http://<tailscale-ip>:<peerapi>/v0/put/<file>` while the system Tailscale
  VPN is up. Discovery via `tskey-api` device list + port probe, or QR of
  `tailsend list --json`. Receive stays in the official Tailscale app.
