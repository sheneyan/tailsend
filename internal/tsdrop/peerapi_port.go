package tsdrop

import (
	"hash/crc32"
	"net/netip"
)

// PeerAPIPorts are the first five deterministic ports Tailscale tries for
// PeerAPI on ip: (32<<10) | crc32 of the last three As16() bytes, with the
// first hash byte incremented per try. After that the daemon may bind a
// random port we cannot guess.
func PeerAPIPorts(ip netip.Addr) []uint16 {
	if !ip.IsValid() {
		return nil
	}
	a16 := ip.As16()
	out := make([]uint16, 0, 5)
	for try := uint8(0); try < 5; try++ {
		hashData := [3]byte{a16[13], a16[14], a16[15]}
		hashData[0] += try
		port := uint16(32<<10) | uint16(crc32.ChecksumIEEE(hashData[:]))
		out = append(out, port)
	}
	return out
}
