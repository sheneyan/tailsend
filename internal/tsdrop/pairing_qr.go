package tsdrop

import (
	"context"
	"encoding/json"
)

// PairingQRMaxBytes keeps a camera-scannable QR version when shown on a
// screen. Version 15-L is 523 bytes; stay under that.
const PairingQRMaxBytes = 500

type qrTarget struct {
	StableID   string `json:"stableID,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	OS         string `json:"os,omitempty"`
	PeerAPIURL string `json:"peerAPIURL"`
}

func (c *Client) pairingTargets(ctx context.Context) ([]Target, error) {
	all, err := c.Targets(ctx)
	if err != nil {
		return nil, err
	}
	var sendable []Target
	seen := map[string]struct{}{}
	if self, ok, err := c.pairingSelf(ctx); err != nil {
		return nil, err
	} else if ok {
		sendable = append(sendable, self)
		if self.StableID != "" {
			seen[self.StableID] = struct{}{}
		}
	}
	for _, t := range all {
		if t.Reason != "" {
			continue
		}
		if t.StableID != "" {
			if _, dup := seen[t.StableID]; dup {
				continue
			}
			seen[t.StableID] = struct{}{}
		}
		sendable = append(sendable, t)
	}
	if sendable == nil {
		sendable = []Target{}
	}
	return sendable, nil
}

// ExportTargetsQRJSON is a minified pairing snapshot for QR encoding:
// hostname/os/stableID/peerAPIURL only, truncated to PairingQRMaxBytes
// (this computer first).
func (c *Client) ExportTargetsQRJSON(ctx context.Context) ([]byte, error) {
	ts, err := c.pairingTargets(ctx)
	if err != nil {
		return nil, err
	}
	return CompactPairingQRJSON(ts, PairingQRMaxBytes)
}

// CompactPairingQRJSON minifies targets and drops overflow so a screen QR
// stays readable. The first target is always kept.
func CompactPairingQRJSON(targets []Target, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = PairingQRMaxBytes
	}
	var out []qrTarget
	for _, t := range targets {
		if t.PeerAPIURL == "" {
			continue
		}
		cand := append(out, qrTarget{
			StableID:   t.StableID,
			Hostname:   t.Hostname,
			OS:         t.OS,
			PeerAPIURL: t.PeerAPIURL,
		})
		b, err := json.Marshal(cand)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 && len(b) > maxBytes {
			break
		}
		out = cand
	}
	if out == nil {
		out = []qrTarget{}
	}
	return json.Marshal(out)
}
