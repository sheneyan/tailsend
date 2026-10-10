package localsend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testServer(t *testing.T, dir string, wait Waiter) *Server {
	t.Helper()
	return &Server{
		Info: Info{
			Alias: "mac", Version: ProtocolVersion, DeviceType: "desktop",
			Fingerprint: "abc", Port: DefaultPort, Protocol: "https",
		},
		SaveDir:    dir,
		AutoSave:   func() bool { return false },
		Wait:       wait,
		AcceptWait: 2 * time.Minute,
	}
}

type scriptWait struct{ accept bool }

func (s scriptWait) Wait(context.Context, Incoming) (bool, error) { return s.accept, nil }

func TestInfoAndRegister(t *testing.T) {
	s := testServer(t, t.TempDir(), scriptWait{accept: true})
	var seen Info
	s.OnPeer = func(ip string, info Info) {
		if ip != "192.0.2.8" {
			t.Fatalf("ip %s", ip)
		}
		seen = info
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/localsend/v2/info", nil))
	if rr.Code != 200 {
		t.Fatalf("info %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/localsend/v1/info", nil))
	if rr.Code != 200 {
		t.Fatalf("v1 info %d", rr.Code)
	}
	body := bytes.NewBufferString(`{"alias":"Phone","version":"2.0","fingerprint":"peer","port":53317,"protocol":"https","deviceType":"mobile"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/register", body)
	req.RemoteAddr = "192.0.2.8:9999"
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 || seen.Fingerprint != "peer" {
		t.Fatalf("register %d seen %+v", rr.Code, seen)
	}
}

func TestPrepareAcceptDeclineTimeoutAndBusy(t *testing.T) {
	dir := t.TempDir()
	s := testServer(t, dir, scriptWait{accept: true})
	code, resp := prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":3}}}`)
	if code != 200 || resp.SessionID == "" || resp.Files["f1"] == "" {
		t.Fatalf("accept %d %+v", code, resp)
	}
	s.clearSession()

	s.Wait = scriptWait{accept: false}
	code, _ = prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":3}}}`)
	if code != 403 {
		t.Fatalf("decline %d", code)
	}

	s.Wait = blockWait{}
	s.AcceptWait = 20 * time.Millisecond
	code, _ = prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":3}}}`)
	if code != 403 {
		t.Fatalf("timeout %d", code)
	}

	s.AcceptWait = time.Hour
	hold := &holdWait{started: make(chan struct{})}
	s.Wait = hold
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", bytes.NewBufferString(`{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"b.png","size":1}}}`))
		s.Handler().ServeHTTP(rr, req.WithContext(ctx))
	}()
	<-hold.started
	code, _ = prepare(t, s, `{"info":{"alias":"Other","fingerprint":"x"},"files":{"f2":{"id":"f2","fileName":"c.png","size":1}}}`)
	if code != 409 {
		t.Fatalf("busy %d", code)
	}
}

type blockWait struct{}

func (blockWait) Wait(ctx context.Context, _ Incoming) (bool, error) {
	<-ctx.Done()
	return false, nil
}

type holdWait struct{ started chan struct{} }

func (h *holdWait) Wait(ctx context.Context, _ Incoming) (bool, error) {
	close(h.started)
	<-ctx.Done()
	return false, nil
}

func TestAutoSaveSkipsWaiter(t *testing.T) {
	s := testServer(t, t.TempDir(), scriptWait{accept: false})
	s.AutoSave = func() bool { return true }
	code, resp := prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":1}}}`)
	if code != 200 || resp.Files["f1"] == "" {
		t.Fatalf("autosave %d %+v", code, resp)
	}
}

