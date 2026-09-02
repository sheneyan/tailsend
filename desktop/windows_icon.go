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
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	gclpHicon      = -14
	gclpHiconSm    = -34
	gaRoot         = 2
)

func init() {
	// Before any window is created so the taskbar does not pin the default Go icon.
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	setID := shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	s, err := windows.UTF16PtrFromString("com.sheneyan.tailsend")
	if err == nil {
		setID.Call(uintptr(unsafe.Pointer(s)))
	}
}

func scheduleWindowsWindowChrome() {
	go func() {
		var hiconSmall, hiconBig uintptr
		for i := 0; i < 80; i++ {
			time.Sleep(100 * time.Millisecond)
			if hiconSmall == 0 {
				hiconSmall, hiconBig = loadAppIcons()
				if hiconSmall == 0 {
					continue
				}
			}
			applyWindowsIcon(hiconSmall, hiconBig)
		}
	}()
}

func loadAppIcons() (small, big uintptr) {
	icoPath := filepath.Join(os.TempDir(), "tailsend-appicon.ico")
	if err := os.WriteFile(icoPath, appIconICO, 0o644); err != nil {
		return 0, 0
	}
	p, err := windows.UTF16PtrFromString(icoPath)
	if err != nil {
		return 0, 0
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	loadImage := user32.NewProc("LoadImageW")
	small, _, _ = loadImage.Call(0, uintptr(unsafe.Pointer(p)), imageIcon, 16, 16, lrLoadFromFile)
	big, _, _ = loadImage.Call(0, uintptr(unsafe.Pointer(p)), imageIcon, 32, 32, lrLoadFromFile)
	if big == 0 {
		big = small
	}
	if small == 0 {
		small = big
	}
	return small, big
}

func applyWindowsIcon(hiconSmall, hiconBig uintptr) {
	if hiconSmall == 0 {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMessage := user32.NewProc("SendMessageW")
	setClassLongPtr := user32.NewProc("SetClassLongPtrW")
	getAncestor := user32.NewProc("GetAncestor")

	forEachProcessWindow(func(hwnd windows.HWND) {
		root := hwnd
		r, _, _ := getAncestor.Call(uintptr(hwnd), gaRoot)
		if r != 0 {
			root = windows.HWND(r)
		}
		sendMessage.Call(uintptr(root), wmSetIcon, iconBig, hiconBig)
		sendMessage.Call(uintptr(root), wmSetIcon, iconSmall, hiconSmall)
		sendMessage.Call(uintptr(hwnd), wmSetIcon, iconBig, hiconBig)
		sendMessage.Call(uintptr(hwnd), wmSetIcon, iconSmall, hiconSmall)
		setClassLongPtr.Call(uintptr(root), uintptr(int32(gclpHicon)), hiconBig)
		setClassLongPtr.Call(uintptr(root), uintptr(int32(gclpHiconSm)), hiconSmall)
	})
}

func findTailsendHWND() windows.HWND {
	var found windows.HWND
	forEachProcessWindow(func(hwnd windows.HWND) {
		if found == 0 {
			found = hwnd
		}
	})
	return found
}

func forEachProcessWindow(fn func(windows.HWND)) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	isVisible := user32.NewProc("IsWindowVisible")

	pid := uint32(os.Getpid())
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
		fn(hwnd)
		return 1
	})
	enumWindows.Call(cb, 0)
}
