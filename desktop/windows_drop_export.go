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
	emitDroppedPaths(splitPOSIXLines(C.GoString(cpaths)))
}
