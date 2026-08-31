# Taildrop constraints

These limits come from Tailscale, not from Tailsend. Official docs:
[Taildrop](https://tailscale.com/docs/features/taildrop).

## Enable it

Taildrop is alpha. Each tailnet must opt in:

**Admin console → Settings → General → Send Files**

Until that is on, `tailsend list` / `send` fail with
`file sharing is not enabled on this tailnet`.

## Who can send to whom

Default:

- Same logged-in **user**
- **Untagged** devices
- Both sides running Tailscale

Not allowed by default:

- Another user’s devices on the same tailnet
- **Tagged** nodes (auth-key / server nodes almost always have tags)
- tvOS

Taildrop is allowed even when ACLs would otherwise block IP traffic between
those two personal devices.

### Tagged nodes / other users

Tailscale grants (example — check current docs before pasting into production):

```json
{
  "grants": [
    {
      "src": ["autogroup:member"],
      "dst": ["tag:server"],
      "app": {
        "https://tailscale.com/cap/file-send": [{}]
      }
    }
  ]
}
```

Related capability names seen in clients:

- `https://tailscale.com/cap/file-send`
- `https://tailscale.com/cap/file-sharing-target`

Without grants, Tailsend shows the peer in `list` with a reason such as
`owned by different user` or `not a Taildrop target` instead of a silent
empty list.

## Platform receive behavior

| OS | Receive |
|---|---|
| macOS | `~/Downloads`. First use: System Settings → Login Items & Extensions → Sharing → enable Tailscale. Resume **to** macOS/iOS may not work. |
| Windows | `Downloads` on v1.34+. Explorer context menu “Send with Tailscale” is the official sender; Tailsend is an alternative sender. |
| Linux | Inbox in the daemon. `tailsend recv .` or `sudo tailscale file get .`. Often needs root because `tailscaled` runs as root. |
| iOS / Android | Official Tailscale notification / Files / Downloads. Tailsend CLI does not run there yet. |

## Resume

Interrupted transfers can often resume for about an hour, except when macOS or
iOS is the **receiver**. Tailsend Phase 0 does not expose a separate resume
command; retrying `send` of the same file lets the daemon try.

## What Taildrop is not

- Not a folder-sync protocol (Tailsend zips directories).
- Not clipboard / text share (out of scope).
- Not a replacement for the Tailscale VPN app.
- Not LAN-only: traffic follows Tailscale (direct UDP or DERP).
