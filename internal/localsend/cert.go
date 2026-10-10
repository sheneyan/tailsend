package localsend

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	AutoSave bool `json:"autoSave"`
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "tailsend")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func LoadSettings(dir string) (Settings, error) {
	b, err := os.ReadFile(filepath.Join(dir, "localsend.json"))
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func SaveSettings(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return replaceFile(dir, "localsend.json", b, 0o600)
}

func LoadOrCreateCert(dir string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, "localsend-cert.pem")
	keyPath := filepath.Join(dir, "localsend-key.pem")
	certExists, err := fileExists(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyExists, err := fileExists(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	if certExists || keyExists {
		return tls.LoadX509KeyPair(certPath, keyPath)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Tailsend"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		DNSNames:     []string{"tailsend.local"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyTmp, err := writeTemp(dir, keyPEM, 0o600)
	if err != nil {
		return tls.Certificate{}, err
	}
	certTmp, err := writeTemp(dir, certPEM, 0o644)
	if err != nil {
		os.Remove(keyTmp)
		return tls.Certificate{}, err
	}
	if err := os.Rename(keyTmp, keyPath); err != nil {
		os.Remove(keyTmp)
		os.Remove(certTmp)
		return tls.Certificate{}, err
	}
	if err := os.Rename(certTmp, certPath); err != nil {
		os.Remove(certTmp)
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func replaceFile(dir, name string, data []byte, mode os.FileMode) error {
	tmp, err := writeTemp(dir, data, mode)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func writeTemp(dir string, data []byte, mode os.FileMode) (string, error) {
	f, err := os.CreateTemp(dir, ".localsend-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
