package localsend

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectFilesFlattensDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := CollectFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("%+v", files)
	}
	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = true
	}
	if !names["a.txt"] || !names["a (2).txt"] {
		t.Fatalf("%v", names)
	}
}

func TestSendRejectsWrongCertAndRetriesPINOnce(t *testing.T) {
	dir := t.TempDir()
	good, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	other, err := LoadOrCreateCert(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	goodFP := FingerprintDER(good.Certificate[0])
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{other}})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := Info{Alias: "mac", Version: ProtocolVersion, DeviceType: "desktop", Fingerprint: goodFP, Port: DefaultPort, Protocol: "https"}
	err = Send(context.Background(), local, Target{IP: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Fingerprint: goodFP, Alias: "x"}, []FileSend{{Path: file, Name: "a.txt"}}, nil, nil)
	if err == nil || err.Error() != ErrCertMismatch.Error() {
		t.Fatalf("got %v", err)
	}

	pins := 0
	srvDir := t.TempDir()
	recv := testServer(t, srvDir, scriptWait{accept: true})
	var gotPIN string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/localsend/v2/prepare-upload", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pin") == "" {
			http.Error(w, "pin", http.StatusUnauthorized)
			return
		}
		gotPIN = r.URL.Query().Get("pin")
		if pins == 0 {
			pins++
			http.Error(w, "pin", http.StatusUnauthorized)
			return
		}
		recv.Handler().ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/localsend/v2/upload", recv.Handler().ServeHTTP)
	ln2, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{good}})
	if err != nil {
		t.Fatal(err)
	}
	defer ln2.Close()
	go http.Serve(ln2, mux)
	asks := 0
	err = Send(context.Background(), local, Target{IP: "127.0.0.1", Port: ln2.Addr().(*net.TCPAddr).Port, Fingerprint: goodFP, Alias: "phone"}, []FileSend{{Path: file, Name: "a.txt"}}, func() (string, error) {
		asks++
		return "1234", nil
	}, nil)
	if err == nil || err.Error() != ErrInvalidPIN.Error() {
		t.Fatalf("first retry should fail, got %v asks %d", err, asks)
	}
	if asks != 1 {
		t.Fatalf("asks %d", asks)
	}

	asks = 0
	pins = 1 // next empty pin still 401; provided pin is accepted
	err = Send(context.Background(), local, Target{IP: "127.0.0.1", Port: ln2.Addr().(*net.TCPAddr).Port, Fingerprint: goodFP, Alias: "phone"}, []FileSend{{Path: file, Name: "a.txt"}}, func() (string, error) {
		asks++
		return "9999", nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if asks != 1 || gotPIN != "9999" {
		t.Fatalf("asks %d pin %s", asks, gotPIN)
	}
}
