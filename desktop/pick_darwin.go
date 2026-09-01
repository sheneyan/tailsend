//go:build darwin

package main

import (
	"context"
	"os/exec"
	"strings"
)

// Wails presents NSOpenPanel as a sheet on the WebView window. On recent
// macOS that sheet is cancelled immediately (white flash). A standalone
// AppleScript chooser is application-modal and stays up.

func pickFilesNative(_ context.Context) ([]string, error) {
	script := `try
  set theFiles to choose file with prompt "Send with Tailsend" with multiple selections allowed
  set posixPaths to {}
  repeat with f in theFiles
    set end of posixPaths to POSIX path of f
  end repeat
  set AppleScript's text item delimiters to linefeed
  return posixPaths as text
on error number -128
  return ""
end try`
	out, err := osascript(script)
	if err != nil {
		return nil, err
	}
	return splitPOSIXLines(out), nil
}

func pickDirNative(_ context.Context) (string, error) {
	script := `try
  return POSIX path of (choose folder with prompt "Save received files to")
on error number -128
  return ""
end try`
	out, err := osascript(script)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func osascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	b, err := cmd.Output()
	if err != nil {
		// Cancelled choosers sometimes still exit 1 even with the try block.
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			msg := string(ee.Stderr)
			if strings.Contains(msg, "User canceled") || strings.Contains(msg, "-128") {
				return "", nil
			}
		}
		if _, ok := err.(*exec.ExitError); ok {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}
