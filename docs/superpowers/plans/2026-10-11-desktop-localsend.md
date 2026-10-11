# Desktop LocalSend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** While the desktop GUI is open, speak LocalSend v2 on the LAN so an official LocalSend phone can see this computer and exchange files, without using Tailscale.

**Architecture:** `internal/localsend` owns the certificate, UDP peer table, HTTPS server, and pinned client. `desktop.App` starts that node on startup and emits Wails events. Vue adds an **On this network** list, an incoming dialog, and an auto-save checkbox under the existing Tailscale grid. Taildrop code stays in `internal/tsdrop`.

**Tech Stack:** Go 1.22, `net/http`, `crypto/tls`, Wails v2, Vue 3. No new module dependencies.

**Spec:** `docs/superpowers/specs/2026-10-11-desktop-localsend-design.md`

---

## File map

| File | Role |
|---|---|
| Create `internal/localsend/protocol.go` | Info JSON, fingerprint, safe file name, announce parse |
| Create `internal/localsend/protocol_test.go` | Parse, fingerprint, base name |
| Create `internal/localsend/cert.go` | PEM cert + `localsend.json` under the config dir |
| Create `internal/localsend/cert_test.go` | Stable fingerprint, default auto-save off |
| Create `internal/localsend/server.go` | v1/v2 info, register, prepare-upload, upload, cancel |
| Create `internal/localsend/server_test.go` | Accept, decline, timeout, 409, token, sha256, cancel, names |
| Create `internal/localsend/discover.go` | Peer table, 30s prune, UDP read loop, 5s announce |
| Create `internal/localsend/discover_test.go` | Ignore self, prune, UDP reply |
| Create `internal/localsend/send.go` | Flatten paths, pinned TLS, prepare/upload, one PIN retry |
| Create `internal/localsend/send_test.go` | Cert pin, PIN once, directory flatten, 403/409 |
| Create `internal/localsend/node.go` | Bind 53317, wire server + UDP, offer wait, settings |
| Create `internal/localsend/node_test.go` | Port status, auto-save skips the waiter, offer answer |
| Create `desktop/lan.go` | Start/stop node, Wails methods, progress events |
| Modify `desktop/app.go` | Call `startLAN` from `startup`; outbound busy flag |
| Modify `desktop/main.go` | `OnShutdown` closes the node |
| Modify `desktop/frontend/src/App.vue` | Nearby list, dialogs, checkbox |
| Modify `desktop/frontend/wailsjs/go/main/App.js` | JS wrappers for the new methods |
| Modify `README.md`, `README.zh.md`, `docs/architecture.md`, `CHANGELOG.md` | LAN peer is separate from Taildrop |

Do not add Android or CLI sockets. Do not scan the subnet. Do not implement `prepare-download`.

Run protocol tests from the repo root:

```bash
go test ./internal/localsend/ -count=1
```

---

### Task 1: Protocol types

**Files:**
- Create: `internal/localsend/protocol.go`
- Create: `internal/localsend/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/protocol_test.go`:

```go
package localsend

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestFingerprintDERLowerHex(t *testing.T) {
	sum := sha256.Sum256([]byte("cert"))
	got := FingerprintDER([]byte("cert"))
	want := hexEncode(sum[:])
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if bytes.ContainsAny(got, "ABCDEF") {
		t.Fatalf("fingerprint must be lowercase: %s", got)
	}
}

func TestParseAnnounceRequiresFingerprint(t *testing.T) {
	_, err := ParseAnnounce([]byte(`{"alias":"Phone","announce":true}`))
	if err == nil {
		t.Fatal("expected error")
	}
	info, err := ParseAnnounce([]byte(`{"alias":"Phone","version":"2.0","fingerprint":"abc","port":53317,"protocol":"https","deviceType":"mobile","announce":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Alias != "Phone" || info.Port != 53317 || info.Announce == nil || !*info.Announce {
		t.Fatalf("%+v", info)
	}
}

