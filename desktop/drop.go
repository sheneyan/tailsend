package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var guiApp *App

// Disable the WebView's own drop target on every OS. Linux/macOS would
// otherwise open the file as a page. Windows WebView2 would steal OLE
// drops; native IDropTarget on the Chrome child windows handles Explorer.
func webViewDropDisabled() bool {
	return true
}

func emitDroppedPaths(paths []string) {
	if guiApp == nil || guiApp.ctx == nil || len(paths) == 0 {
		return
	}
	runtime.EventsEmit(guiApp.ctx, "files-dropped", paths)
}
