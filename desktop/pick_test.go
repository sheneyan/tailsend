package main

import (
	"reflect"
	"testing"
)

func TestSplitPOSIXLines(t *testing.T) {
	got := splitPOSIXLines("/Users/me/a.txt\n/Users/me/b.txt\n")
	want := []string{"/Users/me/a.txt", "/Users/me/b.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if splitPOSIXLines("") != nil && len(splitPOSIXLines("")) != 0 {
		t.Fatalf("empty should be empty, got %v", splitPOSIXLines(""))
	}
}
