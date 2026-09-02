package main

import (
	"runtime"
	"testing"
)

func TestWebViewDropDisabled(t *testing.T) {
	got := webViewDropDisabled()
	if runtime.GOOS == "windows" && got {
		t.Fatal("Windows must leave WebView2 AllowExternalDrag on so Explorer drops reach OnFileDrop")
	}
	if runtime.GOOS != "windows" && !got {
		t.Fatal("non-Windows must disable WebView drop so dropped files are not opened as a page")
	}
}
