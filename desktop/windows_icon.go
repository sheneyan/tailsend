//go:build windows

package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed build/appicon.ico
var appIconICO []byte

const (
	imageIcon      = 1
	lrLoadFromFile = 0x0010
	lrDefaultSize  = 0x0040
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	gclpHicon      = -14
	gclpHiconSm    = -34
)

func scheduleWindowsWindowChrome() {
	go func() {
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if applyWindowsIcon() {
				return
			}
		}
	}()
}

func applyWindowsIcon() bool {
	hwnd := findTailsendHWND()
	if hwnd == 0 {
		return false
	}
	icoPath := filepath.Join(os.TempDir(), "tailsend-appicon.ico")
	if err := os.WriteFile(icoPath, appIconICO, 0o644); err != nil {
		return false
	}
	p, err := windows.UTF16PtrFromString(icoPath)
	if err != nil {
		return false
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	loadImage := user32.NewProc("LoadImageW")
	sendMessage := user32.NewProc("SendMessageW")
	setClassLongPtr := user32.NewProc("SetClassLongPtrW")

	hicon, _, _ := loadImage.Call(0, uintptr(unsafe.Pointer(p)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
	if hicon == 0 {
		hicon, _, _ = loadImage.Call(0, uintptr(unsafe.Pointer(p)), imageIcon, 32, 32, lrLoadFromFile)
	}
	if hicon == 0 {
		return false
	}
	sendMessage.Call(uintptr(hwnd), wmSetIcon, iconBig, hicon)
	sendMessage.Call(uintptr(hwnd), wmSetIcon, iconSmall, hicon)
	setClassLongPtr.Call(uintptr(hwnd), uintptr(int32(gclpHicon)), hicon)
	setClassLongPtr.Call(uintptr(hwnd), uintptr(int32(gclpHiconSm)), hicon)
	return true
}

func findTailsendHWND() windows.HWND {
	user32 := windows.NewLazySystemDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	isVisible := user32.NewProc("IsWindowVisible")
	getText := user32.NewProc("GetWindowTextW")

	pid := uint32(os.Getpid())
	var found windows.HWND
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var wpid uint32
		getPID.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wpid)))
		if wpid != pid {
			return 1
		}
		vis, _, _ := isVisible.Call(uintptr(hwnd))
		if vis == 0 {
			return 1
		}
		var buf [256]uint16
		getText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
		if windows.UTF16ToString(buf[:]) == "Tailsend" {
			found = hwnd
			return 0
		}
		return 1
	})
	enumWindows.Call(cb, 0)
	return found
}
