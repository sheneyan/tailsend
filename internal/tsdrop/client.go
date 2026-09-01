package tsdrop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

// BackendState is the tailscaled run state we care about.
type BackendState int

const (
	Unknown BackendState = iota
	Stopped
	NeedsLogin
	Starting
	Running
)

func (s BackendState) String() string {
	switch s {
	case Stopped:
		return "Stopped"
	case NeedsLogin:
		return "NeedsLogin"
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	default:
		return "Unknown"
	}
}

func parseState(s string) BackendState {
	switch s {
	case "Running":
		return Running
	case "NeedsLogin":
		return NeedsLogin
	case "Starting":
		return Starting
	case "Stopped", "NoState":
		return Stopped
	default:
		return Unknown
	}
}

// Self is the local Tailscale node.
type Self struct {
	Hostname string       `json:"hostname"`
	DNSName  string       `json:"dnsName"`
	IPs      []netip.Addr `json:"ips,omitempty"`
	Online   bool         `json:"online"`
	OS       string       `json:"os"`
}

// Target is a tailnet peer that might accept a Taildrop.
type Target struct {
	StableID   string       `json:"stableID"`
	Hostname   string       `json:"hostname"`
	DNSName    string       `json:"dnsName"`
	IPs        []netip.Addr `json:"ips,omitempty"`
	OS         string       `json:"os"`
	Online     bool         `json:"online"`
	PeerAPIURL string       `json:"peerAPIURL,omitempty"`
	Reason     string       `json:"reason,omitempty"`
}

// WaitingFile is a file sitting in the local Taildrop inbox.
type WaitingFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Progress is a send update.
type Progress struct {
	Name   string `json:"name"`
	PeerID string `json:"peerID"`
	Sent   int64  `json:"sent"`
	Total  int64  `json:"total"`
	State  string `json:"state"`
	Err    string `json:"err,omitempty"`
}

// SendItem is a local path to send.
type SendItem struct {
	Path string
	Name string
	Size int64
}

// Conflict is how Receive treats an existing dest file.
type Conflict int

const (
	ConflictSkip Conflict = iota
	ConflictOverwrite
	ConflictRename
)

func ParseConflict(s string) (Conflict, error) {
	switch s {
	case "", "skip":
		return ConflictSkip, nil
	case "overwrite":
		return ConflictOverwrite, nil
	case "rename":
		return ConflictRename, nil
	default:
		return 0, fmt.Errorf("conflict must be skip|overwrite|rename, got %q", s)
	}
}

// Client talks to tailscaled LocalAPI.
type Client struct {
	LC *local.Client
}

// New wraps a LocalAPI client. A nil lc uses the platform default socket.
func New(lc *local.Client) *Client {
	if lc == nil {
		lc = &local.Client{}
	}
	return &Client{LC: lc}
}

// Probe reports whether tailscaled is up and who we are.
func (c *Client) Probe(ctx context.Context) (BackendState, Self, error) {
	st, err := c.LC.Status(ctx)
	if err != nil {
		return Unknown, Self{}, mapError(err)
	}
	self := Self{}
	if st.Self != nil {
		self = selfFromPeer(st.Self)
	}
	return parseState(st.BackendState), self, nil
}

func selfFromPeer(p *ipnstate.PeerStatus) Self {
	return Self{
		Hostname: p.HostName,
		DNSName:  trimDot(p.DNSName),
		IPs:      p.TailscaleIPs,
		Online:   p.Online,
		OS:       p.OS,
	}
}

func trimDot(s string) string {
	return strings.TrimSuffix(s, ".")
}

