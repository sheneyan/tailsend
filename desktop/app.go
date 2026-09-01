package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sheneyan/tailsend/internal/tsdrop"
)

// Snapshot is one UI poll of daemon + peers + inbox.
type Snapshot struct {
	State   string               `json:"state"`
	Self    tsdrop.Self          `json:"self"`
	Targets []tsdrop.Target      `json:"targets"`
	Inbox   []tsdrop.WaitingFile `json:"inbox"`
	Error   string               `json:"error,omitempty"`
}

// App is the Wails backend.
type App struct {
	ctx    context.Context
	client *tsdrop.Client
}

func NewApp(c *tsdrop.Client) *App {
	if c == nil {
		c = tsdrop.New(nil)
	}
	return &App{client: c}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		runtime.EventsEmit(ctx, "files-dropped", paths)
	})
}

func (a *App) emit(name string, data ...interface{}) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data...)
}

// Snapshot reports Tailscale state, sendable peers, and inbox.
func (a *App) Snapshot() Snapshot {
	ctx := a.requestCtx()
	st, self, err := a.client.Probe(ctx)
	if err != nil {
		return Snapshot{State: tsdrop.Unknown.String(), Error: err.Error()}
	}
	snap := Snapshot{State: st.String(), Self: self}
	if st != tsdrop.Running {
		return snap
	}
	targets, err := a.client.Targets(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	if targets == nil {
		targets = []tsdrop.Target{}
	}
	snap.Targets = targets
	inbox, err := a.client.Inbox(ctx)
	if err == nil {
		if inbox == nil {
			inbox = []tsdrop.WaitingFile{}
		}
		snap.Inbox = inbox
	}
	return snap
}

func (a *App) requestCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// SendTo pushes the given local paths to a peer StableID.
func (a *App) SendTo(stableID string, paths []string) error {
	if stableID == "" {
		return fmt.Errorf("no device selected")
	}
	if len(paths) == 0 {
		return fmt.Errorf("no files selected")
	}
	items := make([]tsdrop.SendItem, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		items = append(items, tsdrop.SendItem{Path: p})
	}
	return a.client.Send(a.requestCtx(), stableID, items, func(p tsdrop.Progress) {
		a.emit("progress", p)
	})
}

// SelectFiles opens a native multi-file picker.
func (a *App) SelectFiles() ([]string, error) {
	return pickFilesNative(a.ctx)
}

// SelectRecvDir opens a native directory picker for inbox drain.
func (a *App) SelectRecvDir() (string, error) {
	return pickDirNative(a.ctx)
}

// DefaultRecvDir is ~/Downloads when it exists, otherwise the home directory.
func (a *App) DefaultRecvDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	dl := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dl); err == nil && st.IsDir() {
		return dl
	}
	return home
}

// ReceiveTo copies inbox files into dir using skip|overwrite|rename.
func (a *App) ReceiveTo(dir, conflict string) ([]string, error) {
	if dir == "" {
		dir = a.DefaultRecvDir()
	}
	c, err := tsdrop.ParseConflict(conflict)
	if err != nil {
		return nil, err
	}
	return a.client.Receive(a.requestCtx(), dir, c)
}

// PairingJSON is the sendable-target list for a phone to import.
func (a *App) PairingJSON() (string, error) {
	b, err := a.client.ExportTargetsJSON(a.requestCtx())
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// PairingQR returns a PNG data URL of PairingJSON.
func (a *App) PairingQR() (string, error) {
	payload, err := a.PairingJSON()
	if err != nil {
		return "", err
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// Platform is the GUI OS, for inbox-hint copy.
func (a *App) Platform() string {
	return goruntime.GOOS
}
