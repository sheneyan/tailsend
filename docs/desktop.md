# Building and using the desktop GUI

The GUI lives in `desktop/` (Wails v2 + Vue). Always pass `-tags production`
(or `-tags dev`). A plain `go build` succeeds but the app exits at runtime with
`will not build without the correct build tags`.

```bash
cd desktop
go build -tags production -o Tailsend .   # Windows: Tailsend.exe
```

`make run` in `desktop/` does the same on macOS/Linux.

## macOS

WebKit is built in. Apple Silicon / recent Xcode may warn that
`setShowsBaselineSeparator:` is deprecated; ignore it. If the **linker** fails
on `UTType`, the repo already links `UniformTypeIdentifiers`
(`desktop/link_darwin.go`).

File **picker** uses AppleScript (`osascript choose file`), not a Wails sheet.
Wails sheets close immediately on click in the WebView. Folders can still be
dropped on the window.

Traffic-light buttons: the title bar is hidden-inset; the header leaves space
on the left.

## Windows 11

Needs Go (CGO), `gcc`, and **Evergreen** WebView2 (Win11 usually already has
it). Do **not** install `Microsoft.WebView2.FixedVersionRuntime.*.cab` — that
cab is an extract-only runtime, not a right-click installer.

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

Default `proxy.golang.org` often times out (`dial tcp 142.250.x.x:443`).

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
cd D:\tailsend\desktop
go build -tags production -o Tailsend.exe .
.\Tailsend.exe
```

Fallback: `go env -w GOPROXY=https://goproxy.io,direct`.

### Drag-and-drop

Explorer drops must use Wails `OnFileDrop` (real `D:\...` paths). If you only
see a corner hint like “drop here to transfer files” and the tray stays empty,
that was WebView2 eating the drop — current builds disable that. After `git
pull`, rebuild.

Click-to-pick uses the native Windows dialog (not AppleScript).

## Linux (Ubuntu 24 + XFCE is fine)

Needs GTK3 + WebKit **dev packages**, not GNOME. XFCE is fine.

Ubuntu 24 ships **webkit2gtk-4.1**, while Wails v2 defaults to 4.0. Install 4.1
and add the `webkit2_41` tag:

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 && echo ok
```

Ubuntu’s `apt install golang-go` is too old (often 1.22, sometimes older). This
repo needs **Go 1.26+** because `tailscale.com` v1.102 requires `go 1.26.6`.
Do **not** use `sudo go build`. Install the official tarball:

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

```bash
cd tailsend/desktop
go build -tags production,webkit2_41 -o Tailsend .
./Tailsend
```

If `apt` has no 4.1 package, install `libwebkit2gtk-4.0-dev` and drop `,webkit2_41`.

**Inbox:** Linux does not auto-save to Downloads. Send a file to this machine,
then **Inbox** in the GUI, or `tailsend recv .` / `sudo tailscale file get .`.
`tailscaled` often runs as root, so recv may need `sudo`.

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
