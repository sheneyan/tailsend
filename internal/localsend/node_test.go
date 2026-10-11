package localsend

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStartReportsPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	n, err := StartWith(context.Background(), Config{
		SaveDir:   t.TempDir(),
		ConfigDir: t.TempDir(),
	}, nil, nil, errString("listen tcp 127.0.0.1:53317: bind: address already in use"))
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	if n.State().Status != "53317 is in use" {
		t.Fatalf("%+v", n.State())
	}
}

func errString(s string) error { return stringErr(s) }

type stringErr string

func (e stringErr) Error() string { return string(e) }

func TestNodeAutoSaveAndOffer(t *testing.T) {
	dir := t.TempDir()
	cfgDir := t.TempDir()
	if err := SaveSettings(cfgDir, Settings{AutoSave: false}); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	offers := make(chan Incoming, 1)
	n, err := StartWith(context.Background(), Config{
		SaveDir:   dir,
		ConfigDir: cfgDir,
		OnOffer:   func(in Incoming) { offers <- in },
	}, ln, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	if n.State().AutoSave {
		t.Fatal("autoSave")
	}
	go func() {
		in := <-offers
		n.Accept(in.SessionID)
	}()
	// Drive prepare through the server waiter by calling Wait directly.
	ok, err := n.Wait(context.Background(), Incoming{SessionID: "s1", Alias: "Phone"})
	if err != nil || !ok {
		t.Fatalf("accept %v %v", ok, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ok, err = n.Wait(ctx, Incoming{SessionID: "s2", Alias: "Phone"})
	if ok {
		t.Fatal("timeout should decline")
	}
	if err := n.SetAutoSave(true); err != nil {
		t.Fatal(err)
	}
	if !n.State().AutoSave {
		t.Fatal("flag")
	}
}

func TestCloseUnblocksPrepare(t *testing.T) {
	cfgDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	n, err := StartWith(context.Background(), Config{
		SaveDir:   t.TempDir(),
		ConfigDir: cfgDir,
		OnOffer:   func(Incoming) { close(ready) },
	}, ln, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(n.Close)
	cert, err := LoadOrCreateCert(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		Certificates:       []tls.Certificate{cert},
	}}}
	body := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"b.png","size":1}}}`
	done := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := client.Post("https://"+ln.Addr().String()+"/api/localsend/v2/prepare-upload", "application/json", strings.NewReader(body))
		if err != nil {
			errCh <- err
			return
		}
		done <- res
	}()
	select {
	case <-ready:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("offer")
	}
	start := time.Now()
	n.Close()
	select {
	case res := <-done:
		defer res.Body.Close()
		if time.Since(start) > 2*time.Second {
			t.Fatalf("took %s", time.Since(start))
		}
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("status %d", res.StatusCode)
		}
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("hung")
	}
}