func TestSafeBaseStripsPath(t *testing.T) {
	cases := map[string]string{
		"a.png":           "a.png",
		"../a.png":        "a.png",
		`dir\a.png`:       "a.png",
		"/tmp/a.png":      "a.png",
		"..":              "file",
		"":                "file",
		"   ":             "file",
	}
	for in, want := range cases {
		if got := SafeBase(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}
```

Add this helper at the bottom of the test file (it stays in tests; production uses `encoding/hex`):

```go
func hexEncode(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = digits[c>>4]
		out[i*2+1] = digits[c&0x0f]
	}
	return string(out)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1`

Expected: FAIL. `FingerprintDER` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/protocol.go`:

```go
package localsend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
)

const (
	ProtocolVersion = "2.0"
	DefaultPort     = 53317
	MulticastIP     = "224.0.0.167"
)

type Info struct {
	Alias       string `json:"alias"`
	Version     string `json:"version"`
	DeviceModel string `json:"deviceModel,omitempty"`
	DeviceType  string `json:"deviceType,omitempty"`
	Fingerprint string `json:"fingerprint"`
	Port        int    `json:"port,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Download    bool   `json:"download"`
	Announce    *bool  `json:"announce,omitempty"`
}

func FingerprintDER(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

func ParseAnnounce(b []byte) (Info, error) {
	var info Info
	if err := json.Unmarshal(b, &info); err != nil {
		return Info{}, err
	}
	if info.Fingerprint == "" {
		return Info{}, errors.New("missing fingerprint")
	}
	return info, nil
}

func SafeBase(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimSpace(name)
	base := path.Base(name)
	if base == "." || base == "/" || base == ".." || strings.TrimSpace(base) == "" {
		return "file"
	}
	return base
}

func BoolPtr(v bool) *bool { return &v }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/protocol.go internal/localsend/protocol_test.go
git commit -m "feat: LocalSend announce and file-name helpers"
```

---

### Task 2: Certificate and settings

**Files:**
- Create: `internal/localsend/cert.go`
- Create: `internal/localsend/cert_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/cert_test.go`:

```go
package localsend

import "testing"

func TestLoadOrCreateCertStableFingerprint(t *testing.T) {
	dir := t.TempDir()
	c1, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	f1 := FingerprintDER(c1.Certificate[0])
	f2 := FingerprintDER(c2.Certificate[0])
	if f1 == "" || f1 != f2 {
		t.Fatalf("fingerprint changed: %s %s", f1, f2)
	}
}

func TestSettingsDefaultAutoSaveOff(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.AutoSave {
		t.Fatal("default autoSave must be false")
	}
	if err := SaveSettings(dir, Settings{AutoSave: true}); err != nil {
		t.Fatal(err)
	}
	s, err = LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s.AutoSave {
		t.Fatal("autoSave not persisted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1 -run 'TestLoadOrCreateCert|TestSettings'`

Expected: FAIL. `LoadOrCreateCert` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/cert.go`:

```go
package localsend

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	AutoSave bool `json:"autoSave"`
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "tailsend")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func LoadSettings(dir string) (Settings, error) {
	b, err := os.ReadFile(filepath.Join(dir, "localsend.json"))
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func SaveSettings(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, "localsend.json"), b, 0o600)
}

func LoadOrCreateCert(dir string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, "localsend-cert.pem")
	keyPath := filepath.Join(dir, "localsend-key.pem")
	if pair, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return pair, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tailsend"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		DNSNames:     []string{"tailsend.local"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/cert.go internal/localsend/cert_test.go
git commit -m "feat: persist LocalSend cert and auto-save setting"
```

---

### Task 3: HTTPS receive server

**Files:**
- Create: `internal/localsend/server.go`
- Create: `internal/localsend/server_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/server_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1 -run 'TestInfo|TestPrepare|TestAutoSave|TestUpload'`

Expected: FAIL. `Server` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/server.go`:

```go
package localsend

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Incoming struct {
	SessionID string     `json:"sessionID"`
	Alias     string     `json:"alias"`
	Files     []FileMeta `json:"files"`
}

type FileMeta struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	FileType string `json:"fileType,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type PrepareRequest struct {
	Info  Info                `json:"info"`
	Files map[string]FileMeta `json:"files"`
}

type PrepareResponse struct {
	SessionID string            `json:"sessionId"`
	Files     map[string]string `json:"files"`
}

type Waiter interface {
	Wait(ctx context.Context, in Incoming) (bool, error)
}

type Server struct {
	Info       Info
	SaveDir    string
	AutoSave   func() bool
	Wait       Waiter
	OnPeer     func(ip string, info Info)
	OnSaved    func(alias string, count int)
	AcceptWait time.Duration

	mu      sync.Mutex
	session *recvSession
}

type recvSession struct {
	id     string
	alias  string
	tokens map[string]string
	metas  map[string]FileMeta
	paths  map[string]string
	got    int
}

func (s *Server) clearSession() {
	s.mu.Lock()
	s.session = nil
	s.mu.Unlock()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/localsend/v1/info", s.handleInfo)
	mux.HandleFunc("/api/localsend/v2/info", s.handleInfo)
	mux.HandleFunc("/api/localsend/v2/register", s.handleRegister)
	mux.HandleFunc("/api/localsend/v2/prepare-upload", s.handlePrepare)
	mux.HandleFunc("/api/localsend/v2/upload", s.handleUpload)
	mux.HandleFunc("/api/localsend/v2/cancel", s.handleCancel)
	return mux
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, s.Info)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusBadRequest)
		return
	}
	var info Info
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if s.OnPeer != nil && info.Fingerprint != "" {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		s.OnPeer(host, info)
	}
	writeJSON(w, http.StatusOK, s.Info)
}

