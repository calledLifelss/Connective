package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestSignatureRoundTrip(t *testing.T) {
	pub, priv, id := testKeys(t)
	m := validManifest()
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := trusted(t, id, pub).VerifyManifest(sm); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestSignatureTampered(t *testing.T) {
	pub, priv, id := testKeys(t)
	m := validManifest()
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		t.Fatal(err)
	}
	sm.Manifest.Version = "9.9.9" // attacker bumps the version
	if err := trusted(t, id, pub).VerifyManifest(sm); err == nil {
		t.Fatal("tampered manifest must fail verification")
	}
}

func TestSignatureWrongKey(t *testing.T) {
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, priv, id := testKeys(t)
	m := validManifest()
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := trusted(t, id, other).VerifyManifest(sm); err == nil {
		t.Fatal("wrong-key signature must fail")
	}
}

func TestSignatureUnknownKeyID(t *testing.T) {
	pub, priv, _ := testKeys(t)
	m := validManifest()
	sm, err := SignManifest(m, "someone-else", priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := trusted(t, "test-key-1", pub).VerifyManifest(sm); err == nil {
		t.Fatal("unknown key id must fail")
	}
}

func TestSignatureNoTrustedKeys(t *testing.T) {
	_, priv, id := testKeys(t)
	m := validManifest()
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := (TrustedKeys{}).VerifyManifest(sm); err == nil {
		t.Fatal("empty trust set must fail closed")
	}
}

func TestHashVerify(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.bin")
	content := []byte("hello connective")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256Of(content)
	if err := VerifySHA256(p, sum); err != nil {
		t.Fatalf("correct hash rejected: %v", err)
	}
	if err := VerifySHA256(p, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("mismatch must fail")
	}
	if err := VerifySHA256(filepath.Join(dir, "missing"), sum); err == nil {
		t.Fatal("missing file must fail")
	}
}
