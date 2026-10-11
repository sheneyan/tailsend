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
	peers := tab.List(now.Add(29 * time.Second))
	if len(peers) != 1 || peers[0].Alias != "Phone" || peers[0].IP != "192.0.2.2" || peers[0].Port != 53317 {
		t.Fatalf("%+v", peers)
	}
	peers = tab.List(now.Add(30 * time.Second))
	if len(peers) != 0 {
		t.Fatalf("still listed after 30s: %+v", peers)
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
