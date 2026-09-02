package main

import (
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var guiApp *App

var (
	dropMu         sync.Mutex
	pendingDropped []string
)

// Disable the WebView's own drop target on every OS. Linux/macOS would
// otherwise open the file as a page. Windows WebView2 would steal OLE
// drops; native IDropTarget on nested Chrome child windows handles Explorer.
func webViewDropDisabled() bool {
	return true
}

func emitDroppedPaths(paths []string) {
	if len(paths) == 0 {
		return
	}
	dropMu.Lock()
	pendingDropped = append(pendingDropped, paths...)
	dropMu.Unlock()
	if guiApp == nil || guiApp.ctx == nil {
		return
	}
	runtime.EventsEmit(guiApp.ctx, "files-dropped", paths)
}

func takePendingDropped() []string {
	dropMu.Lock()
	defer dropMu.Unlock()
	out := pendingDropped
	pendingDropped = nil
	if out == nil {
		return []string{}
	}
	return out
}
