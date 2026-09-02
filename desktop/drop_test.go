package main

import "testing"

func TestWebViewDropDisabled(t *testing.T) {
	if !webViewDropDisabled() {
		t.Fatal("WebView drop must stay disabled; Windows uses OLE IDropTarget, Linux uses GTK dest")
	}
}

func TestTakePendingDropped(t *testing.T) {
	_ = takePendingDropped()
	emitDroppedPaths([]string{`D:\a.go`, `D:\b.exe`})
	got := takePendingDropped()
	if len(got) != 2 || got[0] != `D:\a.go` || got[1] != `D:\b.exe` {
		t.Fatalf("got %v", got)
	}
	if len(takePendingDropped()) != 0 {
		t.Fatal("queue should be empty after take")
	}
}
