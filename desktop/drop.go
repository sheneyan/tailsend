package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

var guiApp *App

// Disable the WebView's own drop dest so a drop does not open the file as a
// page. Linux re-installs a GTK dest. Windows has no Explorer drop in v1
// (click-to-pick only; see TODO.md).
func webViewDropDisabled() bool {
	return true
}

func emitDroppedPaths(paths []string) {
	if guiApp == nil || guiApp.ctx == nil || len(paths) == 0 {
		return
	}
	runtime.EventsEmit(guiApp.ctx, "files-dropped", paths)
}
