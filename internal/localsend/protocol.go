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
