package update

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ManifestSchema is the version of this manifest format. Bump when the
// shape changes; readers reject unknown major schemas loudly.
const ManifestSchema = 1

// Artifact types. FULL is the always-supported safety net; DELTA is an
// optimization the selector may prefer when valid and beneficial.
const (
	ArtifactFull  = "full"
	ArtifactDelta = "delta"
)

// Artifact describes one downloadable update payload.
type Artifact struct {
	Type        string `json:"type"` // full | delta
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`                 // lowercase hex
	URL         string `json:"url"`                    // http(s) or file URL
	FromVersion string `json:"from_version,omitempty"` // delta source
}

// Manifest is the unsigned update description. JSON field order is
// fixed (struct order) so CanonicalBytes is deterministic for signing.
type Manifest struct {
	Schema       int        `json:"manifest_version"`
	Version      string     `json:"version"`
	Channel      string     `json:"channel"`
	ReleaseDate  string     `json:"release_date,omitempty"`
	ReleaseNotes []string   `json:"release_notes,omitempty"`
	MinVersion   string     `json:"minimum_version,omitempty"`
	Platform     string     `json:"platform"`
	Arch         string     `json:"architecture"`
	Artifacts    []Artifact `json:"artifacts"`
}

// SignedManifest is what providers distribute: the manifest plus an
// Ed25519 signature over CanonicalBytes and the signing key id.
type SignedManifest struct {
	Manifest  Manifest `json:"manifest"`
	Signature string   `json:"signature"` // hex Ed25519 signature
	KeyID     string   `json:"key_id"`
}

// CanonicalBytes returns the deterministic bytes covered by Signature.
func (m Manifest) CanonicalBytes() ([]byte, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("update: canonicalize: %w", err)
	}
	return raw, nil
}

// Validate checks structural invariants (not trust — see verify.go).
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("update: unsupported manifest schema %d", m.Schema)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if !ValidChannel(m.Channel) {
		return fmt.Errorf("update: unknown channel %q", m.Channel)
	}
	if m.MinVersion != "" {
		if _, err := ParseVersion(m.MinVersion); err != nil {
			return fmt.Errorf("update: %w", err)
		}
	}
	if strings.TrimSpace(m.Platform) == "" {
		return fmt.Errorf("update: manifest missing platform")
	}
	if strings.TrimSpace(m.Arch) == "" {
		return fmt.Errorf("update: manifest missing architecture")
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("update: manifest has no artifacts")
	}
	seen := map[string]bool{}
	for i, a := range m.Artifacts {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("update: artifact %d: %w", i, err)
		}
		if a.Type == ArtifactDelta && a.FromVersion == "" {
			return fmt.Errorf("update: artifact %d: delta needs from_version", i)
		}
		key := a.Type + "|" + a.FromVersion
		if seen[key] {
			return fmt.Errorf("update: duplicate artifact %q", key)
		}
		seen[key] = true
	}
	return nil
}

// Validate checks one artifact's fields.
func (a Artifact) Validate() error {
	switch a.Type {
	case ArtifactFull, ArtifactDelta:
	default:
		return fmt.Errorf("unknown artifact type %q", a.Type)
	}
	if strings.TrimSpace(a.Filename) == "" {
		return fmt.Errorf("missing filename")
	}
	if strings.Contains(a.Filename, "..") {
		return fmt.Errorf("unsafe filename %q", a.Filename)
	}
	if a.Size <= 0 {
		return fmt.Errorf("bad size %d", a.Size)
	}
	if len(a.SHA256) != 64 || !isHex(a.SHA256) {
		return fmt.Errorf("bad sha256")
	}
	if strings.TrimSpace(a.URL) == "" {
		return fmt.Errorf("missing url")
	}
	if a.FromVersion != "" {
		if _, err := ParseVersion(a.FromVersion); err != nil {
			return fmt.Errorf("bad from_version: %w", err)
		}
	}
	return nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// ParseSignedManifest decodes and structurally validates a manifest.
// Signature trust is established separately (verify.go).
func ParseSignedManifest(raw []byte) (*SignedManifest, error) {
	var sm SignedManifest
	if err := json.Unmarshal(raw, &sm); err != nil {
		return nil, fmt.Errorf("update: bad manifest: %w", err)
	}
	if err := sm.Manifest.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sm.Signature) == "" {
		return nil, fmt.Errorf("update: manifest is unsigned")
	}
	if strings.TrimSpace(sm.KeyID) == "" {
		return nil, fmt.Errorf("update: manifest missing key id")
	}
	return &sm, nil
}
