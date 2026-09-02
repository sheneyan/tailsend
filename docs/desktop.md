# Building and using the desktop GUI

The GUI lives in `desktop/` (Wails v2 + Vue). The compiled frontend is already
in `desktop/frontend/dist`; a normal rebuild does **not** need `npm`.

## Compilation notes

Read this before `go build`. Most failures so far were one of these.

### Commands

| OS | Command |
|---|---|
| macOS (raw binary) | `cd desktop && go build -tags production -o Tailsend .` |
| macOS (Dock / no Terminal) | `cd desktop && make app` then `open Tailsend.app` |
| Windows | `cd desktop && go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .` |
| Linux (Ubuntu 24 / webkit2gtk 4.1) | `cd desktop && go build -tags production,webkit2_41 -o Tailsend .` |
| Linux (webkit2gtk 4.0 only) | `cd desktop && go build -tags production -o Tailsend .` |

Equivalent: `make run` on macOS; on Ubuntu 24 `make TAGS=production,webkit2_41 run`.

### Do / do not

- **Always** pass `-tags production` (or `-tags dev`). A plain `go build` links
  but the app exits: `will not build without the correct build tags`.
- On Ubuntu 24 also pass **`webkit2_41`**. Wails defaults to webkit2gtk-4.0;
  24.04 ships 4.1. Missing the tag looks like `Package webkit2gtk-4.0 was not found`.
- **Do not** `sudo go build` / `sudo go test`. Root uses a different `PATH` and
  module cache; the binary will be owned by root.
- **Do not** use Ubuntu `apt install golang-go`. It is too old. Need **Go 1.26+**
  (this repo’s `tailscale.com` v1.102 requires 1.26.6). Install the official
  tarball from https://go.dev/dl into `/usr/local/go` and put
  `/usr/local/go/bin` **first** on `PATH`.
- `go.work` / `go.mod` say `go 1.22` so old parsers can *read* the file. That
  does not mean you can *compile* with Go 1.22.
- CGO is required for the GUI (`gcc` / MinGW). The CLI in `cmd/tailsend` does
  not need CGO and is cross-compiled as `tailsend-cli-<os>-<arch>` (`make cli-all`).
- Package names have **no trailing `~`**. `apt install libwebkit2gtk-4.1-dev~`
  fails with `Unable to locate package`.
- After `git pull`, rebuild. Running an old `./Tailsend` is the previous build.

### If the compiler (or the binary) says…

| Message / symptom | Fix |
|---|---|
| `will not build without the correct build tags` | Add `-tags production` (Linux 24: `production,webkit2_41`) |
| `invalid go version '1.27.0': must match format 1.23` | Still on apt Go. `which go` must be `/usr/local/go/bin/go`, not `/usr/bin/go` |
| `Package webkit2gtk-4.0 was not found` | Ubuntu 24: install `libwebkit2gtk-4.1-dev` and add `,webkit2_41` |
| `Unable to locate package libwebkit2gtk-4.1-dev~` | Drop the `~`. Enable `universe` if 4.1 is missing: `sudo add-apt-repository universe` |
| `gcc: command not found` / `cgo: C compiler "gcc" not found` | Linux: `sudo apt install gcc pkg-config`. Windows: MinGW on `PATH` (see below) |
| `multiple definition of tailsendScheduleLinuxDrop` | Old tree; `git pull` — export and C bodies are split on `main` |
| linker `UTType` / `_OBJC_CLASS_$_UTType` | Already handled by `desktop/link_darwin.go`; `git pull` |
| `proxy.golang.org` / `dial tcp 142.250…:443` timeout | `go env -w GOPROXY=https://goproxy.cn,direct` and `GOSUMDB=sum.golang.google.cn` |
| WebView2 `.cab` has no Install | That file is Fixed Version (extract only). Use Evergreen, not the `.cab` |
| GUI starts then blank / GPU `libEGL` `Permission denied` on `/dev/dri/renderD128` | Software fallback; Inbox still works. Optional: `sudo usermod -aG render,video $USER` and log out |

## macOS

WebKit is built in. Apple Silicon / recent Xcode may warn that
`setShowsBaselineSeparator:` is deprecated; ignore it.

File **picker** uses AppleScript (`osascript choose file`), not a Wails sheet.
Wails sheets close immediately on click in the WebView. Folders can still be
dropped on the window.

Traffic-light buttons: the title bar is hidden-inset; the header leaves space
on the left.

## Windows 11

Needs Go (CGO), `gcc`, and **Evergreen** WebView2 (Win11 usually already has
it). Do **not** install `Microsoft.WebView2.FixedVersionRuntime.*.cab`.

### PATH

Settings → search **environment variables** → **Edit the system environment
variables** → **Environment Variables** → user **Path** → **New**:

- Go: `C:\Program Files\Go\bin`
- MinGW / WinLibs: `...\mingw64\bin`

OK everything, **open a new PowerShell**.

### winget (if missing)