func (s *Server) handlePrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusBadRequest)
		return
	}
	var req PrepareRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || len(req.Files) == 0 {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.session != nil {
		s.mu.Unlock()
		http.Error(w, "busy", http.StatusConflict)
		return
	}
	sess := &recvSession{
		id:     randomID(),
		alias:  req.Info.Alias,
		tokens: map[string]string{},
		metas:  map[string]FileMeta{},
		paths:  map[string]string{},
	}
	s.session = sess
	s.mu.Unlock()

	files := make([]FileMeta, 0, len(req.Files))
	for id, meta := range req.Files {
		if meta.ID == "" {
			meta.ID = id
		}
		files = append(files, meta)
	}
	accept := false
	if s.AutoSave != nil && s.AutoSave() {
		accept = true
	} else if s.Wait != nil {
		wait := s.AcceptWait
		if wait <= 0 {
			wait = 2 * time.Minute
		}
		ctx, cancel := context.WithTimeout(r.Context(), wait)
		ok, _ := s.Wait.Wait(ctx, Incoming{SessionID: sess.id, Alias: req.Info.Alias, Files: files})
		cancel()
		accept = ok
	}
	if !accept {
		s.clearSession()
		http.Error(w, "rejected", http.StatusForbidden)
		return
	}
	resp := PrepareResponse{SessionID: sess.id, Files: map[string]string{}}
	s.mu.Lock()
	for _, meta := range files {
		tok := randomID()
		sess.tokens[meta.ID] = tok
		sess.metas[meta.ID] = meta
		resp.Files[meta.ID] = tok
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	s.mu.Lock()
	sess := s.session
	if sess == nil || q.Get("sessionId") != sess.id {
		s.mu.Unlock()
		http.Error(w, "token", http.StatusForbidden)
		return
	}
	fileID := q.Get("fileId")
	if sess.tokens[fileID] == "" || sess.tokens[fileID] != q.Get("token") {
		s.mu.Unlock()
		http.Error(w, "token", http.StatusForbidden)
		return
	}
	meta := sess.metas[fileID]
	s.mu.Unlock()

	dest, err := destPath(s.SaveDir, meta.FileName)
	if err != nil {
		s.failUpload(w)
		return
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		s.failUpload(w)
		return
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), r.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(dest)
		s.failUpload(w)
		return
	}
	if meta.SHA256 != "" && !strings.EqualFold(meta.SHA256, hex.EncodeToString(h.Sum(nil))) {
		os.Remove(dest)
		http.Error(w, "checksum", http.StatusUnprocessableEntity)
		return
	}
	s.mu.Lock()
	if s.session == sess {
		sess.paths[fileID] = dest
		sess.got++
		done := sess.got == len(sess.metas)
		alias, n := sess.alias, sess.got
		if done {
			s.session = nil
		}
		s.mu.Unlock()
		if done && s.OnSaved != nil {
			s.OnSaved(alias, n)
		}
	} else {
		s.mu.Unlock()
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) failUpload(w http.ResponseWriter) {
	http.Error(w, "write", http.StatusInternalServerError)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusBadRequest)
		return
	}
	id := r.URL.Query().Get("sessionId")
	s.mu.Lock()
	sess := s.session
	if sess != nil && sess.id == id {
		for _, p := range sess.paths {
			os.Remove(p)
		}
		s.session = nil
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func destPath(dir, fileName string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := SafeBase(fileName)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i < 1000; i++ {
		name := base
		if i > 1 {
			name = stem + " (" + strconv.Itoa(i) + ")" + ext
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return p, nil
		}
	}
	return "", errors.New("no free name")
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
```

`clearSession` is on `*Server`. Tests are `package localsend`, so they can call it. The busy case must cancel the blocked request when the test ends. Use a pointer waiter and a cancelable request, as written in the test below. Do not copy `holdWait` by value.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/server.go internal/localsend/server_test.go
git commit -m "feat: LocalSend receive session over HTTP"
```

---

### Task 4: Peer table and UDP

**Files:**
- Create: `internal/localsend/discover.go`
- Create: `internal/localsend/discover_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/discover_test.go`:

```go
package localsend

import (
	"net"
	"testing"
	"time"
)

func TestTableDropsSelfAndPrunes(t *testing.T) {
	now := time.Unix(1000, 0)
	tab := NewTable("self")
	if tab.HandleUDP(net.ParseIP("192.0.2.1"), []byte(`{"fingerprint":"self","announce":true}`), now) {
		t.Fatal("self must not reply")
	}
	if len(tab.List(now)) != 0 {
		t.Fatal("stored self")
	}
	reply := tab.HandleUDP(net.ParseIP("192.0.2.2"), []byte(`{"alias":"Phone","fingerprint":"peer","port":53317,"protocol":"https","deviceType":"mobile","announce":true}`), now)
	if !reply {
		t.Fatal("expected reply")
	}
	peers := tab.List(now.Add(30 * time.Second))
	if len(peers) != 0 {
		t.Fatalf("still listed after 30s: %+v", peers)
	}
	peers = tab.List(now.Add(29 * time.Second))
	if len(peers) != 1 || peers[0].Alias != "Phone" || peers[0].IP != "192.0.2.2" || peers[0].Port != 53317 {
		t.Fatalf("%+v", peers)
	}
}

func TestUDPLoopRepliesAnnounceFalse(t *testing.T) {
	a, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	tab := NewTable("self")
	self := Info{Alias: "mac", Version: ProtocolVersion, Fingerprint: "self", Port: DefaultPort, Protocol: "https", DeviceType: "desktop"}
	done := make(chan struct{})
	go func() {
		_ = ReadAnnounce(a, tab, self, func(ip string, port int) {
			if ip != "127.0.0.1" || port != 53317 {
				t.Errorf("register target %s %d", ip, port)
			}
			close(done)
		})
	}()
	msg := []byte(`{"alias":"Phone","fingerprint":"peer","port":53317,"protocol":"https","announce":true}`)
	if _, err := b.WriteToUDP(msg, a.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("register callback timed out")
	}
	_ = b.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := b.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	info, err := ParseAnnounce(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if info.Announce == nil || *info.Announce {
		t.Fatalf("reply announce: %+v", info.Announce)
	}
	if info.Fingerprint != "self" {
		t.Fatalf("reply fp %s", info.Fingerprint)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1 -run 'TestTable|TestUDP'`

Expected: FAIL. `NewTable` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/discover.go`:

```go
package localsend

import (
	"encoding/json"
	"net"
	"sync"
	"time"
)

const PeerTTL = 30 * time.Second

type Peer struct {
	Alias       string `json:"alias"`
	Fingerprint string `json:"fingerprint"`
	DeviceType  string `json:"deviceType"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	seen        time.Time
}

type Table struct {
	mu    sync.Mutex
	self  string
	peers map[string]Peer
}

func NewTable(selfFingerprint string) *Table {
	return &Table{self: selfFingerprint, peers: map[string]Peer{}}
}

func (t *Table) Upsert(ip string, info Info, now time.Time) {
	if info.Fingerprint == "" || info.Fingerprint == t.self {
		return
	}
	port := info.Port
	if port == 0 {
		port = DefaultPort
	}
	protocol := info.Protocol
	if protocol == "" {
		protocol = "https"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	prev := t.peers[info.Fingerprint]
	if ip == "" {
		ip = prev.IP
	}
	t.peers[info.Fingerprint] = Peer{
		Alias: info.Alias, Fingerprint: info.Fingerprint, DeviceType: info.DeviceType,
		IP: ip, Port: port, Protocol: protocol, seen: now,
	}
}

func (t *Table) HandleUDP(from net.IP, payload []byte, now time.Time) bool {
	info, err := ParseAnnounce(payload)
	if err != nil || info.Fingerprint == t.self {
		return false
	}
	ip := ""
	if from != nil {
		ip = from.String()
	}
	t.Upsert(ip, info, now)
	return info.Announce != nil && *info.Announce
}

func (t *Table) List(now time.Time) []Peer {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Peer, 0, len(t.peers))
	for fp, p := range t.peers {
		if now.Sub(p.seen) >= PeerTTL {
			delete(t.peers, fp)
			continue
		}
		out = append(out, p)
	}
	return out
}

func ReadAnnounce(conn *net.UDPConn, tab *Table, self Info, onAnnounce func(ip string, port int)) error {
	buf := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		payload := append([]byte(nil), buf[:n]...)
		if !tab.HandleUDP(addr.IP, payload, time.Now()) {
			continue
		}
		info, _ := ParseAnnounce(payload)
		if onAnnounce != nil {
			port := info.Port
			if port == 0 {
				port = DefaultPort
			}
			onAnnounce(addr.IP.String(), port)
		}
		reply := self
		reply.Announce = BoolPtr(false)
		b, err := json.Marshal(reply)
		if err != nil {
			continue
		}
		_, _ = conn.WriteToUDP(b, addr)
	}
}

func AnnounceBytes(info Info) ([]byte, error) {
	info.Announce = BoolPtr(true)
	return json.Marshal(info)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/discover.go internal/localsend/discover_test.go
git commit -m "feat: remember LocalSend peers from UDP announce"
```

---

### Task 5: Pinned sender

**Files:**
- Create: `internal/localsend/send.go`
- Create: `internal/localsend/send_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/send_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1 -run 'TestCollect|TestSend'`

Expected: FAIL. `CollectFiles` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/send.go`:

```go
package localsend

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"context"
)

var (
	ErrCertMismatch = errors.New("certificate does not match")
	ErrPINRequired  = errors.New("pin required")
	ErrInvalidPIN   = errors.New("invalid pin")
	ErrRejected     = errors.New("rejected")
	ErrBusy         = errors.New("peer is busy")
)

type FileSend struct {
	Path string
	Name string
}

type Target struct {
	IP          string
	Port        int
	Fingerprint string
	Alias       string
}

type ProgressFunc func(name string, sent, total int64, state, errText string)

func CollectFiles(paths []string) ([]FileSend, error) {
	var out []FileSend
	seen := map[string]int{}
	add := func(disk, name string) {
		name = SafeBase(name)
		n := seen[name]
		seen[name]++
		if n > 0 {
			ext := filepath.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			name = stem + " (" + strconv.Itoa(n+1) + ")" + ext
		}
		out = append(out, FileSend{Path: disk, Name: name})
	}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			add(p, st.Name())
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			add(path, d.Name())
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func Send(ctx context.Context, local Info, target Target, files []FileSend, askPIN func() (string, error), progress ProgressFunc) error {
	if len(files) == 0 {
		return errors.New("no files selected")
	}
	client, err := pinnedClient(target.Fingerprint)
	if err != nil {
		return err
	}
	pin := ""
	code, resp, err := prepareUpload(ctx, client, local, target, files, pin)
	if err != nil {
		return err
	}
	if code == http.StatusUnauthorized {
		if askPIN == nil {
			return ErrPINRequired
		}
		pin, err = askPIN()
		if err != nil {
			return err
		}
		code, resp, err = prepareUpload(ctx, client, local, target, files, pin)
		if err != nil {
			return err
		}
		if code == http.StatusUnauthorized {
			return ErrInvalidPIN
		}
	}
	switch code {
	case http.StatusForbidden:
		return ErrRejected
	case http.StatusConflict:
		return ErrBusy
	case http.StatusOK:
	default:
		return errors.New("prepare failed")
	}
	var total int64
	for _, f := range files {
		st, err := os.Stat(f.Path)
		if err != nil {
			return err
		}
		total += st.Size()
	}
	var sent int64
	for _, f := range files {
		token := resp.Files[f.Name]
		if token == "" {
			return errors.New("missing token")
		}
		body, err := os.Open(f.Path)
		if err != nil {
			return err
		}
		st, _ := body.Stat()
		url := endpoint(target, "/api/localsend/v2/upload") + "?sessionId=" + resp.SessionID + "&fileId=" + f.Name + "&token=" + token
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			body.Close()
			return err
		}
		req.ContentLength = st.Size()
		res, err := client.Do(req)
		body.Close()
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("upload failed")
		}
		sent += st.Size()
		if progress != nil {
			progress(f.Name, sent, total, "sending", "")
		}
	}
	if progress != nil {
		progress(target.Alias, sent, total, "done", "")
	}
	return nil
}

func prepareUpload(ctx context.Context, client *http.Client, local Info, target Target, files []FileSend, pin string) (int, PrepareResponse, error) {
	filesJSON := map[string]FileMeta{}
	for _, f := range files {
		st, err := os.Stat(f.Path)
		if err != nil {
			return 0, PrepareResponse{}, err
		}
		filesJSON[f.Name] = FileMeta{ID: f.Name, FileName: f.Name, Size: st.Size()}
	}
	local.Announce = nil
	payload := PrepareRequest{Info: local, Files: filesJSON}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, PrepareResponse{}, err
	}
	url := endpoint(target, "/api/localsend/v2/prepare-upload")
	if pin != "" {
		url += "?pin=" + pin
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return 0, PrepareResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrCertMismatch) || strings.Contains(err.Error(), ErrCertMismatch.Error()) {
			return 0, PrepareResponse{}, ErrCertMismatch
		}
		return 0, PrepareResponse{}, err
	}
	defer res.Body.Close()
	var resp PrepareResponse
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
			return 0, PrepareResponse{}, err
		}
	}
	return res.StatusCode, resp, nil
}

