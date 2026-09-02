.PHONY: build run app dmg winres

# Linux on Ubuntu 24: make TAGS=production,webkit2_41
TAGS ?= production

# Windows (PowerShell / cmd): hide the extra console window:
#   go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .

build:
	go build -tags $(TAGS) -o Tailsend .

run: build
	./Tailsend

# macOS .app + unsigned .dmg → dist-release/Tailsend.dmg
app dmg:
	sh ./pack-macos.sh

# existing .app only → dmg
dmg-only:
	SKIP_BUILD=1 sh ./pack-macos.sh

# Windows exe icon: rsrc_windows_amd64.syso / rsrc_windows_arm64.syso
winres:
	sh ./gen-winres.sh