// Targets lists tailnet peers and why they can or cannot receive files.
func (c *Client) Targets(ctx context.Context) ([]Target, error) {
	if _, err := c.LC.FileTargets(ctx); err != nil {
		mapped := mapError(err)
		if errors.Is(mapped, ErrFileSharingDisabled) || errors.Is(mapped, ErrPermission) {
			if errors.Is(mapped, ErrPermission) {
				return nil, fmt.Errorf("%w: %v", ErrFileSharingDisabled, mapped)
			}
			return nil, mapped
		}
	}
	st, err := c.LC.Status(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	var out []Target
	for _, p := range st.Peer {
		if p == nil || p.ShareeNode {
			continue
		}
		out = append(out, targetFromPeer(p))
	}
	slices.SortFunc(out, func(a, b Target) int {
		an := strings.ToLower(strings.TrimSpace(a.Hostname))
		bn := strings.ToLower(strings.TrimSpace(b.Hostname))
		if an == "" {
			an = strings.ToLower(a.DNSName)
		}
		if bn == "" {
			bn = strings.ToLower(b.DNSName)
		}
		if c := strings.Compare(an, bn); c != 0 {
			return c
		}
		return strings.Compare(a.StableID, b.StableID)
	})
	return out, nil
}

func targetFromPeer(p *ipnstate.PeerStatus) Target {
	peerAPI := ""
	if len(p.PeerAPIURL) > 0 {
		peerAPI = p.PeerAPIURL[0]
	}
	return Target{
		StableID:   string(p.ID),
		Hostname:   p.HostName,
		DNSName:    trimDot(p.DNSName),
		IPs:        p.TailscaleIPs,
		OS:         p.OS,
		Online:     p.Online,
		PeerAPIURL: peerAPI,
		Reason:     reasonFor(p),
	}
}

func reasonFor(p *ipnstate.PeerStatus) string {
	if p.TaildropTarget == ipnstate.TaildropTargetAvailable {
		return ""
	}
	if p.NoFileSharingReason != "" {
		return p.NoFileSharingReason
	}
	switch p.TaildropTarget {
	case ipnstate.TaildropTargetOffline:
		return "offline"
	case ipnstate.TaildropTargetOwnedByOtherUser:
		return "owned by another user"
	case ipnstate.TaildropTargetMissingCap:
		return "file sharing not enabled"
	case ipnstate.TaildropTargetNoPeerAPI:
		return "no PeerAPI"
	case ipnstate.TaildropTargetUnsupportedOS:
		return "unsupported OS"
	case ipnstate.TaildropTargetIpnStateNotRunning:
		return "Tailscale not running"
	case ipnstate.TaildropTargetNoPeerInfo:
		return "no peer info"
	default:
		if !p.Online {
			return "offline"
		}
		return "not a Taildrop target"
	}
}

// ExportTargetsJSON is the mobile pairing payload: sendable targets only.
func (c *Client) ExportTargetsJSON(ctx context.Context) ([]byte, error) {
	all, err := c.Targets(ctx)
	if err != nil {
		return nil, err
	}
	var sendable []Target
	for _, t := range all {
		if t.Reason == "" {
			sendable = append(sendable, t)
		}
	}
	if sendable == nil {
		sendable = []Target{}
	}
	return json.MarshalIndent(sendable, "", "  ")
}

// Lookup finds a target by hostname, MagicDNS, StableID, or Tailscale IP.
func (c *Client) Lookup(ctx context.Context, name string) (Target, error) {
	name = strings.TrimSuffix(name, ":")
	name = strings.TrimSuffix(name, ".")
	want := strings.ToLower(name)
	all, err := c.Targets(ctx)
	if err != nil {
		return Target{}, err
	}
	var found *Target
	for i := range all {
		t := &all[i]
		if strings.EqualFold(t.StableID, name) ||
			strings.EqualFold(t.Hostname, name) ||
			strings.EqualFold(t.DNSName, name) ||
			strings.EqualFold(strings.Split(t.DNSName, ".")[0], name) {
			found = t
			break
		}
		for _, ip := range t.IPs {
			if ip.String() == want {
				found = t
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		return Target{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if found.Reason != "" {
		return *found, fmt.Errorf("%s: %s", found.Hostname, found.Reason)
	}
	return *found, nil
}

// Send pushes files or directories (zipped) to dest's StableNodeID.
func (c *Client) Send(ctx context.Context, dest string, items []SendItem, onProgress func(Progress)) error {
	id := tailcfg.StableNodeID(dest)
	for _, item := range items {
		if err := c.sendOne(ctx, id, item, onProgress); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) sendOne(ctx context.Context, dest tailcfg.StableNodeID, item SendItem, onProgress func(Progress)) error {
	info, err := os.Stat(item.Path)
	if err != nil {
		return err
	}
	name := item.Name
	if name == "" {
		name = info.Name()
	}
	if info.IsDir() {
		return c.sendDir(ctx, dest, item.Path, name+".zip", onProgress)
	}
	f, err := os.Open(item.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	size := info.Size()
	if item.Size != 0 {
		size = item.Size
	}
	return c.push(ctx, dest, name, size, f, onProgress)
}

func (c *Client) sendDir(ctx context.Context, dest tailcfg.StableNodeID, dir, zipName string, onProgress func(Progress)) error {
	pr, pw := io.Pipe()
	errc := make(chan error, 1)
	go func() {
		errc <- pw.CloseWithError(writeZip(pw, dir))
	}()
	pushErr := c.push(ctx, dest, zipName, -1, pr, onProgress)
	zipErr := <-errc
	if pushErr != nil {
		return pushErr
	}
	return zipErr
}

func (c *Client) push(ctx context.Context, dest tailcfg.StableNodeID, name string, size int64, r io.Reader, onProgress func(Progress)) error {
	cr := &countingReader{r: r, total: size, name: name, peer: string(dest), fn: onProgress}
	if onProgress != nil {
		onProgress(Progress{Name: name, PeerID: string(dest), Total: size, State: "starting"})
	}
	err := c.LC.PushFile(ctx, dest, size, name, cr)
	if err != nil {
		if onProgress != nil {
			onProgress(Progress{Name: name, PeerID: string(dest), Sent: cr.n.Load(), Total: size, State: "failed", Err: err.Error()})
		}
		return mapError(err)
	}
	sent := cr.n.Load()
	if size >= 0 {
		sent = size
	}
	if onProgress != nil {
		onProgress(Progress{Name: name, PeerID: string(dest), Sent: sent, Total: size, State: "done"})
	}
	return nil
}

type countingReader struct {
	r     io.Reader
	n     atomic.Int64
	total int64
	name  string
	peer  string
	fn    func(Progress)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		sent := c.n.Add(int64(n))
		if c.fn != nil {
			c.fn(Progress{Name: c.name, PeerID: c.peer, Sent: sent, Total: c.total, State: "running"})
		}
	}
	return n, err
}

// Inbox lists files waiting in the daemon staging directory.
func (c *Client) Inbox(ctx context.Context) ([]WaitingFile, error) {
	wfs, err := c.LC.WaitingFiles(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return convertWaiting(wfs), nil
}

func convertWaiting(wfs []apitype.WaitingFile) []WaitingFile {
	out := make([]WaitingFile, 0, len(wfs))
	for _, wf := range wfs {
		out = append(out, WaitingFile{Name: wf.Name, Size: wf.Size})
	}
	return out
}

// Receive copies inbox files into destDir and deletes them from the inbox.
func (c *Client) Receive(ctx context.Context, destDir string, conflict Conflict) ([]string, error) {
	wfs, err := c.LC.WaitingFiles(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	var errs []error
	for _, wf := range wfs {
		path, err := c.receiveOne(ctx, destDir, wf, conflict)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if path != "" {
			written = append(written, path)
		}
	}
	if len(errs) > 0 {
		return written, errs[0]
	}
	return written, nil
}

func (c *Client) receiveOne(ctx context.Context, destDir string, wf apitype.WaitingFile, conflict Conflict) (string, error) {
	base := filepath.Base(wf.Name)
	if base != wf.Name || base == "." || base == string(filepath.Separator) {
		return "", fmt.Errorf("unsafe inbox name %q", wf.Name)
	}
	dest := filepath.Join(destDir, base)
	if _, err := os.Stat(dest); err == nil {
		switch conflict {
		case ConflictSkip:
			return "", fmt.Errorf("skip existing %s", base)
		case ConflictOverwrite:
			// keep dest
		case ConflictRename:
			dest = filepath.Join(destDir, uniqueName(destDir, base))
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	rc, _, err := c.LC.GetWaitingFile(ctx, wf.Name)
	if err != nil {
		return "", mapError(err)
	}
	defer rc.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(f, rc)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(dest)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(dest)
		return "", closeErr
	}
	if err := c.LC.DeleteWaitingFile(ctx, wf.Name); err != nil {
		return dest, mapError(err)
	}
	return dest, nil
}

func uniqueName(dir, base string) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(dir, cand)); os.IsNotExist(err) {
			return cand
		}
	}
}

// WatchInbox polls the inbox until ctx is done.
func (c *Client) WatchInbox(ctx context.Context) (<-chan []WaitingFile, error) {
	ch := make(chan []WaitingFile, 1)
	go func() {
		defer close(ch)
		for {
			files, err := c.LC.AwaitWaitingFiles(ctx, time.Second)
			if err != nil {
				return
			}
			select {
			case ch <- convertWaiting(files):
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
