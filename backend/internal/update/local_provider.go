package update

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DirProvider serves releases from a local directory for deterministic
// tests and development. Layout:
//
//	dir/
//	  manifest.json   (SignedManifest)
//	  <artifact files…>
//
// Artifact URLs may be relative filenames (resolved against dir) or
// file:// URLs. This provider is NEVER the production update source.
type DirProvider struct {
	Dir string
}

// Name for logs.
func (p DirProvider) Name() string { return "local:" + p.Dir }

// Check reads, validates and query-matches the directory manifest.
func (p DirProvider) Check(ctx context.Context, q Query) (*Release, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(p.Dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("update: local provider: %w", err)
	}
	sm, err := ParseSignedManifest(raw)
	if err != nil {
		return nil, err
	}
	if err := MatchQuery(sm.Manifest, q); err != nil {
		return nil, err
	}
	return &Release{Manifest: *sm, Source: p.Dir}, nil
}

// OpenArtifact opens a manifest artifact by relative name or file URL.
// Anything escaping the directory is rejected.
func (p DirProvider) OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := a.Filename
	if u, err := url.Parse(a.URL); err == nil && u.Scheme == "file" {
		if u.Host != "" && u.Host != "localhost" {
			return nil, fmt.Errorf("update: refusing remote file host %q", u.Host)
		}
		name = u.Path
	} else if strings.Contains(a.URL, "://") {
		return nil, fmt.Errorf("update: local provider cannot fetch %q", a.URL)
	} else if a.URL != "" {
		name = a.URL
	}
	clean := filepath.Clean("/" + name)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("update: unsafe artifact path %q", name)
	}
	f, err := os.Open(filepath.Join(p.Dir, rel))
	if err != nil {
		return nil, fmt.Errorf("update: missing asset %q: %w", rel, err)
	}
	return f, nil
}
