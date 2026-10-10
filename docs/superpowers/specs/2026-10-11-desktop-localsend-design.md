# Desktop: LocalSend peer beside Taildrop

The desktop GUI speaks LocalSend protocol v2 on the local network while
it is open. A phone that has official LocalSend, and is not on the
tailnet, can see this computer and exchange files with it. Tailscale
discovery and Taildrop send stay as they are. The two lists can both
be visible.

Android and the CLI do not speak LocalSend in this slice.

## Goal

Transfer files with a LocalSend phone on the same Wi-Fi without joining
that phone to Tailscale first.

Success:

- Official LocalSend lists this computer and can send files to it.
- Tailsend lists that phone and can send the picked files to it.
- Incoming files ask before save. A setting turns on auto-save.
- Tailscale down, or no tailnet devices, still leaves the LocalSend
  list usable.

## Non-goals

- Android or CLI LocalSend
- Subnet HTTP scan (`/24` legacy discovery)
- LocalSend download API (`prepare-download` / browser URL)
- Directory trees in the protocol. A picked folder is flattened to files
- Requiring a PIN on this computer
- Closing another process that already holds port 53317
- Embedding Tailscale, or sending LocalSend frames over the tailnet

## Package

`internal/localsend` is the protocol. It does not import Wails or
`internal/tsdrop`.

`desktop.App` starts the node in `startup` and stops it on shutdown.
The Vue window renders devices, the incoming dialog, and the auto-save
checkbox. It does not open sockets.

## Identity

Generated once and stored under `os.UserConfigDir()/tailsend`:

- `localsend-cert.pem` and `localsend-key.pem`: self-signed leaf
- `localsend.json`: `{ "autoSave": false }`

Fingerprint is the SHA-256 of the certificate DER, lowercase hex, no
separators. Restarting the GUI keeps the same fingerprint.

Announcement fields:

| Field | Value |
|---|---|
| alias | OS hostname, or `Tailsend` if that is empty |
| version | `2.0` |
| deviceModel | `runtime.GOOS` |
| deviceType | `desktop` |
| protocol | `https` |
| port | `53317` |
| download | `false` |
| fingerprint | cert SHA-256 as above |

## Discovery

While the TCP port is bound:

- Listen for UDP on `224.0.0.167:53317`.
- Every 5 seconds, send one datagram with `announce: true`.
- On a datagram with `announce: true` and a different fingerprint:
  `POST https://<ip>:<port>/api/localsend/v2/register` with our info,
  and send one UDP reply with `announce: false`.
- Drop our own fingerprint.
- Remember alias, address, port, protocol, and fingerprint.
- Drop a peer that has not announced again for 30 seconds.

If TCP `0.0.0.0:53317` cannot bind, do not announce. UDP listen may
also fail when another LocalSend holds the port. The UI then shows
`53317 is in use`. Do not kill the other process.

No subnet scan. An empty list stays empty, with the copy below.

## HTTP server

Bind `0.0.0.0:53317` with the stored certificate.

| Route | Role |
|---|---|
| `GET /api/localsend/v1/info` and `GET /api/localsend/v2/info` | Our info JSON |
| `POST /api/localsend/v2/register` | Return our info. Record the caller as a peer |
| `POST /api/localsend/v2/prepare-upload` | Accept, reject, or wait |
| `POST /api/localsend/v2/upload` | Write one file |
| `POST /api/localsend/v2/cancel` | Abort the session |

`prepare-upload`:

- A second session while one is open returns `409`.
- `autoSave: false`: block until the UI accepts, declines, or 2 minutes
  pass. Decline and timeout return `403`. Accept returns `sessionId`
  and a token per accepted file.
- `autoSave: true`: accept immediately. No dialog.
- This computer does not require a PIN. A `pin` query is ignored.

`upload`:

- Missing session, file id, or token, or a token that does not match,
  returns `403`.
- `fileName` keeps only the last path segment. `..` and separators
  never enter the save path.
