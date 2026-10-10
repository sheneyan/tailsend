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

	mu       sync.Mutex
	session  *recvSession
	finished map[string][]string
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
			if s.finished == nil {
				s.finished = map[string][]string{}
			}
			paths := make([]string, 0, len(sess.paths))
			for _, p := range sess.paths {
				paths = append(paths, p)
			}
			s.finished[sess.id] = paths
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
	for _, p := range s.finished[id] {
		os.Remove(p)
	}
	delete(s.finished, id)
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
