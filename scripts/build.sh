#!/bin/sh
# One command to remember. GUI cannot be cross-compiled (CGO / WebView).
# Usage: ./scripts/build.sh [help|cli|cli-all|gui|android|android-install|test|all]
set -e
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"

usage() {
	cat <<'EOF'
Tailsend 编译（只记这一条）

  ./scripts/build.sh              打印本说明
  ./scripts/build.sh cli          本机 CLI → ./tailsend
  ./scripts/build.sh cli-all      六平台 CLI → dist/tailsend-cli-*
  ./scripts/build.sh gui          本机 GUI 包（mac dmg / linux tar.gz / win exe）
  ./scripts/build.sh android      Debug APK
  ./scripts/build.sh android-install  装到已连接的真机并启动
  ./scripts/build.sh test         Go 单测（含 desktop）
  ./scripts/build.sh all          本机能编的都编

Windows PowerShell（仓库根目录）:

  powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
  powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 gui

底层命令（忘了脚本时）
  CLI     make cli          |  make cli-all
  mac GUI cd desktop && make dmg
  Linux   cd desktop && sh ./pack-linux.sh
  Win GUI cd desktop; powershell -ExecutionPolicy Bypass -File .\pack-windows.ps1
  Android ANDROID_HOME=... ./android/gradlew -p android :app:assembleDebug
          真机: 同上 :app:installDebug
  测试    go test ./... && go test ./desktop

GUI 必须 -tags production。Ubuntu 24 再加 webkit2_41。不要 sudo go build。
EOF
}

need_go() {
	if ! command -v go >/dev/null 2>&1; then
		echo "go not on PATH (need 1.26+; not apt golang-go)" >&2
		exit 1
	fi
}

cmd_cli() {
	need_go
	echo "→ CLI (this machine)"
	make cli
	echo "wrote $ROOT/tailsend"
}

cmd_cli_all() {
	need_go
	echo "→ CLI all platforms"
	make cli-all
}

cmd_gui() {
	need_go
	sys=$(uname -s)
	case "$sys" in
	Darwin)
		echo "→ macOS GUI (.app + .dmg)"
		make -C "$ROOT/desktop" dmg
		echo "open: open $ROOT/desktop/Tailsend.app"
		;;
	Linux)
		echo "→ Linux GUI tarball"
		sh "$ROOT/desktop/pack-linux.sh"
		;;
	MINGW*|MSYS*|CYGWIN*|Windows_NT)
		echo "→ Windows GUI"
		powershell.exe -ExecutionPolicy Bypass -File "$ROOT/desktop/pack-windows.ps1"
		;;
	*)
		echo "no GUI pack script for $sys (build on that OS)" >&2
		exit 1
		;;
	esac
}

android_home() {
	if [ -n "${ANDROID_HOME:-}" ]; then
		echo "$ANDROID_HOME"
		return
	fi
	if [ -n "${ANDROID_SDK_ROOT:-}" ]; then
		echo "$ANDROID_SDK_ROOT"
		return
	fi
	props="$ROOT/android/local.properties"
	if [ -f "$props" ]; then
		sed -n 's/^sdk.dir=//p' "$props" | tr -d '\r' | tail -n 1
	fi
}

cmd_android() {
	ah=$(android_home)
	if [ -z "$ah" ]; then
		echo "set ANDROID_HOME or android/local.properties sdk.dir" >&2
		exit 1
	fi
	export ANDROID_HOME="$ah"
	export PATH="$ANDROID_HOME/platform-tools:$PATH"
	echo "→ Android debug APK (ANDROID_HOME=$ANDROID_HOME)"
	"$ROOT/android/gradlew" -p "$ROOT/android" :app:assembleDebug
	echo "apk: $ROOT/android/app/build/outputs/apk/debug/app-debug.apk"
}

cmd_android_install() {
	cmd_android
	export ANDROID_HOME
	export PATH="$ANDROID_HOME/platform-tools:$PATH"
	if ! command -v adb >/dev/null 2>&1; then
		echo "adb not on PATH" >&2
		exit 1
	fi
	"$ROOT/android/gradlew" -p "$ROOT/android" :app:installDebug
	adb shell am start -n com.sheneyan.tailsend/.MainActivity
}

cmd_test() {
	need_go
	echo "→ go test"
	go test ./...
	go test ./desktop
}

cmd_all() {
	cmd_cli_all
	cmd_gui
	if [ -n "$(android_home)" ]; then
		cmd_android
	else
		echo "skip android (no ANDROID_HOME / local.properties)"
	fi
}

target=${1:-help}
case "$target" in
help|-h|--help) usage ;;
cli) cmd_cli ;;
cli-all) cmd_cli_all ;;
gui) cmd_gui ;;
android) cmd_android ;;
android-install) cmd_android_install ;;
test) cmd_test ;;
all) cmd_all ;;
*)
	echo "unknown target: $target" >&2
	usage >&2
	exit 2
	;;
esac