- Destination is `App.DefaultRecvDir()` (existing Downloads, otherwise
  the home directory). An existing name becomes `name (2).ext`.
- When the request included `sha256`, a mismatch returns `422` and
  deletes the partial file.
- A write error returns `500` and deletes the partial file.

`cancel` deletes files this session already wrote.

## Sending

The picked files are the same list as a Taildrop send. A directory is
walked and each regular file is uploaded under its base name. Two files
with the same base name get a numeric suffix. Do not zip.

Client flow to a remembered peer:

1. TLS to `https://<ip>:<port>`. Accept the certificate only when its
   SHA-256 matches the fingerprint from discovery.
2. `POST /api/localsend/v2/prepare-upload` with file metadata.
3. `401`: ask for a PIN once, retry with `?pin=`. A second `401` stops.
4. `403`: show rejected. `409`: show busy.
5. `POST /api/localsend/v2/upload` for each token. Reuse the existing
   progress event on the left of the window.
6. On local cancel, `POST /api/localsend/v2/cancel`.

A Taildrop send and a LocalSend receive may run at the same time.
A Taildrop send and a LocalSend send share the one progress line and
the existing `sending` lock, so only one outbound transfer runs.

## UI

Right column, under the Tailscale grid. The Tailscale empty state does
not replace this block.

**On this network**

- Status line: `53317 is in use` when bind failed. Otherwise no status
  line.
- Checkbox: **Save incoming files without asking.** Default off. Writes
  `localsend.json` immediately.
- Cards use the same click path as Tailscale cards (pick files if the
  list is empty, then send). Meta text is `LocalSend`. Icon follows
  `deviceType` (`mobile` or `desktop`). The map key is the fingerprint,
  so two peers may share an alias.
- Empty copy: `No LocalSend devices yet. Open LocalSend on the phone, on this Wi-Fi.`
- Device list updates from backend events. It does not wait for the
  15-second Tailscale poll.

Incoming dialog, only when auto-save is off:

- Title: `{alias} wants to send`
- Rows: file name and size
- **Save to Downloads** / **Decline**
- No answer in 2 minutes declines.

Auto-save on: no dialog. The left column notes how many files were
saved and from whom.

After a successful send: `Sent to {alias}. Their LocalSend app saves the files.`

PIN dialog: one field, one retry.

The Tailscale setup banner stays about Tailscale only.

## Errors

| Case | What the user sees |
|---|---|
| Port 53317 taken | Nearby heading: `53317 is in use`. No announce |
| Peer declined, or 2 minutes with no answer | Progress: rejected |
| PIN still wrong after one retry | Progress: that error. Stop |
| `409` | Progress: peer is busy |
| Cert fingerprint mismatch | Do not upload. Progress: certificate does not match |
| Incoming `sha256` mismatch | HTTP `422`, partial file removed |
| Peer cancels | Partial files from that session removed |
| File name has a path | Save the base name only |
| Disk write fails | HTTP `500`. Dialog or left column shows the error |
| Multicast hears nothing | Empty nearby list and the empty copy |

The operating system may show its own incoming-connection prompt the
first time the port binds. Tailsend does not add firewall rules.

## Tests

In-process HTTP and UDP. No live LAN and no Tailscale daemon.

- Parse an announce datagram. Drop our own fingerprint.
- `register` request and response match the v2 fields above.
- `prepare-upload`: accept, decline, 2-minute timeout, second session
  `409`.
- `autoSave: true` accepts `prepare-upload` without a waiter.
- `upload`: bad token is `403`. Bad `sha256` is `422` and leaves no
  file. Cancel removes a file already written. A path in `fileName`
  saves the base name only. Collision uses `name (2).ext`.
- Sender rejects a certificate whose fingerprint differs from discovery.
- PIN is retried once.
- A directory becomes one upload metadata entry per regular file.

## Docs

After implementation, update `README.md`, `README.zh.md`,
`docs/architecture.md`, and `CHANGELOG.md`: the desktop window is a
LocalSend peer on the LAN while it is open, separate from Taildrop.