```powershell
winget --version
# if that fails:
Install-PackageProvider -Name NuGet -Force
Install-Module -Name Microsoft.WinGet.Client -Force -Repository PSGallery
Repair-WinGetPackageManager -Force
```

Then:

```powershell
winget install -e --id GoLang.Go
winget install -e --id Microsoft.EdgeWebView2Runtime
winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT
```

New terminal, then `go version` and `gcc --version`.

### Go module proxy (China / blocked Google)

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
cd D:\tailsend\desktop
go build -tags production -o Tailsend.exe .
.\Tailsend.exe
```

Fallback: `go env -w GOPROXY=https://goproxy.io,direct`.

### Drag-and-drop

Explorer drops use Wails `OnFileDrop` (real `D:\...` paths), including `.exe`.
Wails v2.15 must **not** set `DisableWebViewDrop` on Windows: that calls
`AllowExternalDrag(false)` and the drop never reaches JS. JS `preventDefault`
stops WebView2 from opening the file. After `git pull`, rebuild.

Click-to-pick uses the native Windows dialog (not AppleScript).

## Linux (Ubuntu 24 + XFCE is fine)

Needs GTK3 + WebKit **dev packages**, not GNOME. XFCE is fine.

### Packages

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 && echo ok
```

If `apt` has no 4.1 package, install `libwebkit2gtk-4.0-dev` and **omit**
`,webkit2_41` from the build tags.

### Official Go (not apt)

```bash
go version
which go    # must become /usr/local/go/bin/go, not /usr/bin/go

sudo apt remove -y golang-go golang-1.22-go 2>/dev/null
cd /tmp
wget https://go.dev/dl/go1.27.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.27.0.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc
hash -r
go version   # go1.27.0
```

If `go.work` errors with `must match format 1.23`, you are still on the apt
binary: `/usr/bin/go` is ahead of `/usr/local/go/bin` in `PATH`.

### Build and run

```bash
cd tailsend
git pull
cd desktop
go build -tags production,webkit2_41 -o Tailsend .
./Tailsend
```

**Inbox:** Linux does not auto-save to Downloads. Send a file to this machine,
then **Inbox** in the GUI, or `tailsend recv .` / `sudo tailscale file get .`.
`tailscaled` often runs as root, so recv may need `sudo`.

**Drag-and-drop:** Drop onto the window and **release the mouse**. The file
should appear in the left tray, not open as a WebKit page. Click-to-pick still
works.

If an older build left the mouse unable to click anywhere, the X11 pointer grab
was still held: press **Escape**, or from another TTY/SSH run `killall Tailsend`.
Then rebuild this version.

`libEGL` / `DRI3` `Permission denied` on `/dev/dri/renderD128` is GPU access;
WebKit falls back to software. Inbox/UI still work. To quiet it:
`sudo usermod -aG render,video $USER` then log out and back in.

## Packaging (no extra console, real app icon)

`go build` of the GUI is a **development binary**. It is not a signed
installer. Double-clicking that binary looks unfinished:

| OS | What you see | What to do |
|---|---|---|
| Windows | A console window behind the GUI; generic title-bar icon | Build with `-ldflags "-H windowsgui"`. Current `main` also hides a console that *this* process owns (double-click). The exe icon is `rsrc_windows_*.syso` (from `build/appicon.png`); rebuild after `git pull`. If Explorer still shows the old icon, copy the new exe to a new folder — Windows caches icons. |
| macOS | Terminal pops open; generic Go icon | `cd desktop && make app` → `Tailsend.app` (Dock icon from `desktop/build/appicon.png`) |
| Linux | Terminal if you launched from a shell; generic window icon | Copy `Tailsend.desktop` to `~/.local/share/applications/` and `build/appicon.png` to `~/.local/share/icons/hicolor/512x512/apps/tailsend.png`. `Terminal=false`. |

Windows release-style build:

```powershell
cd desktop
go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .
```

macOS:

```bash
cd desktop
make app          # or: sh ./pack-macos.sh
open Tailsend.app
```

The window/taskbar/Dock icon is `desktop/build/appicon.png` (also used in
About on macOS and as the GTK icon on Linux). On Windows it is compiled into
`Tailsend.exe` via `rsrc_windows_amd64.syso` / `rsrc_windows_arm64.syso`.

Signed `.dmg` / `.msi` / `.deb` GitHub Releases are **not** in Phase 1.
`wails build` can produce platform packages later; it expects
`desktop/build/appicon.png`.

## After a successful send

The GUI shows where the file should land:

| Target OS | Hint |
|---|---|
| Linux | Inbox; run `tailsend recv .` or `sudo tailscale file get .` |
| Windows | Usually `%USERPROFILE%\Downloads` (Tailscale v1.34+; older: Desktop) |
| macOS | `~/Downloads` |
| iOS / Android | Official Tailscale notification / Files |

## Device list

Peers are sorted by hostname. The GUI refreshes about every 15 seconds (and
right after a send).
