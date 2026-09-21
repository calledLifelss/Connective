package update

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// sha256Of hashes bytes (test helper for expected hashes).
func sha256Of(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// testKeys generates an Ed25519 pair for tests only (never committed,
// never production).
func testKeys(t *testing.T) (pub ed25519.PublicKey, priv ed25519.PrivateKey, id string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv, "test-key-1"
}

// writeZip creates a zip of name→content files, returning path+sha.
func writeZip(t *testing.T, dir string, files map[string]string) (string, string) {
	t.Helper()
	p := filepath.Join(dir, "payload.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return p, hex.EncodeToString(sum[:])
}

// fixtureDir builds a provider dir: artifacts + signed manifest.json.
// Returns dir, manifest, private key id mapping helper.
func fixtureDir(t *testing.T, m Manifest, priv ed25519.PrivateKey, keyID string) string {
	t.Helper()
	dir := t.TempDir()
	sm, err := SignManifest(m, keyID, priv)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(sm, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func trusted(t *testing.T, id string, pub ed25519.PublicKey) TrustedKeys {
	t.Helper()
	return TrustedKeys{Keys: map[string]ed25519.PublicKey{id: pub}}
}
