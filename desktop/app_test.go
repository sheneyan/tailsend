package main

import (
	"encoding/base64"
	"encoding/binary"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheneyan/tailsend/internal/tsdrop"
	"github.com/sheneyan/tailsend/internal/tsdrop/tsdroptest"

	"tailscale.com/ipn/ipnstate"
)

func phone() *ipnstate.PeerStatus {
	return &ipnstate.PeerStatus{
		ID:             "n-phone",
		HostName:       "pixel",
		DNSName:        "pixel.tailnet.ts.net.",
		OS:             "android",
		Online:         true,
		TailscaleIPs:   []netip.Addr{netip.MustParseAddr("100.64.0.2")},
		PeerAPIURL:     []string{"http://100.64.0.2:41641"},
		TaildropTarget: ipnstate.TaildropTargetAvailable,
	}
}

func TestSnapshotRunning(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self: &ipnstate.PeerStatus{
			HostName:     "macbook",
			DNSName:      "macbook.tailnet.ts.net.",
			OS:           "macOS",
			Online:       true,
			TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		},
		Peers: []*ipnstate.PeerStatus{phone()},
		Inbox: map[string][]byte{"a.txt": []byte("x")},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	snap := app.Snapshot()
	if snap.State != "Running" || snap.Self.Hostname != "macbook" {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Error != "" {
		t.Fatalf("error = %s", snap.Error)
	}
	if len(snap.Targets) != 1 || snap.Targets[0].Hostname != "pixel" {
		t.Fatalf("targets = %+v", snap.Targets)
	}
	if len(snap.Inbox) != 1 || snap.Inbox[0].Name != "a.txt" {
		t.Fatalf("inbox = %+v", snap.Inbox)
	}
}

func TestSnapshotDaemonDown(t *testing.T) {
	d := &tsdroptest.Daemon{DialErr: true}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	snap := app.Snapshot()
	if snap.Error == "" {
		t.Fatal("expected error")
	}
	if !strings.Contains(snap.Error, "not running") && snap.State != "Unknown" {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestSnapshotNeedsLogin(t *testing.T) {
	d := &tsdroptest.Daemon{BackendState: "NeedsLogin"}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	snap := app.Snapshot()
	if snap.State != "NeedsLogin" {
		t.Fatalf("state = %s", snap.State)
	}
}

func TestSnapshotHandlesListPlease(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self: &ipnstate.PeerStatus{
			ID:           "n-self",
			HostName:     "macbook",
			DNSName:      "macbook.tailnet.ts.net.",
			OS:           "macOS",
			Online:       true,
			TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
			PeerAPIURL:   []string{"http://100.64.0.1:41641"},
		},
		Peers: []*ipnstate.PeerStatus{phone()},
		Inbox: map[string][]byte{
			tsdrop.ListPleaseName: []byte(`{"v":1,"from":["100.64.0.2"]}`),
			"note.txt":            []byte("keep"),
		},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	snap := app.Snapshot()
	if snap.Error != "" {
		t.Fatalf("error = %s", snap.Error)
	}
	if len(d.Puts) != 1 || d.Puts[0].Name != tsdrop.PairJSONName || d.Puts[0].Target != "n-phone" {
		t.Fatalf("puts = %+v", d.Puts)
	}
	if len(snap.Inbox) != 1 || snap.Inbox[0].Name != "note.txt" {
		t.Fatalf("inbox = %+v", snap.Inbox)
	}
}

func TestSendTo(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self:         &ipnstate.PeerStatus{HostName: "macbook"},
		Peers:        []*ipnstate.PeerStatus{phone()},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	path := filepath.Join(t.TempDir(), "hi.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.SendTo("n-phone", []string{path}); err != nil {
		t.Fatal(err)
	}
	if len(d.Puts) != 1 || string(d.Puts[0].Body) != "hello" {
		t.Fatalf("puts = %+v", d.Puts)
	}
}

func TestPairingJSONIncludesSelf(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self: &ipnstate.PeerStatus{
			ID:           "n-self",
			HostName:     "macbook",
			PeerAPIURL:   []string{"http://100.64.0.1:41641"},
			Online:       true,
			TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		},
		Peers: []*ipnstate.PeerStatus{phone()},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	js, err := app.PairingJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, `"hostname": "macbook"`) || !strings.Contains(js, `"hostname": "pixel"`) {
		t.Fatalf("json = %s", js)
	}
	if strings.Index(js, "macbook") > strings.Index(js, "pixel") {
		t.Fatalf("self should be first: %s", js)
	}
}

func TestPairingQR(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self:         &ipnstate.PeerStatus{HostName: "macbook"},
		Peers:        []*ipnstate.PeerStatus{phone()},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	url, err := app.PairingQR()
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("prefix: %s", url[:32])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 40 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a png, len=%d", len(raw))
	}
	width := binary.BigEndian.Uint32(raw[16:20])
	if width < 768 {
		t.Fatalf("qr png width = %d, want >= 768 (220px UI of a dense 512px QR is unscannable)", width)
	}
}

func TestReceiveToRename(t *testing.T) {
	d := &tsdroptest.Daemon{
		BackendState: "Running",
		Self:         &ipnstate.PeerStatus{HostName: "macbook"},
		Inbox:        map[string][]byte{"note.txt": []byte("new")},
	}
	app := NewApp(tsdrop.New(d.LocalClient(t)))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := app.ReceiveTo(dir, "rename")
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("written = %v", written)
	}
}
