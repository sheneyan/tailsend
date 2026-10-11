package localsend

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrCertMismatch = errors.New("certificate does not match")
	ErrPINRequired  = errors.New("pin required")
	ErrInvalidPIN   = errors.New("invalid pin")
	ErrRejected     = errors.New("rejected")
	ErrBusy         = errors.New("peer is busy")
)

type FileSend struct {
	Path string
	Name string
}

type Target struct {
	IP          string
	Port        int
	Fingerprint string
	Alias       string
}

type ProgressFunc func(name string, sent, total int64, state, errText string)

func CollectFiles(paths []string) ([]FileSend, error) {
	var out []FileSend
	seen := map[string]int{}
	add := func(disk, name string) {
		name = SafeBase(name)
		n := seen[name]
		seen[name]++
		if n > 0 {
			ext := filepath.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			name = stem + " (" + strconv.Itoa(n+1) + ")" + ext
		}
		out = append(out, FileSend{Path: disk, Name: name})
	}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			if !st.Mode().IsRegular() {
				continue
			}
			add(p, st.Name())
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			add(path, d.Name())
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func Send(ctx context.Context, local Info, target Target, files []FileSend, askPIN func() (string, error), progress ProgressFunc) error {
	if len(files) == 0 {
		return errors.New("no files selected")
	}
	client, err := pinnedClient(target.Fingerprint)
	if err != nil {
		return err
	}
	pin := ""
	code, resp, err := prepareUpload(ctx, client, local, target, files, pin)
	if err != nil {
		return err
	}
	if code == http.StatusUnauthorized {
		if askPIN == nil {
			return ErrPINRequired
		}
		pin, err = askPIN()
		if err != nil {
			return err
		}
		code, resp, err = prepareUpload(ctx, client, local, target, files, pin)
		if err != nil {
			return err
		}
		if code == http.StatusUnauthorized {
			return ErrInvalidPIN
		}
	}
	switch code {
	case http.StatusForbidden:
		return ErrRejected
	case http.StatusConflict:
		return ErrBusy
	case http.StatusOK:
	default:
		return errors.New("prepare failed")
	}
	var total int64
	for _, f := range files {
		st, err := os.Stat(f.Path)
		if err != nil {
			return err
		}
		total += st.Size()
	}
	var sent int64
	for _, f := range files {
		token := resp.Files[f.Name]
		if token == "" {
			return errors.New("missing token")
		}
		body, err := os.Open(f.Path)
		if err != nil {
			return err
		}
		st, err := body.Stat()
		if err != nil {
			body.Close()
			return err
		}
		q := url.Values{}
		q.Set("sessionId", resp.SessionID)
		q.Set("fileId", f.Name)
		q.Set("token", token)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(target, "/api/localsend/v2/upload")+"?"+q.Encode(), body)
		if err != nil {
			body.Close()
			return err
		}
		req.ContentLength = st.Size()
		if progress != nil {
			progress(f.Name, sent, total, "sending", "")
		}
		res, err := client.Do(req)
		body.Close()
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("upload failed")
		}
		sent += st.Size()
	}
	if progress != nil {
		progress(target.Alias, sent, total, "done", "")
	}
	return nil
}

func prepareUpload(ctx context.Context, client *http.Client, local Info, target Target, files []FileSend, pin string) (int, PrepareResponse, error) {
	filesJSON := map[string]FileMeta{}
	for _, f := range files {
		st, err := os.Stat(f.Path)
		if err != nil {
			return 0, PrepareResponse{}, err
		}
		filesJSON[f.Name] = FileMeta{ID: f.Name, FileName: f.Name, Size: st.Size()}
	}
	local.Announce = nil
	payload := PrepareRequest{Info: local, Files: filesJSON}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, PrepareResponse{}, err
	}
	u := endpoint(target, "/api/localsend/v2/prepare-upload")
	if pin != "" {
		u += "?pin=" + url.QueryEscape(pin)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return 0, PrepareResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrCertMismatch) || strings.Contains(err.Error(), ErrCertMismatch.Error()) {
			return 0, PrepareResponse{}, ErrCertMismatch
		}
		return 0, PrepareResponse{}, err
	}
	defer res.Body.Close()
	var resp PrepareResponse
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
			return 0, PrepareResponse{}, err
		}
	}
	return res.StatusCode, resp, nil
}

func endpoint(target Target, path string) string {
	return "https://" + net.JoinHostPort(target.IP, strconv.Itoa(target.Port)) + path
}

func pinnedClient(fingerprint string) (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
				VerifyConnection: func(cs tls.ConnectionState) error {
					if len(cs.PeerCertificates) == 0 {
						return ErrCertMismatch
					}
					if FingerprintDER(cs.PeerCertificates[0].Raw) != fingerprint {
						return ErrCertMismatch
					}
					return nil
				},
			},
		},
	}, nil
}
