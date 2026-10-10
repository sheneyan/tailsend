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
	if bytes.ContainsAny([]byte(got), "ABCDEF") {
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
		"a.png":      "a.png",
		"../a.png":   "a.png",
		`dir\a.png`:  "a.png",
		"/tmp/a.png": "a.png",
		"..":         "file",
		"":           "file",
		"   ":        "file",
	}
	for in, want := range cases {
		if got := SafeBase(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}

func hexEncode(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = digits[c>>4]
		out[i*2+1] = digits[c&0x0f]
	}
	return string(out)
}
