package update

import (
	"crypto/ed25519"
	"encoding/hex"
)

// Production release trust roots. GENERATED at release-setup time from
// the public half only — no private material is, was, or will ever be
// in this file. To rotate: generate a new pair, add its key id here,
// keep signing with either key during overlap, then drop the old id.
func defaultTrustedKeys() TrustedKeys {
	pub, err := hex.DecodeString("c05f37b614d6a19a078d7944af1b5f059012f4f9ed40feb31e80f180d94db0fa")
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return TrustedKeys{}
	}
	return TrustedKeys{Keys: map[string]ed25519.PublicKey{
		"connective-release-1": ed25519.PublicKey(pub),
	}}
}

// DefaultTrustedKeys exposes the embedded production roots to the
// daemon. A CONNECTIVE_UPDATE_TRUSTED_KEYS file, when set, replaces
// (not augments) these for dev/test isolation.
func DefaultTrustedKeys() TrustedKeys { return defaultTrustedKeys() }
