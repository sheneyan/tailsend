package main

import (
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var guiApp *App

// Wails v2.15: DisableWebViewDrop calls AllowExternalDrag(false) on Windows,
// which blocks Explorer file drops entirely (EnableFileDrop never fires).
// Linux/macOS still need it so the WebView does not navigate to the file.
func webViewDropDisabled() bool {
	return goruntime.GOOS != "windows"
}

func emitDroppedPaths(paths []string) {
	if guiApp == nil || guiApp.ctx == nil || len(paths) == 0 {
		return
	}
	runtime.EventsEmit(guiApp.ctx, "files-dropped", paths)
}
