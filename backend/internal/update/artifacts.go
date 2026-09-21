package update

import "fmt"

// fullArtifactOf returns the manifest's full artifact, if any.
func fullArtifactOf(rel *Release) *Artifact {
	if rel == nil {
		return nil
	}
	for i := range rel.Manifest.Manifest.Artifacts {
		if rel.Manifest.Manifest.Artifacts[i].Type == ArtifactFull {
			a := rel.Manifest.Manifest.Artifacts[i]
			return &a
		}
	}
	return nil
}

// SelectArtifact picks the smallest safe supported update: a delta
// sourced at exactly the current version when it is valid and
// beneficial, otherwise the full artifact. Full is the safety net and
// always wins ties or doubt.
func SelectArtifact(current string, m Manifest) (*Artifact, error) {
	cur, err := ParseVersion(current)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	var full, delta *Artifact
	for i := range m.Artifacts {
		a := &m.Artifacts[i]
		switch a.Type {
		case ArtifactFull:
			if full == nil {
				full = a
			}
		case ArtifactDelta:
			from, err := ParseVersion(a.FromVersion)
			if err != nil {
				continue // invalid delta entry: ignore, full covers us
			}
			if from.Compare(cur) != 0 {
				continue // delta from another version: not usable
			}
			if delta == nil || a.Size < delta.Size {
				delta = a
			}
		}
	}
	if full == nil {
		return nil, fmt.Errorf("update: manifest offers no full artifact")
	}
	if delta != nil && delta.Size < full.Size {
		return delta, nil
	}
	return full, nil
}
