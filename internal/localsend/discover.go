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
