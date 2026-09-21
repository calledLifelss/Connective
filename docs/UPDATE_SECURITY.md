# Update security

Threat model: a network attacker, a compromised mirror, or a malicious
release file must never yield code execution or a bricked install.

## Trust chain

1. **Signed manifest.** Every release ships `manifest.json` =
   `SignedManifest{manifest, signature, key_id}` with Ed25519 over the
   canonical manifest bytes. Verification (`TrustedKeys`) fails closed:
   empty trust set, unknown key id, bad hex, and bad signature all
   reject. Tampering with any field (e.g. bumping the version) breaks
   the signature (tested).
2. **Hashed artifacts.** Every artifact carries SHA-256, hashed on the
   wire during download and re-verified before staging. Mismatch →
   discard staged bytes, installation untouched (tested).
3. **Confined paths.** Filenames with `..`, absolute delta entries,
   zip-slip members, and absolute/escaped versions are rejected
   (tested). Installation roots are operator flags, never manifest data.

## Keys

- Production private key: does NOT exist yet and must NEVER be
  committed, embedded, or stored in test fixtures. It lives in the
  release operator's key file (0600) and, later, a CI secret.
- Production public key: will be embedded at build time (generated
  `trusted_keys.go`, `key_id` pinned) once the repository exists. Until
  then the daemon loads `{key_id: hexpub}` from
  `CONNECTIVE_UPDATE_TRUSTED_KEYS` (dev/test) and ships NO trust roots.
- Tests use ephemeral `ed25519.GenerateKey` pairs plus committed
  signed fixtures whose public halves only are stored
  (`frontend/test/testdata/updates/keys.json`).

## Release workflow (future)

1. `scripts/build-release` produces the bundle.
2. `scripts/generate-manifest` hashes artifacts into `manifest.json`.
3. `scripts/sign-manifest` signs with the operator key → committed
   alongside the GitHub Release (signature + key id public).
4. `scripts/verify-update` re-checks signature + hashes pre-publish.
5. GitHub Actions (post-task) repeats 2–4 with the CI-held key.

## Logging

Update check, discovered version, artifact selection, download,
verification, installation, and rollback are logged. Never logged:
private keys, credentials, tokens, or personal data beyond the
already-present install paths.
