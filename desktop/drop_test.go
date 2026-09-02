package main

import "testing"

func TestWebViewDropDisabled(t *testing.T) {
	if !webViewDropDisabled() {
		t.Fatal("WebView drop must stay disabled; Windows uses OLE IDropTarget, Linux uses GTK dest")
	}
}
