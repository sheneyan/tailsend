# Security

Tailsend is a local client. It speaks to `tailscaled` over the platform
LocalAPI (unix socket / named pipe / macOS GUI token) and asks the daemon to
push files through Tailscale’s existing Taildrop path.

## Trust model

- Identity is the already-logged-in Tailscale node. Tailsend does not store
  Tailscale auth keys or account passwords.
- Authorization is whatever Taildrop already enforces (same user, untagged,
  or tailnet grants).
- File bytes go over WireGuard (or DERP). Tailsend does not add another
  encryption layer.

## Please do not

- Paste `tskey-auth-` / `tskey-api-` / tailnet policy files into issues
- Open LocalAPI to the network
- Run untrusted binaries as root just to drain the Linux inbox

## Reporting

Open a **private** GitHub security advisory if the repo has that enabled,
or email the maintainer listed on the GitHub profile. Give us time to fix
before publishing details.

LocalAPI access is equivalent to controlling Tailscale on that machine.
Treat a `tailsend` binary with the same care as the `tailscale` CLI.
