.PHONY: build run app

# Linux on Ubuntu 24: make TAGS=production,webkit2_41
TAGS ?= production

# Windows (PowerShell / cmd): hide the extra console window:
#   go build -tags production -ldflags "-H windowsgui" -o Tailsend.exe .

build:
	go build -tags $(TAGS) -o Tailsend .

run: build
	./Tailsend

# macOS .app (Dock icon, no Terminal on double-click)
app:
	sh ./pack-macos.sh
