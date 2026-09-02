//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Hide the extra console that Windows attaches to a `go build` GUI binary
// when the user double-clicks it. Leave the console alone if we were
// launched from an existing terminal (pid of the console owner is not us).
func init() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	getWindowThreadProcessId := user32.NewProc("GetWindowThreadProcessId")
	showWindow := user32.NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	var pid uint32
	getWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != uint32(os.Getpid()) {
		return
	}
	showWindow.Call(hwnd, uintptr(windows.SW_HIDE))
}