func TestUploadTokenChecksumCancelAndName(t *testing.T) {
	dir := t.TempDir()
	s := testServer(t, dir, scriptWait{accept: true})
	payload := []byte("abc")
	sum := sha256.Sum256(payload)
	meta := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"../sec/a.png","size":3,"sha256":"` + hex.EncodeToString(sum[:]) + `"}}}`
	code, resp := prepare(t, s, meta)
	if code != 200 {
		t.Fatal(code)
	}
	bad := upload(t, s, resp.SessionID, "f1", "nope", payload)
	if bad != 403 {
		t.Fatalf("bad token %d", bad)
	}
	ok := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], payload)
	if ok != 200 {
		t.Fatalf("upload %d", ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.png")); err != nil {
		t.Fatal(err)
	}
	s.clearSession()

	badSum := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"b.png","size":3,"sha256":"0000000000000000000000000000000000000000000000000000000000000000"}}}`
	_, resp = prepare(t, s, badSum)
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], payload); code != 422 {
		t.Fatalf("checksum %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.png")); !os.IsNotExist(err) {
		t.Fatalf("partial left: %v", err)
	}
	s.clearSession()

	if err := os.WriteFile(filepath.Join(dir, "a.png"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, resp = prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":3}}}`)
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], payload); code != 200 {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(dir, "a (2).png")); err != nil {
		t.Fatal(err)
	}
	s.clearSession()

	_, resp = prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"c.png","size":3}}}`)
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], payload); code != 200 {
		t.Fatal(code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/cancel?sessionId="+resp.SessionID, nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("cancel %d", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.png")); !os.IsNotExist(err) {
		t.Fatalf("cancel left file: %v", err)
	}
}

func prepare(t *testing.T, s *Server, body string) (int, PrepareResponse) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", bytes.NewBufferString(body))
	s.Handler().ServeHTTP(rr, req)
	var resp PrepareResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	return rr.Code, resp
}

func upload(t *testing.T, s *Server, session, file, token string, body []byte) int {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/upload?sessionId="+session+"&fileId="+file+"&token="+token, bytes.NewReader(body))
	s.Handler().ServeHTTP(rr, req)
	return rr.Code
}

type acceptAfterDone struct{}

func (acceptAfterDone) Wait(ctx context.Context, _ Incoming) (bool, error) {
	<-ctx.Done()
	return true, nil
}

func TestPrepareCanceledContextDoesNotAccept(t *testing.T) {
	s := testServer(t, t.TempDir(), acceptAfterDone{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	body := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":1}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/localsend/v2/prepare-upload", bytes.NewBufferString(body))
	s.Handler().ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != 403 {
		t.Fatalf("canceled %d", rr.Code)
	}
	s.Wait = scriptWait{accept: true}
	code, resp := prepare(t, s, body)
	if code != 200 || resp.SessionID == "" {
		t.Fatalf("next %d %+v", code, resp)
	}
}

func TestIdleTimeoutFreesSession(t *testing.T) {
	s := testServer(t, t.TempDir(), scriptWait{accept: true})
	s.IdleWait = 30 * time.Millisecond
	meta := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":1}}}`
	code, first := prepare(t, s, meta)
	if code != 200 || first.SessionID == "" {
		t.Fatalf("accept %d %+v", code, first)
	}
	deadline := time.Now().Add(2 * time.Second)
	var nextCode int
	var next PrepareResponse
	for time.Now().Before(deadline) {
		nextCode, next = prepare(t, s, meta)
		if nextCode == 200 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if nextCode != 200 || next.SessionID == "" || next.SessionID == first.SessionID {
		t.Fatalf("after idle %d %+v", nextCode, next)
	}
}

func TestUploadSameFileTwice(t *testing.T) {
	dir := t.TempDir()
	s := testServer(t, dir, scriptWait{accept: true})
	meta := `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":3},"f2":{"id":"f2","fileName":"b.png","size":3}}}`
	code, resp := prepare(t, s, meta)
	if code != 200 {
		t.Fatal(code)
	}
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], []byte("abc")); code != 200 {
		t.Fatalf("first %d", code)
	}
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], []byte("xyz")); code != 409 {
		t.Fatalf("second %d", code)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abc" {
		t.Fatalf("replaced %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.png")); !os.IsNotExist(err) {
		t.Fatalf("other file: %v", err)
	}
}

func TestUploadRejectsWrongSize(t *testing.T) {
	dir := t.TempDir()
	s := testServer(t, dir, scriptWait{accept: true})
	code, resp := prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":2}}}`)
	if code != 200 {
		t.Fatal(code)
	}
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], []byte("abc")); code != 400 {
		t.Fatalf("extra %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.png")); !os.IsNotExist(err) {
		t.Fatalf("extra left file: %v", err)
	}
	s.clearSession()

	code, resp = prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"b.png","size":3}}}`)
	if code != 200 {
		t.Fatal(code)
	}
	if code := upload(t, s, resp.SessionID, "f1", resp.Files["f1"], []byte("ab")); code != 400 {
		t.Fatalf("short %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.png")); !os.IsNotExist(err) {
		t.Fatalf("short left file: %v", err)
	}
}

func TestPrepareRejectsNegativeSize(t *testing.T) {
	s := testServer(t, t.TempDir(), scriptWait{accept: true})
	code, _ := prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":-1}}}`)
	if code != 400 {
		t.Fatalf("neg %d", code)
	}
	code, resp := prepare(t, s, `{"info":{"alias":"Phone","fingerprint":"peer"},"files":{"f1":{"id":"f1","fileName":"a.png","size":1}}}`)
	if code != 200 || resp.SessionID == "" {
		t.Fatalf("next %d %+v", code, resp)
	}
}
