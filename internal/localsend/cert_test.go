package localsend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateCertStableFingerprint(t *testing.T) {
	dir := t.TempDir()
	c1, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	f1 := FingerprintDER(c1.Certificate[0])
	f2 := FingerprintDER(c2.Certificate[0])
	if f1 == "" || f1 != f2 {
		t.Fatalf("fingerprint changed: %s %s", f1, f2)
	}
}

func TestSettingsDefaultAutoSaveOff(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.AutoSave {
		t.Fatal("default autoSave must be false")
	}
	if err := SaveSettings(dir, Settings{AutoSave: true}); err != nil {
		t.Fatal(err)
	}
	s, err = LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s.AutoSave {
		t.Fatal("autoSave not persisted")
	}
}

func TestLoadOrCreateCertLeavesPartialCert(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "localsend-cert.pem")
	garbage := []byte("not a certificate\n")
	if err := os.WriteFile(certPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateCert(dir); err == nil {
		t.Fatal("expected error when cert exists without a key")
	}
	got, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(garbage) {
		t.Fatalf("cert bytes changed: %q", got)
	}
}

func TestLoadOrCreateCertKeyMode(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadOrCreateCert(dir); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "localsend-key.pem")
	certPath := filepath.Join(dir, "localsend-cert.pem")
	st, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&0o777 != 0o600 {
		t.Fatalf("key mode %o, want 0600", st.Mode()&0o777)
	}
	if _, err := os.Stat(certPath); err != nil {
		t.Fatal(err)
	}
}