func endpoint(target Target, path string) string {
	return "https://" + net.JoinHostPort(target.IP, strconv.Itoa(target.Port)) + path
}

func pinnedClient(fingerprint string) (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
				VerifyConnection: func(cs tls.ConnectionState) error {
					if len(cs.PeerCertificates) == 0 {
						return ErrCertMismatch
					}
					if FingerprintDER(cs.PeerCertificates[0].Raw) != fingerprint {
						return ErrCertMismatch
					}
					return nil
				},
			},
		},
	}, nil
}
```

Run `gofmt` on `send.go`. `Send` takes `local Info` and passes it to `prepareUpload`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/send.go internal/localsend/send_test.go
git commit -m "feat: send files to a LocalSend peer with cert pin"
```

---

### Task 6: Node lifecycle

**Files:**
- Create: `internal/localsend/node.go`
- Create: `internal/localsend/node_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/localsend/node_test.go`:

```go
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
```

`StartWith` when `ln` is nil and `listenErr` is non-nil returns a node whose status is `53317 is in use` and does not announce.

`Wait` on timeout returns `(false, nil)`. The second `Wait` in the test has no `Accept`, so it hits `ctx.Done()`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/localsend/ -count=1 -run 'TestStart|TestNode'`

Expected: FAIL. `StartWith` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/localsend/node.go`:

