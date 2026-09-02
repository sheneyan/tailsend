//go:build windows

package main

// C drop-target bodies live in windows_drop.go. //export cannot share
// a file with C function definitions.

/*
#include <stdlib.h>
*/
import "C"

//export goWinDropped
func goWinDropped(cpaths *C.char) {
	// COM Drop runs on the Windows UI thread, not a Go thread.
	paths := append([]string(nil), splitPOSIXLines(C.GoString(cpaths))...)
	go emitDroppedPaths(paths)
}
