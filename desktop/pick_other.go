//go:build !darwin

package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func pickFilesNative(ctx context.Context) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("app not started")
	}
	return runtime.OpenMultipleFilesDialog(ctx, runtime.OpenDialogOptions{
		Title:           "Send with Tailsend",
		ResolvesAliases: true,
	})
}

func pickDirNative(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("app not started")
	}
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title:                "Save received files to",
		CanCreateDirectories: true,
	})
}