```go
package localsend

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

const StatusPortInUse = "53317 is in use"

type Config struct {
	SaveDir    string
	ConfigDir  string
	OnPeers    func([]Peer)
	OnOffer    func(Incoming)
	OnSaved    func(alias string, count int)
	ListenAddr string // empty means ":53317"
}

type State struct {
	Status   string `json:"status"`
	AutoSave bool   `json:"autoSave"`
	Peers    []Peer `json:"peers"`
}

type Node struct {
	status   string
	cfg      Config
	settings Settings
	table    *Table
	srv      *Server
	httpSrv  *http.Server
	udp      *net.UDPConn
	cancel   context.CancelFunc

	mu      sync.Mutex
	pending map[string]chan bool
}

func Start(ctx context.Context, cfg Config) (*Node, error) {
	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":53317"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return StartWith(ctx, cfg, nil, nil, err)
	}
	udp, _ := net.ListenMulticastUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(MulticastIP), Port: DefaultPort})
	return StartWith(ctx, cfg, ln, udp, nil)
}

func StartWith(ctx context.Context, cfg Config, ln net.Listener, udp *net.UDPConn, listenErr error) (*Node, error) {
	if cfg.ConfigDir == "" {
		dir, err := ConfigDir()
		if err != nil {
			return nil, err
		}
		cfg.ConfigDir = dir
	}
	settings, err := LoadSettings(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	n := &Node{
		cfg:      cfg,
		settings: settings,
		pending:  map[string]chan bool{},
		table:    NewTable(""),
	}
	if ln == nil {
		n.status = StatusPortInUse
		return n, nil
	}
	cert, err := LoadOrCreateCert(cfg.ConfigDir)
	if err != nil {
		ln.Close()
		return nil, err
	}
	fp := FingerprintDER(cert.Certificate[0])
	n.table = NewTable(fp)
	host, _ := os.Hostname()
	if host == "" {
		host = "Tailsend"
	}
	info := Info{
		Alias: host, Version: ProtocolVersion, DeviceModel: runtime.GOOS,
		DeviceType: "desktop", Fingerprint: fp, Port: DefaultPort, Protocol: "https",
	}
	n.srv = &Server{
		Info: info, SaveDir: cfg.SaveDir, AutoSave: func() bool { return n.State().AutoSave },
		Wait: n, OnSaved: cfg.OnSaved, AcceptWait: 2 * time.Minute,
		OnPeer: func(ip string, peer Info) {
			n.table.Upsert(ip, peer, time.Now())
			n.emitPeers()
		},
	}
	tlsLn := tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}})
	n.httpSrv = &http.Server{Handler: n.srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go n.httpSrv.Serve(tlsLn)
	runCtx, cancel := context.WithCancel(ctx)
	n.cancel = cancel
	if udp != nil && listenErr == nil {
		n.udp = udp
		go ReadAnnounce(udp, n.table, info, func(ip string, port int) {
			n.registerBack(ip, port)
			n.emitPeers()
		})
		go n.announceLoop(runCtx, info)
	}
	go n.pruneLoop(runCtx)
	return n, nil
}

func (n *Node) Wait(ctx context.Context, in Incoming) (bool, error) {
	ch := make(chan bool, 1)
	n.mu.Lock()
	n.pending[in.SessionID] = ch
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.pending, in.SessionID)
		n.mu.Unlock()
	}()
	if n.cfg.OnOffer != nil {
		n.cfg.OnOffer(in)
	}
	select {
	case <-ctx.Done():
		return false, nil
	case ok := <-ch:
		return ok, nil
	}
}

func (n *Node) Accept(id string)  { n.answer(id, true) }
func (n *Node) Decline(id string) { n.answer(id, false) }

func (n *Node) answer(id string, ok bool) {
	n.mu.Lock()
	ch := n.pending[id]
	n.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ok:
	default:
	}
}

func (n *Node) SetAutoSave(v bool) error {
	n.mu.Lock()
	n.settings.AutoSave = v
	n.mu.Unlock()
	return SaveSettings(n.cfg.ConfigDir, Settings{AutoSave: v})
}

func (n *Node) State() State {
	n.mu.Lock()
	auto := n.settings.AutoSave
	n.mu.Unlock()
	peers := []Peer{}
	if n.table != nil {
		peers = n.table.List(time.Now())
	}
	if peers == nil {
		peers = []Peer{}
	}
	return State{Status: n.status, AutoSave: auto, Peers: peers}
}

func (n *Node) Send(ctx context.Context, fingerprint string, paths []string, askPIN func() (string, error), progress ProgressFunc) error {
	var target Peer
	found := false
	for _, p := range n.State().Peers {
		if p.Fingerprint == fingerprint {
			target = p
			found = true
			break
		}
	}
	if !found {
		return os.ErrNotExist
	}
	files, err := CollectFiles(paths)
	if err != nil {
		return err
	}
	return Send(ctx, n.srv.Info, Target{IP: target.IP, Port: target.Port, Fingerprint: target.Fingerprint, Alias: target.Alias}, files, askPIN, progress)
}

func (n *Node) Close() {
	if n.cancel != nil {
		n.cancel()
	}
	if n.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = n.httpSrv.Shutdown(ctx)
		cancel()
	}
	if n.udp != nil {
		_ = n.udp.Close()
	}
}

func (n *Node) announceLoop(ctx context.Context, info Info) {
	if n.udp == nil {
		return
	}
	send := func() {
		b, err := AnnounceBytes(info)
		if err != nil {
			return
		}
		dst := &net.UDPAddr{IP: net.ParseIP(MulticastIP), Port: DefaultPort}
		_, _ = n.udp.WriteToUDP(b, dst)
	}
	send()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			send()
		}
	}
}

func (n *Node) pruneLoop(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			n.emitPeers()
		}
	}
}

func (n *Node) emitPeers() {
	if n.cfg.OnPeers != nil {
		n.cfg.OnPeers(n.State().Peers)
	}
}

func (n *Node) registerBack(ip string, port int) {
	if n.srv == nil {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}}}
	body, _ := json.Marshal(n.srv.Info)
	req, err := http.NewRequest(http.MethodPost, endpoint(Target{IP: ip, Port: port}, "/api/localsend/v2/register"), bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	var info Info
	if json.NewDecoder(res.Body).Decode(&info) == nil {
		n.table.Upsert(ip, info, time.Now())
	}
}

```

