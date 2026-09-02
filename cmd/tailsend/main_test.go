package main

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheneyan/tailsend/internal/tsdrop"
	"github.com/sheneyan/tailsend/internal/tsdrop/tsdroptest"

	"tailscale.com/ipn/ipnstate"
)

func running(t *testing.T, peers ...*ipnstate.PeerStatus) (*tsdrop.Client, *tsdroptest.Daemon) {
	t.Helper()
	d := &tsdroptest.Daemon{
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
	return tsdrop.New(d.LocalClient(t)), d
}

func TestCLIStatus(t *testing.T) {
	c, _ := running(t)
	var out bytes.Buffer
	code := run([]string{"status"}, c, &out, &out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "Running") || !strings.Contains(s, "macbook") {
		t.Fatalf("status output: %s", s)
	}
}

func TestCLIListJSON(t *testing.T) {
	c, _ := running(t, &ipnstate.PeerStatus{
		ID:             "n-phone",
		HostName:       "pixel",
		DNSName:        "pixel.tailnet.ts.net.",
		OS:             "android",
		Online:         true,
		PeerAPIURL:     []string{"http://100.64.0.2:1"},
		TaildropTarget: ipnstate.TaildropTargetAvailable,
	})
	var out bytes.Buffer
	code := run([]string{"list", "--json"}, c, &out, ioDiscard())
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"hostname": "pixel"`) {
		t.Fatalf("json: %s", out.String())
	}
}

func TestCLIPairJSON(t *testing.T) {
	c, _ := running(t, &ipnstate.PeerStatus{
		ID:             "n-phone",
		HostName:       "pixel",
		DNSName:        "pixel.tailnet.ts.net.",
		OS:             "android",
		Online:         true,
		PeerAPIURL:     []string{"http://100.64.0.2:1"},
		TaildropTarget: ipnstate.TaildropTargetAvailable,
	})
	var listOut, pairOut bytes.Buffer
	if code := run([]string{"list", "--json"}, c, &listOut, ioDiscard()); code != 0 {
		t.Fatalf("list exit %d: %s", code, listOut.String())
	}
	if code := run([]string{"pair"}, c, &pairOut, ioDiscard()); code != 0 {
		t.Fatalf("pair exit %d: %s", code, pairOut.String())
	}
	if strings.TrimSpace(listOut.String()) != strings.TrimSpace(pairOut.String()) {
		t.Fatalf("pair JSON != list --json\nlist: %s\npair: %s", listOut.String(), pairOut.String())
	}
}

func TestCLIPairQR(t *testing.T) {
	c, _ := running(t, &ipnstate.PeerStatus{
		ID:             "n-phone",
		HostName:       "pixel",
		DNSName:        "pixel.tailnet.ts.net.",
		OS:             "android",
		Online:         true,
		PeerAPIURL:     []string{"http://100.64.0.2:1"},
		TaildropTarget: ipnstate.TaildropTargetAvailable,
	})
	var out bytes.Buffer
	code := run([]string{"pair", "--qr"}, c, &out, ioDiscard())
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, `"hostname": "pixel"`) {
		t.Fatalf("qr output missing JSON: %s", s)
	}
	if !strings.ContainsAny(s, "█▀▄ ") {
		t.Fatalf("qr output missing terminal QR: %s", s)
	}
}

func TestCLISendRequiresColon(t *testing.T) {
	c, _ := running(t)
	var errb bytes.Buffer
	code := run([]string{"send", "file.txt", "pixel"}, c, ioDiscard(), &errb)
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(errb.String(), ":") {
		t.Fatalf("hint missing colon: %s", errb.String())
	}
}

func TestCLISendByName(t *testing.T) {
	c, d := running(t, &ipnstate.PeerStatus{
		ID:             "n-phone",
		HostName:       "pixel",
		DNSName:        "pixel.tailnet.ts.net.",
		OS:             "android",
		Online:         true,
		PeerAPIURL:     []string{"http://100.64.0.2:1"},
		TaildropTarget: ipnstate.TaildropTargetAvailable,
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "hi.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := run([]string{"send", path, "pixel:"}, c, &out, &out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if len(d.Puts) != 1 || string(d.Puts[0].Body) != "hello" {
		t.Fatalf("puts = %+v", d.Puts)
	}
}

func TestCLIRecv(t *testing.T) {
	c, d := running(t)
	d.Inbox = map[string][]byte{"shot.png": []byte("PNG")}
	dest := t.TempDir()
	var out bytes.Buffer
	code := run([]string{"recv", dest}, c, &out, &out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	got, err := os.ReadFile(filepath.Join(dest, "shot.png"))
	if err != nil || string(got) != "PNG" {
		t.Fatalf("file = %q err=%v", got, err)
	}
}

func TestCLIDaemonDown(t *testing.T) {
	d := &tsdroptest.Daemon{DialErr: true}
	c := tsdrop.New(d.LocalClient(t))
	var errb bytes.Buffer
	code := run([]string{"status"}, c, ioDiscard(), &errb)
	if code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, errb.String())
	}
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }
