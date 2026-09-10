package tsdrop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"tailscale.com/tailcfg"
)

const (
	// ListPleaseName is the Taildrop command file a phone PUTs to ask the
	// desktop GUI for a fresh pairing JSON.
	ListPleaseName = "tailsend-list-please"
	// PairJSONName is the pairing snapshot PushFile'd back to the phone.
	PairJSONName = "tailsend-pair.json"
)

type listPleaseCmd struct {
	V    int      `json:"v"`
	From []string `json:"from"`
}

// HandleListPlease drains tailsend-list-please from the LocalAPI inbox and
// PushFile’s ExportTargetsJSON to the requesting sendable peer.
func (c *Client) HandleListPlease(ctx context.Context) error {
	wfs, err := c.LC.WaitingFiles(ctx)
	if err != nil {
		return mapError(err)
	}
	var first error
	for _, wf := range wfs {
		if wf.Name != ListPleaseName {
			continue
		}
		if err := c.handleListPleaseInbox(ctx, wf.Name); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *Client) handleListPleaseInbox(ctx context.Context, name string) error {
	rc, _, err := c.LC.GetWaitingFile(ctx, name)
	if err != nil {
		return mapError(err)
	}
	body, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	dest, err := c.listPleaseDest(ctx, body)
	if err != nil {
		return err
	}
	if dest == "" {
		return mapError(c.LC.DeleteWaitingFile(ctx, name))
	}
	if err := c.pushPairJSON(ctx, dest); err != nil {
		return err
	}
	return mapError(c.LC.DeleteWaitingFile(ctx, name))
}

// DrainListPleaseDir handles a tailsend-list-please sitting in dir (macOS /
// Windows Downloads) and deletes it after a reply or a no-op.
func (c *Client) DrainListPleaseDir(ctx context.Context, dir string) error {
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, ListPleaseName)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dest, err := c.listPleaseDest(ctx, body)
	if err != nil {
		return err
	}
	if dest == "" {
		return os.Remove(path)
	}
	if err := c.pushPairJSON(ctx, dest); err != nil {
		return err
	}
	return os.Remove(path)
}

func (c *Client) pushPairJSON(ctx context.Context, dest string) error {
	payload, err := c.ExportTargetsJSON(ctx)
	if err != nil {
		return err
	}
	return c.push(ctx, tailcfg.StableNodeID(dest), PairJSONName, int64(len(payload)), bytes.NewReader(payload), nil)
}

func (c *Client) listPleaseDest(ctx context.Context, body []byte) (string, error) {
	var cmd listPleaseCmd
	if err := json.Unmarshal(body, &cmd); err != nil {
		return "", nil
	}
	if len(cmd.From) == 0 {
		return "", nil
	}
	want := map[string]struct{}{}
	for _, s := range cmd.From {
		ip, err := netip.ParseAddr(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		want[ip.Unmap().String()] = struct{}{}
	}
	if len(want) == 0 {
		return "", nil
	}
	all, err := c.Targets(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range all {
		if t.Reason != "" || t.StableID == "" {
			continue
		}
		for _, ip := range t.IPs {
			if _, ok := want[ip.Unmap().String()]; ok {
				return t.StableID, nil
			}
		}
	}
	return "", nil
}