`registerBack` uses `InsecureSkipVerify` because discovery only learns the peer. File sends still pin the fingerprint from the announce. A failed multicast listen leaves `udp` nil, skips announce, and does not set the port-in-use status. That status is only for TCP bind failure.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/localsend/node.go internal/localsend/node_test.go
git commit -m "feat: start the LocalSend node with the GUI lifetime"
```

---

### Task 7: Desktop wiring

**Files:**
- Create: `desktop/lan.go`
- Modify: `desktop/app.go` (`startup`, `SendTo`)
- Modify: `desktop/main.go` (`OnShutdown`)

- [ ] **Step 1: Write the failing test**

There is no Wails runtime in unit tests. Cover the busy flag with a small function in `desktop/lan.go` instead of a GUI test. Create `desktop/lan_test.go`:

```go
package main

import "testing"

func TestOutboundBusy(t *testing.T) {
	a := NewApp(nil)
	if !a.beginSend() {
		t.Fatal("first send should start")
	}
	if a.beginSend() {
		t.Fatal("second outbound send must wait")
	}
	a.endSend()
	if !a.beginSend() {
		t.Fatal("after end")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `desktop/`: `go test -count=1 -run TestOutboundBusy .`

Expected: FAIL. `beginSend` is undefined.

- [ ] **Step 3: Write minimal implementation**

Create `desktop/lan.go`:

```go
package main

import (
	"context"
	"sync/atomic"

	"github.com/sheneyan/tailsend/internal/localsend"
	"github.com/sheneyan/tailsend/internal/tsdrop"
)

func (a *App) startLAN() {
	cfgDir, err := localsend.ConfigDir()
	if err != nil {
		return
	}
	node, err := localsend.Start(a.requestCtx(), localsend.Config{
		SaveDir:   a.DefaultRecvDir(),
		ConfigDir: cfgDir,
		OnPeers: func(peers []localsend.Peer) {
			a.emit("lan-peers", peers)
		},
		OnOffer: func(in localsend.Incoming) {
			a.emit("lan-offer", in)
		},
		OnSaved: func(alias string, count int) {
			a.emit("lan-saved", map[string]any{"alias": alias, "count": count})
		},
	})
	if err != nil {
		a.emit("lan-status", err.Error())
		return
	}
	a.lan = node
}

func (a *App) shutdownLAN() {
	if a.lan != nil {
		a.lan.Close()
	}
}

func (a *App) LanState() localsend.State {
	if a.lan == nil {
		return localsend.State{Peers: []localsend.Peer{}}
	}
	return a.lan.State()
}

func (a *App) LanSetAutoSave(v bool) error {
	if a.lan == nil {
		return nil
	}
	return a.lan.SetAutoSave(v)
}

func (a *App) LanAccept(sessionID string) {
	if a.lan != nil {
		a.lan.Accept(sessionID)
	}
}

func (a *App) LanDecline(sessionID string) {
	if a.lan != nil {
		a.lan.Decline(sessionID)
	}
}

func (a *App) SendLan(fingerprint string, paths []string, pin string) error {
	if a.lan == nil {
		return errNoLAN
	}
	if !a.beginSend() {
		return errBusySend
	}
	defer a.endSend()
	var ask func() (string, error)
	if pin != "" {
		ask = func() (string, error) { return pin, nil }
	}
	return a.lan.Send(a.requestCtx(), fingerprint, paths, ask, func(name string, sent, total int64, state, errText string) {
		a.emit("progress", tsdrop.Progress{Name: name, Sent: sent, Total: total, State: state, Err: errText})
	})
}

var (
	errNoLAN    = errStr("LocalSend is not running")
	errBusySend = errStr("a send is already running")
)

type errStr string

func (e errStr) Error() string { return string(e) }

func (a *App) beginSend() bool {
	return atomic.CompareAndSwapInt32(&a.outbound, 0, 1)
}

func (a *App) endSend() {
	atomic.StoreInt32(&a.outbound, 0)
}
```

An empty `pin` passes a nil ask callback, so the first HTTP 401 becomes `ErrPINRequired`. A non-empty `pin` is sent on that one retry.

Add to `App` in `desktop/app.go`:

```go
	lan      *localsend.Node
	outbound int32
```

Declare `lan` and `outbound` on `App` in `app.go`, and import `github.com/sheneyan/tailsend/internal/localsend` in both `app.go` and `lan.go`.

In `startup`, after the existing platform hooks:

```go
	a.startLAN()
```

In `SendTo`, wrap the body:

```go
	if !a.beginSend() {
		return errBusySend
	}
	defer a.endSend()
```

Keep the existing validation before `beginSend` so a validation error does not stick the flag.

In `desktop/main.go`, set:

```go
		OnStartup:  app.startup,
		OnShutdown: app.shutdownLAN,
```

- [ ] **Step 4: Run test to verify it passes**

Run from `desktop/`: `go test -count=1 -run TestOutboundBusy .`

Expected: PASS

Run from the repo root: `go test ./internal/localsend/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/lan.go desktop/lan_test.go desktop/app.go desktop/main.go
git commit -m "feat: bind the LocalSend node into the desktop app"
```

---

### Task 8: Vue list and dialogs

**Files:**
- Modify: `desktop/frontend/src/App.vue`
- Modify: `desktop/frontend/wailsjs/go/main/App.js`

- [ ] **Step 1: Add the JS wrappers**

Append to `desktop/frontend/wailsjs/go/main/App.js`:

```js
export function LanState() {
  return window.go.main.App.LanState();
}
export function LanSetAutoSave(v) {
  return window.go.main.App.LanSetAutoSave(v);
}
export function LanAccept(sessionID) {
  return window.go.main.App.LanAccept(sessionID);
}
export function LanDecline(sessionID) {
  return window.go.main.App.LanDecline(sessionID);
}
export function SendLan(fingerprint, paths, pin) {
  return window.go.main.App.SendLan(fingerprint, paths, pin);
}
```

- [ ] **Step 2: Add the nearby block**

In `App.vue`, import the six functions next to the existing App imports.

Inside `<section class="grid-wrap">`, after the Tailscale `.grid`, add:

```html
        <div class="section-label lan-label">On this network</div>
        <p class="hint" v-if="lanStatus">{{ lanStatus }}</p>
        <label class="hint lan-check">
          <input type="checkbox" :checked="autoSave" @change="toggleAutoSave" />
          Save incoming files without asking
        </label>
        <div v-if="!lanPeers.length" class="empty">
          No LocalSend devices yet. Open LocalSend on the phone, on this Wi-Fi.
        </div>
        <div class="grid">
          <button
            v-for="p in lanPeers"
            :key="p.fingerprint"
            class="card"
            :class="{ busy: sending && sendingTo === p.fingerprint }"
            :disabled="sending"
            @click="sendLan(p)"
          >
            <span class="avatar"><OsIcon :os="p.deviceType === 'mobile' ? 'android' : 'mac'" /></span>
            <span class="name">{{ p.alias }}</span>
            <span class="meta">LocalSend</span>
          </button>
        </div>
```

`OsIcon` only knows os names already used by Tailscale (`android`, `ios`, `windows`, `mac`, `linux`). `deviceType === "mobile"` maps to `android`. Anything else maps to `mac`. That is a desktop-shaped icon, which is enough for this slice.

Add dialogs next to the Pair modal:

```html
    <div class="modal" v-if="offer">
      <div class="sheet">
        <div class="drawer-h"><strong>{{ offer.alias }} wants to send</strong></div>
        <ul class="inbox-list">
          <li v-for="f in offer.files" :key="f.id">{{ f.fileName }} <span class="muted">{{ fmtSize(f.size) }}</span></li>
        </ul>
        <div class="row-btns">
          <button class="primary" @click="acceptOffer">Save to Downloads</button>
          <button class="ghost" @click="declineOffer">Decline</button>
        </div>
      </div>
    </div>
    <div class="modal" v-if="pinAsk">
      <div class="sheet">
        <div class="drawer-h"><strong>PIN for {{ pinAsk.alias }}</strong></div>
        <input v-model="pinValue" class="pin" />
        <div class="row-btns">
          <button class="primary" @click="submitPin">Send</button>
          <button class="ghost" @click="pinAsk = null">Cancel</button>
        </div>
      </div>
    </div>
```

Script state:

```js
const lanPeers = ref([]);
const lanStatus = ref("");
const autoSave = ref(false);
const offer = ref(null);
const pinAsk = ref(null);
const pinValue = ref("");
```

`onMounted` also:

```js
  const st = await LanState();
  lanStatus.value = st.status || "";
  autoSave.value = !!st.autoSave;
  lanPeers.value = st.peers || [];
  const offPeers = EventsOn("lan-peers", (peers) => {
    lanPeers.value = peers || [];
  });
  const offOffer = EventsOn("lan-offer", (inOffer) => {
    offer.value = inOffer;
  });
  const offSaved = EventsOn("lan-saved", (p) => {
    landed.value = { title: `Saved ${p.count} file(s) from ${p.alias}`, body: "", commands: [] };
  });
```

Store `offPeers`, `offOffer`, and `offSaved` beside `offProgress`, and call them in `onUnmounted`.

```js
async function toggleAutoSave(ev) {
  autoSave.value = ev.target.checked;
  await LanSetAutoSave(autoSave.value);
}
async function acceptOffer() {
  const id = offer.value?.sessionID || offer.value?.SessionID;
  offer.value = null;
  await LanAccept(id);
}
async function declineOffer() {
  const id = offer.value?.sessionID || offer.value?.SessionID;
  offer.value = null;
  await LanDecline(id);
}
```

Task 3 tags `Incoming` as `sessionID`, `alias`, and `files`. `FileMeta` uses `fileName` and `size`. The dialog reads those names. `acceptOffer` may keep the `SessionID` fallback.

`sendLan`:

```js
async function sendLan(p) {
  if (!files.value.length) {
    await pickFiles();
  }
  if (!files.value.length) return;
  sending.value = true;
  sendingTo.value = p.fingerprint;
  landed.value = null;
  progress.value = { name: "Starting…", pct: 0, stateline: "" };
  const paths = files.value.map((f) => f.path);
  try {
    await SendLan(p.fingerprint, paths, "");
    files.value = [];
    progress.value = { name: "Sent to " + p.alias, pct: 100, stateline: "done" };
    landed.value = {
      title: `Sent to ${p.alias}`,
      body: "Their LocalSend app saves the files.",
      commands: [],
    };
  } catch (e) {
    const msg = String(e);
    if (msg.toLowerCase().includes("pin required")) {
      sending.value = false;
      pinAsk.value = { alias: p.alias, fingerprint: p.fingerprint, paths };
      pinValue.value = "";
      return;
    }
    progress.value = { name: "Failed", pct: 0, stateline: msg };
  } finally {
    if (!pinAsk.value) {
      sending.value = false;
      sendingTo.value = "";
    }
  }
}

async function submitPin() {
  const ask = pinAsk.value;
  const pin = pinValue.value;
  pinAsk.value = null;
  sending.value = true;
  sendingTo.value = ask.fingerprint;
  try {
    await SendLan(ask.fingerprint, ask.paths, pin);
    files.value = [];
    const peer = lanPeers.value.find((x) => x.fingerprint === ask.fingerprint);
    const alias = peer?.alias || ask.alias;
    progress.value = { name: "Sent to " + alias, pct: 100, stateline: "done" };
    landed.value = {
      title: `Sent to ${alias}`,
      body: "Their LocalSend app saves the files.",
      commands: [],
    };
  } catch (e) {
    progress.value = { name: "Failed", pct: 0, stateline: String(e) };
  } finally {
    sending.value = false;
    sendingTo.value = "";
  }
}
```

Add `.lan-label { margin-top: 18px; }` and `.pin { width: 100%; margin: 8px 0; padding: 8px; }` next to the other scoped styles.

The Tailscale empty copy stays inside `v-if="!sendable.length && !blocked.length"`. Do not wrap the new block in that condition.

- [ ] **Step 3: Build the frontend bindings check**

Run from `desktop/frontend`: `npm run build` if `package.json` has a build script. Otherwise run `npx vite build`.

Expected: exit 0. Vue compiles the new template.

From `desktop/`: `go test -count=1 -run TestOutboundBusy .`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add desktop/frontend/src/App.vue desktop/frontend/wailsjs/go/main/App.js internal/localsend/server.go
git commit -m "feat: show LocalSend peers in the desktop window"
```

Commit `server.go` only if the `Incoming` json tags landed there. They belong on the struct in `internal/localsend/server.go`.

---

### Task 9: Docs

**Files:**
- Modify: `README.md`, `README.zh.md`, `docs/architecture.md`, `CHANGELOG.md`

- [ ] **Step 1: Update the docs**

In `README.md`, after the comparison table and before the Taildrop interoperability paragraph, add:

```markdown
While the desktop window is open it is also a LocalSend peer on this LAN
(UDP `224.0.0.167:53317`, HTTPS port `53317`). Phones running official
LocalSend see this computer without joining Tailscale. The nearby list is
separate from the tailnet grid. Incoming files ask before they are saved
to Downloads. **Save incoming files without asking** turns that prompt off.
If port 53317 is already taken, the window says so and does not announce.
```

In `README.zh.md`, in the same place:

```markdown
桌面窗口开着时，它同时是局域网里的 LocalSend 节点（UDP `224.0.0.167:53317`，
HTTPS 端口 `53317`）。手机上的官方 LocalSend 不用加入 Tailscale 也能看见这台电脑。
这块列表和 tailnet 设备是分开的。收到的文件先询问，再存到下载目录。
勾选 **Save incoming files without asking** 后不再询问。
53317 端口已被占用时，窗口会写明，并且不会对外广播。
```

In `docs/architecture.md`, replace the sentence that says Tailsend does not listen on a Tailsend-specific port. The Taildrop path still does not. New paragraph under Discovery:

```markdown
The desktop GUI process, while open, additionally binds LocalSend's
default HTTPS port `53317` and announces on UDP `224.0.0.167:53317`.
That is a second protocol in `internal/localsend`. It does not carry
Taildrop, does not scan the LAN subnet, and stops when the window
closes. Android and the CLI do not open this port.
```

In `CHANGELOG.md` under `## Unreleased`, add:

```markdown
### Desktop LocalSend

- The open GUI is a LocalSend v2 peer on the LAN (announce, register,
  prepare-upload, upload, cancel). Tailscale discovery is unchanged.
- Incoming files prompt by default. Auto-save is a checkbox.
- Port 53317 already in use is shown. The other process is left alone.
```

- [ ] **Step 2: Commit**

```bash
git add README.md README.zh.md docs/architecture.md CHANGELOG.md
git commit -m "docs: desktop window speaks LocalSend on the LAN"
```

---

## Spec coverage

| Spec requirement | Task |
|---|---|
| Desktop-only v2 peer, Taildrop unchanged | File map, Task 7 |
| Cert fingerprint stable, autoSave default false | Task 2 |
| Announce every 5s, UDP reply, ignore self, 30s prune | Task 4, Task 6 |
| TCP bind failure does not announce; status string | Task 6 |
| v1 and v2 info, register, prepare, upload, cancel | Task 3 |
| Prompt / 2 min / 403, auto-save skips waiter, 409 | Task 3, Task 6 |
| Token 403, sha256 422 deletes file, cancel deletes, base name, `name (2)` | Task 3 |
| Pinned send, one PIN retry, flatten directory | Task 5 |
| One outbound transfer, receive stays independent | Task 7 busy flag around `SendTo` and `SendLan` only |
| Nearby UI, checkbox, dialogs, empty copy, events | Task 8 |
| Docs | Task 9 |
| No subnet scan, no download API, no Android/CLI | Not tasked |

Manual check after Task 8, on a machine where port 53317 is free: open the GUI, open LocalSend on a phone on the same Wi-Fi, confirm each side lists the other, send one file each way, decline one incoming offer, then turn on auto-save and receive without a dialog. Quit any other LocalSend on the computer first so the status line is empty.
