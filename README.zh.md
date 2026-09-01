# Tailsend

用 [LocalSend](https://localsend.org) 的用法，走 **Tailscale Taildrop** 传文件。

当前是 CLI + 桌面 GUI（Android / iOS 在后续阶段）。它挂在本机已经在跑的官方
Tailscale 上，**不会自己再创建一个 Tailscale 节点，也没有独立登录**。daemon
没起来时，会提示你去打开 Tailscale 应用。

```bash
tailsend list
tailsend send ./photo.jpg pixel:
```

[English](README.md) · [架构](docs/architecture.md) · [Taildrop 限制](docs/taildrop.md)

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

需要 Go 1.24+（开发环境是 1.27）。

```bash
go install github.com/sheneyan/tailsend/cmd/tailsend@latest
```

从源码：

```bash
git clone https://github.com/sheneyan/tailsend.git
cd tailsend
go build -o tailsend ./cmd/tailsend
```

Windows：`go build -o tailsend.exe ./cmd/tailsend`。需要的话把二进制放到 `PATH`。

### 桌面 GUI

需要开启 CGO 的 Go，以及系统 WebView（macOS 自带；Windows 要 WebView2）。

```bash
git clone https://github.com/sheneyan/tailsend.git
cd tailsend/desktop
go build -tags production -o Tailsend .
./Tailsend
```

如果链接阶段报 `UTType`，在同一目录再编一次即可（仓库里已加 Darwin 的 framework 链接）。
```

必须加 `-tags production`（开发可用 `-tags dev`），否则运行会报
`will not build without the correct build tags`。

改界面：`cd frontend && npm install && npm run build`。打成 `.app` 再用 [Wails v2](https://wails.io) 在 `desktop/` 里跑 `wails build`。

窗口里是设备名网格（不用填 IP）。拖文件或点选，再点设备发送。**Pair phone**
给出配对 QR；**Inbox** 把 Linux 式收件箱存到下载目录。

## 用法

```
tailsend status
tailsend list                          # 设备名，不用填 IP
tailsend list --json                   # 可发送目标的 JSON
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
