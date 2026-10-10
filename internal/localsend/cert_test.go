package localsend

import "testing"

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
