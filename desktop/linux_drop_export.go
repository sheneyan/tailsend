//go:build linux

package main

// C function bodies live in linux_drop.go. A file with //export must not
// define C functions: cgo copies them into _cgo_export.c and the linker
// then sees two copies of tailsendScheduleLinuxDrop.

/*
#include <stdlib.h>
*/
import "C"

//export goEmitDropped
func goEmitDropped(cpaths *C.char) {
	emitDroppedPaths(splitPOSIXLines(C.GoString(cpaths)))
}
