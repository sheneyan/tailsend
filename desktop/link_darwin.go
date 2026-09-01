//go:build darwin

package main

// Newer macOS SDKs put UTType in UniformTypeIdentifiers.
// Wails v2 references it but does not link the framework.

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"
