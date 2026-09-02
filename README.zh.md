# Tailsend

用 [LocalSend](https://localsend.org) 的用法，走 **Tailscale Taildrop** 传文件。

当前是 CLI + 桌面 GUI（Android / iOS 在后续阶段）。它挂在本机已经在跑的官方
Tailscale 上，**不会自己再创建一个 Tailscale 节点，也没有独立登录**。daemon
没起来时，会提示你去打开 Tailscale 应用。

```bash
tailsend list
tailsend send ./photo.jpg pixel:
```

[English](README.md) · [架构](docs/architecture.md) · [Taildrop 限制](docs/taildrop.md) · [桌面 GUI 编译](docs/desktop.md)

> **非官方项目。** 与 Tailscale、LocalSend 无关。Taildrop 是 Tailscale 的功能
> （目前仍是 alpha）。Tailsend 只是它的客户端。

## 现状

| 表面 | 状态 |
|---|---|
| macOS / Linux / Windows CLI | **Phase 0，可用** |
| 桌面 GUI（Wails） | **Phase 1，可用** |
| Android / iOS 发送端 | 计划中 |

需要先安装并登录 [Tailscale](https://tailscale.com/download)。

## 前提

1. 要互传的设备都安装并登录 Tailscale。
2. 在 [管理后台](https://login.tailscale.com/admin/settings/general) 打开
   **Send Files**。Taildrop 需要每个 tailnet 单独 opt-in。
3. 默认只能在**同一用户、未打 tag** 的设备之间传。用 auth key 加入的节点通常
   带 tag，需要额外 ACL grant。详见 [docs/taildrop.md](docs/taildrop.md)。

没有 `tailsend login`。

## 安装

### CLI（多平台）

CLI **不需要 CGO**。`make cli-all` 会在 `dist/` 下编出六个
`tailsend-cli-<系统>-<架构>`（macOS / Linux / Windows × amd64 / arm64）。
要挂到 GitHub Release：把 `scripts/github-release-cli.yml` 拷到
`.github/workflows/`，再推 `v*` 标签（第一次需要
`gh auth refresh -s workflow`）。改名为 `tailsend`（Windows 是 `tailsend.exe`）
放到 `PATH` 即可。

```bash
# Apple Silicon
chmod +x tailsend-cli-darwin-arm64 && mv tailsend-cli-darwin-arm64 /usr/local/bin/tailsend

# Linux x86_64
chmod +x tailsend-cli-linux-amd64 && sudo mv tailsend-cli-linux-amd64 /usr/local/bin/tailsend
```

源码或 `go install`（需要 **Go 1.26+**，不要用 apt 的 `golang-go`）：

```bash
go install github.com/sheneyan/tailsend/cmd/tailsend@latest

git clone https://github.com/sheneyan/tailsend.git
cd tailsend
make cli        # 当前这台机器
make cli-all    # dist/ 下六个平台的 tailsend-cli-*
```

GUI 是另一份带 CGO 的程序，**不能**交叉编译，要在对应系统上打包：

```bash
cd desktop
make dmg           # macOS：Tailsend.app + dist-release/Tailsend.dmg
sh ./pack-linux.sh
# Windows: powershell -ExecutionPolicy Bypass -File .\pack-windows.ps1
```

见 [docs/desktop.md](docs/desktop.md) 的 **Release packages**。公证 / Authenticode：
[docs/signing.md](docs/signing.md)。

### 桌面 GUI

必须加 `-tags production`，否则一点击运行会报
`will not build without the correct build tags`。Ubuntu 24 还要加
`webkit2_41`（系统是 webkit2gtk 4.1，Wails 默认 4.0）。**不要** `sudo go build`，
**不要**用 `apt install golang-go`。

```bash
git clone https://github.com/sheneyan/tailsend.git
cd tailsend/desktop
# macOS / Windows:
go build -tags production -o Tailsend .          # Windows 用 Tailsend.exe
# Ubuntu 24:
# go build -tags production,webkit2_41 -o Tailsend .
./Tailsend
```

编译注意事项（官方 Go、PATH、`~` 包名、WebView2、GTK、报错对照表）见
**[docs/desktop.md](docs/desktop.md)**。

窗口里是设备名网格。先加文件（点击或拖进窗口），再点设备。发完后会提示对端
文件大概在哪（Windows 下载目录 / Linux 的 `tailsend recv .`）。**Inbox** 取
daemon 收件箱。GUI 的 **Pair phone** 和命令行 `tailsend pair [--qr]` 是同一份
配对 JSON（给后续手机 App）。

双击 GUI 若弹出控制台、图标是默认 Go 图标：见
[docs/desktop.md](docs/desktop.md) 的 Packaging。macOS 用 `cd desktop && make app`。

## 用法

```
tailsend status
tailsend list                          # 设备名，不用填 IP
tailsend list --json                   # 可发送目标的 JSON
tailsend pair [--qr]                   # 和 GUI Pair phone 同一份 JSON；--qr 画终端二维码
tailsend send <file-or-dir...> <device>:
tailsend inbox                         # Linux 等 inbox 模式的待取文件
tailsend recv [dir] [--watch] [--conflict=skip|overwrite|rename]
```

`send` 的最后一个参数必须以 `:` 结尾（和 `scp`、`tailscale file cp` 一样）：

```bash
tailsend send README.md pixel:
tailsend send ./docs ./notes.txt macbook:
```

- 设备名来自 `tailsend list`（主机名、MagicDNS 短名或 Tailscale IP）。
- 目录会现场打成 zip（`docs/` → `docs.zip`）。
- `recv` 默认 `--conflict=rename`（`note.txt` → `note-1.txt`）。

### 文件落到哪里

| 接收端 | 行为 |
|---|---|
| macOS | 一般是 `~/Downloads` |
| Windows | 一般是「下载」（Tailscale v1.34+） |
| Linux | daemon **收件箱**，要用 `tailsend recv .` 取出来 |
| iOS / Android | 官方 Tailscale 应用通知 / 文件 |

Linux 上 `tailscaled` 常以 root 运行，收件箱文件也归 root，可能需要
`sudo tailsend recv .`。

## 和 LocalSend 的差别

| | LocalSend | Tailsend |
|---|---|---|
| 网络 | 局域网 mDNS | 你的 tailnet（WireGuard / DERP） |
| 账号 | 无 | 复用 Tailscale |
| 跨互联网 | 否 | 只要 Tailscale 能打到对端 |
| 发现设备 | 附近设备 | `tailsend list` 读 netmap |
| 文件夹 | 原生目录树 | 先 zip 再发 |
| 接收确认 | 应用内 | 跟官方 Taildrop 一样 |

发出去的文件和系统里「分享 → Tailscale」或 `tailscale file cp` 是同一条管道。

## 排障

| 现象 | 常见原因 |
|---|---|
| `tailscale daemon is not running` | 先打开 / 启动 Tailscale |
| `file sharing is not enabled` | 管理后台打开 Send Files |
| `owned by a different user` | Taildrop 默认只给同一用户 |
| 列表里没有，或 not a Taildrop target | 打了 tag、离线、或没有 PeerAPI |
| `destination must end with ':'` | 写成 `pixel:` 而不是 `pixel` |
| Linux `recv` 权限不够 | 用和 `tailscaled` 相同的用户（经常是 sudo） |
| GUI：`Package webkit2gtk-4.0 was not found` | Ubuntu 24 用 `-tags production,webkit2_41`，见 [docs/desktop.md](docs/desktop.md) |
| `invalid go version … must match format 1.23` | 还在用 apt 的 Go；装 https://go.dev/dl，保证 `which go` 是 `/usr/local/go/bin/go` |
| `Unable to locate package libwebkit2gtk-4.1-dev~` | 包名不要带 `~` |

退出码：`0` 成功，`2` daemon/登录，`3` 策略/目标，`4` 读写或其他错误。

## 开发

```bash
go test ./...
go build -o tailsend ./cmd/tailsend
```

单测用假 LocalAPI，不需要真 tailnet。贡献说明见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可

[MIT](LICENSE)。依赖官方 Go LocalAPI 客户端
[`tailscale.com`](https://github.com/tailscale/tailscale)（BSD-3-Clause）。
