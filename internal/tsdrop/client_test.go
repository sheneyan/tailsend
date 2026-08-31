package tsdrop_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheneyan/tailsend/internal/tsdrop"
	"github.com/sheneyan/tailsend/internal/tsdrop/tsdroptest"

	"tailscale.com/ipn/ipnstate"
)

func phonePeer() *ipnstate.PeerStatus {
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

func runningDaemon(peers ...*ipnstate.PeerStatus) *tsdroptest.Daemon {
	return &tsdroptest.Daemon{
		BackendState: "Running",
		Self: &ipnstate.PeerStatus{
			ID:           "n-self",
			HostName:     "macbook",
			DNSName:      "macbook.tailnet.ts.net.",
			OS:           "macOS",
			Online:       true,
			TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		},
		Peers: peers,
	}
}

func TestProbeRunning(t *testing.T) {
	d := runningDaemon()
	c := tsdrop.New(d.LocalClient(t))
	st, self, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st != tsdrop.Running {
		t.Fatalf("state = %v, want Running", st)
	}
	if self.Hostname != "macbook" {
		t.Fatalf("hostname = %q", self.Hostname)
	}
	if self.OS != "macOS" {
		t.Fatalf("os = %q", self.OS)
	}
	if len(self.IPs) != 1 || self.IPs[0].String() != "100.64.0.1" {
		t.Fatalf("ips = %v", self.IPs)
	}
}

func TestProbeNeedsLogin(t *testing.T) {
	d := &tsdroptest.Daemon{BackendState: "NeedsLogin"}
	c := tsdrop.New(d.LocalClient(t))
	st, _, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st != tsdrop.NeedsLogin {
		t.Fatalf("state = %v, want NeedsLogin", st)
	}
}

func TestProbeDaemonDown(t *testing.T) {
	d := &tsdroptest.Daemon{DialErr: true}
	c := tsdrop.New(d.LocalClient(t))
	_, _, err := c.Probe(context.Background())
	if !errors.Is(err, tsdrop.ErrDaemonDown) {
		t.Fatalf("err = %v, want ErrDaemonDown", err)
	}
}

func TestTargetsIncludesSendableAndReasons(t *testing.T) {
	offline := &ipnstate.PeerStatus{
		ID:                  "n-offline",
		HostName:            "nas",
		DNSName:             "nas.tailnet.ts.net.",
		OS:                  "linux",
		Online:              false,
		TaildropTarget:      ipnstate.TaildropTargetOffline,
		NoFileSharingReason: "offline",
	}
	other := &ipnstate.PeerStatus{
		ID:                  "n-other",
		HostName:            "work",
		DNSName:             "work.tailnet.ts.net.",
		OS:                  "windows",
		Online:              true,
		TaildropTarget:      ipnstate.TaildropTargetOwnedByOtherUser,
		NoFileSharingReason: "owned by different user",
	}
	d := runningDaemon(phonePeer(), offline, other)
	c := tsdrop.New(d.LocalClient(t))
	got, err := c.Targets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byHost := map[string]tsdrop.Target{}
	for _, t := range got {
		byHost[t.Hostname] = t
	}
	phone := byHost["pixel"]
	if phone.StableID != "n-phone" || phone.Reason != "" || phone.PeerAPIURL != "http://100.64.0.2:41641" {
		t.Fatalf("phone = %+v", phone)
	}
	if byHost["nas"].Reason == "" {
		t.Fatalf("offline nas should have a reason: %+v", byHost["nas"])
	}
	if byHost["work"].Reason == "" {
		t.Fatalf("other-user work should have a reason: %+v", byHost["work"])
	}
}

func TestSendFile(t *testing.T) {
	d := runningDaemon(phonePeer())
	c := tsdrop.New(d.LocalClient(t))
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hi tailsend"), 0o644); err != nil {
		t.Fatal(err)
	}
	var last tsdrop.Progress
	err := c.Send(context.Background(), "n-phone", []tsdrop.SendItem{{Path: path}}, func(p tsdrop.Progress) {
		last = p
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Puts) != 1 {
		t.Fatalf("puts = %d", len(d.Puts))
	}
	if d.Puts[0].Target != "n-phone" || d.Puts[0].Name != "hello.txt" {
		t.Fatalf("put = %+v", d.Puts[0])
	}
	if string(d.Puts[0].Body) != "hi tailsend" {
		t.Fatalf("body = %q", d.Puts[0].Body)
	}
	if last.State != "done" || last.Sent != int64(len("hi tailsend")) {
		t.Fatalf("progress = %+v", last)
	}
}

func TestSendDirectoryZips(t *testing.T) {
	d := runningDaemon(phonePeer())
	c := tsdrop.New(d.LocalClient(t))
	root := t.TempDir()
	folder := filepath.Join(root, "photos")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "a.jpg"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "raw", "b.jpg"), []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(context.Background(), "n-phone", []tsdrop.SendItem{{Path: folder}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(d.Puts) != 1 || d.Puts[0].Name != "photos.zip" {
		t.Fatalf("puts = %+v", d.Puts)
	}
	zr, err := zip.NewReader(bytes.NewReader(d.Puts[0].Body), int64(len(d.Puts[0].Body)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		names[f.Name] = string(b)
	}
	if names["a.jpg"] != "aaa" || names["raw/b.jpg"] != "bbb" {
		t.Fatalf("zip entries = %v", names)
	}
}

func TestInboxAndReceiveConflict(t *testing.T) {
	d := runningDaemon()
	d.Inbox = map[string][]byte{
		"note.txt": []byte("from phone"),
	}
	c := tsdrop.New(d.LocalClient(t))
	waiting, err := c.Inbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 1 || waiting[0].Name != "note.txt" || waiting[0].Size != int64(len("from phone")) {
		t.Fatalf("inbox = %+v", waiting)
	}

	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "note.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := c.Receive(context.Background(), dest, tsdrop.ConflictRename)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] == filepath.Join(dest, "note.txt") {
		t.Fatalf("written = %v", written)
	}
	if got, _ := os.ReadFile(written[0]); string(got) != "from phone" {
		t.Fatalf("renamed content = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "note.txt")); string(got) != "old" {
		t.Fatalf("original overwritten: %q", got)
	}
	if len(d.Inbox) != 0 {
		t.Fatalf("inbox not drained: %v", d.Inbox)
	}
}

func TestReceiveSkipLeavesInbox(t *testing.T) {
	d := runningDaemon()
	d.Inbox = map[string][]byte{"note.txt": []byte("new")}
	c := tsdrop.New(d.LocalClient(t))
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "note.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := c.Receive(context.Background(), dest, tsdrop.ConflictSkip)
	if err == nil {
		t.Fatal("expected skip error")
	}
	if len(written) != 0 {
		t.Fatalf("written = %v", written)
	}
	if _, ok := d.Inbox["note.txt"]; !ok {
		t.Fatal("skipped file should remain in inbox")
	}
}

func TestFileSharingDisabled(t *testing.T) {
	d := runningDaemon(phonePeer())
	d.FileSharingDisabled = true
	c := tsdrop.New(d.LocalClient(t))
	_, err := c.Targets(context.Background())
	if !errors.Is(err, tsdrop.ErrFileSharingDisabled) {
		t.Fatalf("err = %v", err)
	}
}

func TestExportTargetsJSON(t *testing.T) {
	d := runningDaemon(phonePeer())
	c := tsdrop.New(d.LocalClient(t))
	b, err := c.ExportTargetsJSON(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []tsdrop.Target
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Hostname != "pixel" || got[0].PeerAPIURL == "" {
		t.Fatalf("json = %s", b)
	}
}

func TestMapPushError(t *testing.T) {
	d := runningDaemon(phonePeer())
	d.PutErr = "owned by different user; can only send files to your own devices"
	c := tsdrop.New(d.LocalClient(t))
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("x"), 0o644)
	err := c.Send(context.Background(), "n-phone", []tsdrop.SendItem{{Path: path}}, nil)
	if !errors.Is(err, tsdrop.ErrOtherUser) {
		t.Fatalf("err = %v", err)
	}
}

func TestDNSNameTrim(t *testing.T) {
	d := runningDaemon(phonePeer())
	c := tsdrop.New(d.LocalClient(t))
	got, err := c.Targets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(got[0].DNSName, ".") {
		t.Fatalf("DNSName should drop trailing dot, got %q", got[0].DNSName)
	}
}
