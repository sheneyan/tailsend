# Changelog

## Unreleased

### Phase 0 — CLI

- `tailsend status|list|send|inbox|recv` on macOS, Linux, and Windows
- Talks to official `tailscaled` LocalAPI (no embedded node, no extra login)
- Device list by hostname; send destination uses `name:` like `scp`
- Directories sent as a zip
- Inbox drain with `--conflict=skip|overwrite|rename` and `--watch`
- Unit tests against a fake LocalAPI
