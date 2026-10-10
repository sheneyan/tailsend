package localsend

import (
	"context"
	"net"
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
