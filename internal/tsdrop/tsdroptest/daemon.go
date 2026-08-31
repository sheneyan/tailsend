// Package tsdroptest is a fake Tailscale LocalAPI for tests.
package tsdroptest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

// Daemon is an in-memory LocalAPI.
type Daemon struct {
	mu sync.Mutex

	BackendState        string
	Self                *ipnstate.PeerStatus
	Peers               []*ipnstate.PeerStatus
	FileSharingDisabled bool
	DialErr             bool
	PutErr              string
	Inbox               map[string][]byte
	Puts                []Put
}

// Put is one recorded file-put.
type Put struct {
	Target string
	Name   string
	Body   []byte
}

// LocalClient returns a local.Client talking to this daemon over HTTP.
func (d *Daemon) LocalClient(t *testing.T) *local.Client {
	t.Helper()
	d.mu.Lock()
	if d.Inbox == nil {
		d.Inbox = map[string][]byte{}
	}
	d.mu.Unlock()
	srv := httptest.NewServer(http.HandlerFunc(d.serve))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	return &local.Client{
		OmitAuth: true,
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d.mu.Lock()
			fail := d.DialErr
			d.mu.Unlock()
			if fail {
				return nil, &net.OpError{Op: "dial", Net: network, Addr: dummyAddr(addr), Err: io.EOF}
			}
			var nd net.Dialer
			return nd.DialContext(ctx, "tcp", host)
		},
	}
}

type dummyAddr string

func (a dummyAddr) Network() string { return "tcp" }
func (a dummyAddr) String() string  { return string(a) }

func (d *Daemon) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/localapi/v0/status":
		d.writeStatus(w)
	case path == "/localapi/v0/file-targets":
		d.writeFileTargets(w)
	case strings.HasPrefix(path, "/localapi/v0/file-put/"):
		d.handlePut(w, r, strings.TrimPrefix(path, "/localapi/v0/file-put/"))
	case path == "/localapi/v0/files/" || path == "/localapi/v0/files":
		d.writeInbox(w)
	case strings.HasPrefix(path, "/localapi/v0/files/"):
		d.handleFile(w, r, strings.TrimPrefix(path, "/localapi/v0/files/"))
	default:
		http.NotFound(w, r)
	}
}

func (d *Daemon) writeStatus(w http.ResponseWriter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := &ipnstate.Status{
		BackendState: d.BackendState,
		Self:         d.Self,
		Peer:         map[key.NodePublic]*ipnstate.PeerStatus{},
	}
	for _, p := range d.Peers {
		k := p.PublicKey
		if k.IsZero() {
			k = key.NewNode().Public()
			p.PublicKey = k
		}
		st.Peer[k] = p
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

func (d *Daemon) writeFileTargets(w http.ResponseWriter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.FileSharingDisabled {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"error":"file sharing not enabled by Tailscale admin"}`)
		return
	}
	var fts []apitype.FileTarget
	for _, p := range d.Peers {
		if p.TaildropTarget != ipnstate.TaildropTargetAvailable {
			continue
		}
		peerAPI := ""
		if len(p.PeerAPIURL) > 0 {
			peerAPI = p.PeerAPIURL[0]
		}
		fts = append(fts, apitype.FileTarget{
			Node: &tailcfg.Node{
				StableID: p.ID,
				Name:     p.DNSName,
			},
			PeerAPIURL: peerAPI,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fts)
}

func (d *Daemon) handlePut(w http.ResponseWriter, r *http.Request, rest string) {
	target, name, _ := strings.Cut(rest, "/")
	name, _ = url.PathUnescape(name)
	body, _ := io.ReadAll(r.Body)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.PutErr != "" {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":%q}`, d.PutErr)
		return
	}
	d.Puts = append(d.Puts, Put{Target: target, Name: name, Body: body})
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) writeInbox(w http.ResponseWriter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	files := []apitype.WaitingFile{}
	for name, body := range d.Inbox {
		files = append(files, apitype.WaitingFile{Name: name, Size: int64(len(body))})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (d *Daemon) handleFile(w http.ResponseWriter, r *http.Request, name string) {
	name, _ = url.PathUnescape(name)
	switch r.Method {
	case http.MethodGet:
		d.mu.Lock()
		body, ok := d.Inbox[name]
		d.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.Write(body)
	case http.MethodDelete:
		d.mu.Lock()
		delete(d.Inbox, name)
		d.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
