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
	IdleWait   time.Duration

	mu       sync.Mutex
	session  *recvSession
	finished map[string][]string
}

type recvSession struct {
	id         string
	alias      string
	tokens     map[string]string
	metas      map[string]FileMeta
	paths      map[string]string
	busy       map[string]bool
	got        int
	idle       *time.Timer
	idleGen    int
	copyCancel context.CancelFunc
	copyGen    int
	body       io.ReadCloser
}

func (s *Server) clearSession() {
	s.mu.Lock()
	s.dropSessionLocked(s.session)
	s.session = nil
	s.mu.Unlock()
}

func (s *Server) idleDuration() time.Duration {
	if s.IdleWait > 0 {
		return s.IdleWait
	}
	return 2 * time.Minute
}

// dropSessionLocked invalidates an idle timer. Caller holds s.mu.
func (s *Server) dropSessionLocked(sess *recvSession) {
	if sess == nil {
		return
	}
	sess.idleGen++
	if sess.idle != nil {
		sess.idle.Stop()
		sess.idle = nil
	}
}

// armIdleLocked starts or resets the abandoned-session timer. Caller holds s.mu.
func (s *Server) armIdleLocked(sess *recvSession) {
	s.dropSessionLocked(sess)
	gen := sess.idleGen
	sess.idle = time.AfterFunc(s.idleDuration(), func() {
		s.expireIdle(sess, gen)
	})
}

// expireIdle drops sess when a full IdleWait passed with no new bytes.
func (s *Server) expireIdle(sess *recvSession, gen int) {
	s.mu.Lock()
	if s.session != sess || sess.idleGen != gen {
		s.mu.Unlock()
		return
	}
	paths := make([]string, len(sess.paths))
	i := 0
	for _, p := range sess.paths {
		paths[i] = p
		i++
	}
	cancel, body := sess.copyCancel, sess.body
	sess.copyGen++
	sess.copyCancel = nil
	sess.body = nil
	s.dropSessionLocked(sess)
	s.session = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if body != nil {
		_ = body.Close()
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

func (s *Server) noteActivity(sess *recvSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == sess {
		s.armIdleLocked(sess)
	}
}

func (s *Server) clearBusy(sess *recvSession, fileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == sess {
		delete(sess.busy, fileID)
	}
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
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&info); err != nil {
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
	for _, meta := range req.Files {
		if meta.Size < 0 {
			http.Error(w, "bad size", http.StatusBadRequest)
			return
		}
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
		busy:   map[string]bool{},
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
	if !accept || r.Context().Err() != nil {
		s.clearSession()
		http.Error(w, "rejected", http.StatusForbidden)
		return
	}
	resp := PrepareResponse{SessionID: sess.id, Files: map[string]string{}}
	s.mu.Lock()
	if s.session != sess {
		s.mu.Unlock()
		http.Error(w, "rejected", http.StatusForbidden)
		return
	}
	for _, meta := range files {
		tok := randomID()
		sess.tokens[meta.ID] = tok
		sess.metas[meta.ID] = meta
		resp.Files[meta.ID] = tok
	}
	s.armIdleLocked(sess)
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
	if sess.busy[fileID] || sess.paths[fileID] != "" {
		s.mu.Unlock()
		http.Error(w, "duplicate", http.StatusConflict)
		return
	}
	sess.busy[fileID] = true
	meta := sess.metas[fileID]
	s.mu.Unlock()

	f, dest, err := createDest(s.SaveDir, meta.FileName)
	if err != nil {
		s.clearBusy(sess, fileID)
		s.failUpload(w)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	s.mu.Lock()
	if s.session != sess {
		s.mu.Unlock()
		cancel()
		_ = f.Close()
		_ = os.Remove(dest)
		http.Error(w, "canceled", http.StatusForbidden)
		return
	}
	sess.copyGen++
	copyGen := sess.copyGen
	sess.copyCancel = cancel
	sess.body = r.Body
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if sess.copyGen == copyGen {
			sess.copyCancel = nil
			sess.body = nil
		}
		s.mu.Unlock()
		cancel()
	}()
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(&activityReader{ctx: ctx, r: r.Body, s: s, sess: sess}, meta.Size))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dest)
		s.mu.Lock()
		canceled := s.session != sess
		if !canceled {
			delete(sess.busy, fileID)
		}
		s.mu.Unlock()
		if canceled {
			http.Error(w, "canceled", http.StatusForbidden)
			return
		}
		s.failUpload(w)
		return
	}
	extra := make([]byte, 1)
	m, readErr := r.Body.Read(extra)
	if readErr != nil && readErr != io.EOF && m == 0 {
		os.Remove(dest)
		s.clearBusy(sess, fileID)
		s.failUpload(w)
		return
	}
	sum := hex.EncodeToString(h.Sum(nil))
	s.mu.Lock()
	if s.session != sess {
		s.mu.Unlock()
		os.Remove(dest)
		http.Error(w, "canceled", http.StatusForbidden)
		return
	}
	if n != meta.Size || m > 0 {
		delete(sess.busy, fileID)
		s.mu.Unlock()
		os.Remove(dest)
		http.Error(w, "size", http.StatusBadRequest)
		return
	}
	if meta.SHA256 != "" && !strings.EqualFold(meta.SHA256, sum) {
		delete(sess.busy, fileID)
		s.mu.Unlock()
		os.Remove(dest)
		http.Error(w, "checksum", http.StatusUnprocessableEntity)
		return
	}
	delete(sess.busy, fileID)
	if sess.paths[fileID] == "" {
		sess.paths[fileID] = dest
		sess.got++
	}
	done := sess.got == len(sess.metas)
	alias, count := sess.alias, sess.got
	if done {
		if s.finished == nil {
			s.finished = map[string][]string{}
		}
		paths := make([]string, 0, len(sess.paths))
		for _, p := range sess.paths {
			paths = append(paths, p)
		}
		s.finished[sess.id] = paths
		s.dropSessionLocked(sess)
		s.session = nil
	} else {
		s.armIdleLocked(sess)
	}
	s.mu.Unlock()
	if done && s.OnSaved != nil {
		s.OnSaved(alias, count)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) failUpload(w http.ResponseWriter) {
	http.Error(w, "write", http.StatusInternalServerError)
}

// activityReader resets the idle clock only when a Read returns bytes.
// A canceled context ends the copy after IdleWait with no new bytes.
type activityReader struct {
	ctx  context.Context
	r    io.Reader
	s    *Server
	sess *recvSession
}

func (a *activityReader) Read(p []byte) (int, error) {
	if err := a.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := a.r.Read(p)
	if n > 0 {
		a.s.noteActivity(a.sess)
	}
	if err == nil {
		if cerr := a.ctx.Err(); cerr != nil {
			return n, cerr
		}
	}
	return n, err
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
		s.dropSessionLocked(sess)
		s.session = nil
	}
	for _, p := range s.finished[id] {
		os.Remove(p)
	}
	delete(s.finished, id)
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func createDest(dir, fileName string) (*os.File, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
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
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return f, p, nil
	}
	return nil, "", errors.New("no free name")
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
