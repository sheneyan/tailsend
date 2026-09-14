# Android: prompt then wake official Tailscale

When the Android sender comes to the foreground without a Tailscale
address on any NIC, ask once: start Tailscale? Yes sends the official
app’s `CONNECT_VPN` broadcast and stays in Tailsend. Opening Tailscale’s
UI is only the fallback.

Tailsend still does not embed a node.

## Goal

Share-into-Tailsend and cold launch both fail today if the user forgot
to connect Tailscale. Detect **Tailscale**, not “some VPN”, then let the
user start it without leaving Tailsend when the official app allows it.

## Non-goals

- Embedding Tailscale or any login/auth-key path
- Silently starting Tailscale’s Activity (Android will not hide it)
- Treating `NetworkCapabilities.TRANSPORT_VPN` as “Tailscale is up”
- Blocking send / Pair / hostname refresh until connected
- iOS
- Auto-connecting on a timer while Tailsend stays in the background

## Detection

Reuse `tailscaleIPs()` / `isTailscaleAddr()`:

- IPv4 `100.64.0.0/10`
- IPv6 `fd7a:115c:a1e0::/48`

Empty list → Tailscale is not up for Tailsend. A Clash / corporate /
WireGuard VPN does not count as connected.

Installed official app: `PackageManager` can resolve package
`com.tailscale.ipn`. Android 11+ needs a manifest `<queries>` entry for
that package only. Do not request `QUERY_ALL_PACKAGES`.

## Prompt rules

Check on `MainActivity.onResume` (covers launcher and share-sheet
entry).

| Condition | UI |
|---|---|
| `tailscaleIPs()` non-empty | No prompt |
| Official package missing | Copy only: install and sign in. No Start |
| User tapped Not now in this visible session | No prompt |
| Otherwise | Confirm dialog |

A “visible session” ends when the **process** goes to the background
(`ProcessLifecycleOwner` ON_STOP), not when in-app Pair / scan overlays
pause the activity. Configuration change must not re-ask or reset
decline.

Send, Pair, and picking files stay enabled under the dialog.

## Confirm dialog (English, same voice as the rest of the Android UI)

1. Idle need: **Tailscale is not connected. Start it?**  
   Buttons: **Start** / **Not now**
2. After Start, while polling: **Starting Tailscale…** (no extra
   buttons required)
3. Poll timeout: **Could not start Tailscale. It may need login, VPN
   permission, or another VPN is active.**  
   Button: **Open Tailscale**
4. Package missing (or launch intent missing): **Install official
   Tailscale and sign in. Tailsend does not embed a node.**

Do not link to unofficial clients.

## Start path

**Start** does not call `startActivity` on Tailscale.

Send an **explicit** broadcast:

- Action: `com.tailscale.ipn.CONNECT_VPN`
- Package: `com.tailscale.ipn`
- Class: `com.tailscale.ipn.IPNReceiver`

That is Tailscale’s documented external “start the VPN” receiver. It
does not bring Tailscale to the foreground. It only works if Tailscale
has already been logged in and the user has granted VPN permission at
least once.

Then poll `tailscaleIPs()` every 500ms for **8 seconds**:

- Addresses appear → dismiss the dialog
- Still empty → switch to the timeout copy and **Open Tailscale**
- `sendBroadcast` throws → same timeout UI (the receiver has no
  success callback)

**Open Tailscale** uses `getLaunchIntentForPackage("com.tailscale.ipn")`.
That is the only step that leaves Tailsend. If the intent is null, show
the install copy.

**Not now** dismisses the dialog and suppresses it until the process
backgrounds.

## Components

| Unit | Responsibility |
|---|---|
| `tailscaleIPs` / `isTailscaleAddr` (existing) | “Is the tailnet interface up?” |
| New small helper (e.g. `TailscaleWake`) | Package present? send `CONNECT_VPN`; launch official UI. Injectable for tests |
| `SendViewModel` | Prompt phase: none / confirm / starting / failed / missing; decline-for-session |
| `MainActivity.onResume` | Kick a check when visible |
| `TailsendScreen` | Draw the dialog from state; no package manager calls |

Keep broadcast / `PackageManager` out of composables.

## Error handling

- CONNECT_VPN is fire-and-forget; Tailsend infers success only from
  addresses.
- Another VPN occupying Android’s single VPN slot looks like timeout;
  Tailscale’s own UI explains the conflict after **Open Tailscale**.
- Existing PeerAPI errors stay: “Open the official Tailscale app…”.
- No crash if Tailscale is missing or the receiver is absent.

## Tests

No live tailnet, no real `CONNECT_VPN` in CI.

- Empty IPs + installed + not declined → confirm
- Any Tailscale IPv4 or IPv6 → no prompt
- Declined this session → no prompt
- Package missing → missing/install phase, helper must not send
  broadcast
- After a fake “Start”, IPs appear before 8s → dismiss
- After Start, IPs stay empty → failed / Open Tailscale

## Docs

- `android/README.md`: on open, Tailsend may ask to start official
  Tailscale via `CONNECT_VPN`; first-time login/VPN permission still
  happens in that app.
- `CHANGELOG.md` Unreleased / Phase 2.

## Manual check

1. Tailscale logged in, VPN off, Tailsend open → confirm → Start → VPN
   comes up, dialog goes away, send still works.
2. Not now → no re-prompt until Tailsend is backgrounded and resumed.
3. Tailscale uninstalled → install copy, no broadcast.
4. Another VPN on, Tailscale off → Start times out → Open Tailscale.
5. Share a photo into Tailsend with Tailscale off → same confirm on
   top of the send tray.
