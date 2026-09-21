package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256 hashes path and compares it to wantHex (case-insensitive).
// Mismatch fails closed: the caller must discard the staged artifact.
func VerifySHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("update: verify: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("update: verify: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(wantHex)) {
		return fmt.Errorf("update: sha256 mismatch for %s", path)
	}
	return nil
}

// TrustedKeys is the set of manifest signing keys the build trusts.
// Production wiring embeds the release public key here at build time
// (a generated file, never the private key). Empty means untrusted:
// verification fails closed rather than silently passing.
type TrustedKeys struct {
	Keys map[string]ed25519.PublicKey // keyID → public key
}

// VerifyManifest checks the manifest signature against trusted keys.
// Unknown key id, bad hex, or bad signature all fail closed.
func (t TrustedKeys) VerifyManifest(sm *SignedManifest) error {
	if len(t.Keys) == 0 {
		return fmt.Errorf("update: no trusted signing keys configured")
	}
	pub, ok := t.Keys[sm.KeyID]
	if !ok {
		return fmt.Errorf("update: untrusted signing key %q", sm.KeyID)
	}
	sig, err := hex.DecodeString(strings.TrimSpace(sm.Signature))
	if err != nil {
		return fmt.Errorf("update: bad signature encoding: %w", err)
	}
	msg, err := sm.Manifest.CanonicalBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, msg, sig) {
		return fmt.Errorf("update: manifest signature invalid")
	}
	return nil
}

// SignManifest signs m with priv for tests and release tooling. Private
// keys live only in memory / operator key files — never in the app.
func SignManifest(m Manifest, keyID string, priv ed25519.PrivateKey) (*SignedManifest, error) {
	msg, err := m.CanonicalBytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, msg)
	return &SignedManifest{
		Manifest:  m,
		Signature: hex.EncodeToString(sig),
		KeyID:     keyID,
	}, nil
}
