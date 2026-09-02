//go:build windows

package main

import (
	_ "embed"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmSetIcon   = 0x0080
	iconSmall   = 0
	iconBig     = 1
	gclpHicon   = -14
	gclpHiconSm = -34
	gaRoot      = 2
)

func init() {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	setID := shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	s, err := windows.UTF16PtrFromString("com.sheneyan.tailsend")
	if err == nil {
		setID.Call(uintptr(unsafe.Pointer(s)))
	}
}

func scheduleWindowsWindowChrome() {
	go func() {
		var small, big uintptr
		for i := 0; i < 80; i++ {
			time.Sleep(100 * time.Millisecond)
			if small == 0 {
				small, big = extractExeIcons()
				if small == 0 {
					continue
				}
			}
			applyWindowsIcon(small, big)
		}
	}()
}

func extractExeIcons() (small, big uintptr) {
	exe, err := os.Executable()
	if err != nil {
		return 0, 0
	}
	p, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, 0
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	extract := shell32.NewProc("ExtractIconExW")
	var large, sm uintptr
	extract.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&sm)), 1)
	if sm == 0 {
		sm = large
	}
	if large == 0 {
		large = sm
	}
	return sm, large
}

func applyWindowsIcon(hiconSmall, hiconBig uintptr) {
	if hiconSmall == 0 {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMessage := user32.NewProc("SendMessageW")
	setClassLongPtr := user32.NewProc("SetClassLongPtrW")
	getAncestor := user32.NewProc("GetAncestor")

	forEachWailsWindow(func(hwnd windows.HWND) {
		root := hwnd
		r, _, _ := getAncestor.Call(uintptr(hwnd), gaRoot)
		if r != 0 {
			root = windows.HWND(r)
		}
		sendMessage.Call(uintptr(root), wmSetIcon, iconBig, hiconBig)
		sendMessage.Call(uintptr(root), wmSetIcon, iconSmall, hiconSmall)
		setClassLongPtr.Call(uintptr(root), uintptr(int32(gclpHicon)), hiconBig)
		setClassLongPtr.Call(uintptr(root), uintptr(int32(gclpHiconSm)), hiconSmall)
	})
}

func forEachWailsWindow(fn func(windows.HWND)) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	getClass := user32.NewProc("GetClassNameW")

	pid := uint32(os.Getpid())
	cb := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var wpid uint32
		getPID.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wpid)))
		if wpid != pid {
			return 1
		}
		var buf [256]uint16
		getClass.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
		if windows.UTF16ToString(buf[:]) != "wailsWindow" {
			return 1
		}
		fn(hwnd)
		return 1
	})
	enumWindows.Call(cb, 0)
}
