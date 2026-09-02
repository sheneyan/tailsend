package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

var guiApp *App

func emitDroppedPaths(paths []string) {
	if guiApp == nil || guiApp.ctx == nil || len(paths) == 0 {
		return
	}
	runtime.EventsEmit(guiApp.ctx, "files-dropped", paths)
}
