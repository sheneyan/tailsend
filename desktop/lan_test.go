package main

import "testing"

func TestOutboundBusy(t *testing.T) {
	a := NewApp(nil)
	if !a.beginSend() {
		t.Fatal("first send should start")
	}
	if a.beginSend() {
		t.Fatal("second outbound send must wait")
	}
	a.endSend()
	if !a.beginSend() {
		t.Fatal("after end")
	}
}
