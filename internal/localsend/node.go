package localsend

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	status    string
	cfg       Config
	settings  Settings
	table     *Table
	srv       *Server
	httpSrv   *http.Server
	udp       *net.UDPConn
	runCtx    context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

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
	runCtx, cancel := context.WithCancel(ctx)
	n.runCtx = runCtx
	n.cancel = cancel
	go n.httpSrv.Serve(tlsLn)
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
	var nodeDone <-chan struct{}
	if n.runCtx != nil {
		nodeDone = n.runCtx.Done()
	}
	select {
	case <-ctx.Done():
		return false, nil
	case <-nodeDone:
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
	if n.srv == nil {
		return errors.New("localsend is not listening")
	}
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
	n.closeOnce.Do(func() {
		if n.cancel != nil {
			n.cancel()
		}
		if n.httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := n.httpSrv.Shutdown(ctx)
			cancel()
			if err != nil {
				_ = n.httpSrv.Close()
			}
		}
		if n.udp != nil {
			_ = n.udp.Close()
		}
	})
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

// registerBack learns a peer over HTTPS without pinning. File sends pin inside Send.
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
