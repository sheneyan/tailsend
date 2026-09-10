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
| iOS / Android | Official Tailscale notification / Files / Downloads. Tailsend Android sends only; it does not receive. |

## Official Tailscale only

On **every** surface (CLI, desktop GUI, Android, later iOS) Tailsend is a
parasite: it never ships, sideloads, or embeds Tailscale (`tsnet` /
libtailscale). There is no `tailsend login` and no in-app substitute node.
That rule is older than the storefront note below; see
[architecture.md](architecture.md).

The official **iOS** Tailscale app is **not listed** in the App Store for
**mainland China, Russia, or Belarus**. That is Tailscale’s storefront
availability, not something Tailsend can patch.

If a device has no official Tailscale client, it cannot send or receive
through Tailsend. Obtain the client from [Tailscale’s download page](https://tailscale.com/download)
or from an App Store / Play Store region where Tailscale lists it. Tailsend
docs and code do **not** describe unofficial install paths (region switching,
sideloaded IPAs, or third-party apps that embed a Tailscale node).

## Resume

Interrupted transfers can often resume for about an hour, except when macOS or
iOS is the **receiver**. Tailsend Phase 0 does not expose a separate resume
command; retrying `send` of the same file lets the daemon try.

## What Taildrop is not

- Not a folder-sync protocol (Tailsend zips directories).
- Not clipboard / text share (out of scope).
- Not a replacement for the Tailscale VPN app.
- Not LAN-only: traffic follows Tailscale (direct UDP or DERP).
- Not LocalSend discovery. LocalSend announces on UDP multicast
  `224.0.0.167:53317` (LAN only). A tailnet has no such group; Tailsend
  lists devices from LocalAPI on desktop and from a Pair QR (or hostname
  refresh) on Android.
  Details: [architecture.md](architecture.md#discovery-not-localsend-multicast).
