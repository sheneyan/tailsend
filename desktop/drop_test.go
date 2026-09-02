package main

import "testing"

func TestWebViewDropDisabled(t *testing.T) {
	if !webViewDropDisabled() {
		t.Fatal("WebView drop must stay disabled so dropped files are not opened as a page")
	}
}
