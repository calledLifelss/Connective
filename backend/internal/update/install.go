package update

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// openZip opens a full-artifact bundle (a zip of the target tree).
func openZip(src string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("update: bad full artifact: %w", err)
	}
	return zr, nil
}

// Versioned layout (target model; coexists with the RPM, see
// docs/UPDATE_ARCHITECTURE.md):
//
//	<root>/
//	  versions/0.2.0/…   installed trees (never mutated in place)
//	  versions/0.2.1/…
//	  current -> versions/0.2.1   (atomic symlink swap = activation)
//	  previous -> versions/0.2.0  (rollback anchor)
//	  staging/…                   scratch space, wiped per run
//
// All paths stay confined under Root. The running application never
// overwrites itself: assembly happens in staging/, activation is a
// rename, and the old tree is retained until the new one passes health.
type Installer struct {
	// Root confines every write (production: versioned dir; tests: temp).
	Root string
	// HealthTimeout bounds the new version's startup signal.
	HealthTimeout time.Duration
}

func (in *Installer) healthTimeout() time.Duration {
	if in.HealthTimeout > 0 {
		return in.HealthTimeout
	}
	return 60 * time.Second
}

func (in *Installer) versionsDir() string { return filepath.Join(in.Root, "versions") }
func (in *Installer) stagingDir() string  { return filepath.Join(in.Root, "staging") }
func (in *Installer) currentLink() string { return filepath.Join(in.Root, "current") }
func (in *Installer) prevLink() string    { return filepath.Join(in.Root, "previous") }

// VersionDir returns the tree path for v (confined, no traversal).
func (in *Installer) VersionDir(v string) (string, error) {
	if _, err := ParseVersion(v); err != nil {
		return "", err
	}
	if strings.ContainsAny(v, `/\`) {
		return "", fmt.Errorf("update: unsafe version %q", v)
	}
	return filepath.Join(in.versionsDir(), v), nil
}

// Assemble builds versions/<target> in staging then renames it into
// place: full artifacts are verified zips unpacked whole; deltas are
// applied over a copy of the current tree via applier. The live `current`
// link is untouched until Activate.
func (in *Installer) Assemble(ctx context.Context, target string, stagedArtifact string, kind string, applier DeltaApplier) (string, error) {
	dir, err := in.VersionDir(target)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("update: version %s already installed", target)
	}
	staging, err := os.MkdirTemp(in.stagingDir(), "assemble-*")
	if err != nil {
		if merr := os.MkdirAll(in.stagingDir(), 0o700); merr != nil {
			return "", merr
		}
		staging, err = os.MkdirTemp(in.stagingDir(), "assemble-*")
		if err != nil {
			return "", err
		}
	}
	defer os.RemoveAll(staging)
	build := filepath.Join(staging, "tree")
	switch kind {
	case ArtifactFull:
		if err := unzipAll(stagedArtifact, build); err != nil {
			return "", err
		}
	case ArtifactDelta:
		curTree, cerr := os.Readlink(in.currentLink())
		if cerr != nil {
			return "", fmt.Errorf("update: delta needs a current install: %w", cerr)
		}
		if !filepath.IsAbs(curTree) {
			curTree = filepath.Join(in.Root, curTree)
		}
		if applier == nil {
			applier = ZipOverlayApplier{}
		}
		if err := applier.Apply(ctx, curTree, stagedArtifact, build); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("update: unknown artifact kind %q", kind)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(in.versionsDir(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(build, dir); err != nil {
		return "", fmt.Errorf("update: stage target: %w", err)
	}
	return dir, nil
}

// Activate atomically points `current` at versions/<target>, anchoring
// the old tree in `previous` for rollback. Returns the previous version
// ("" when none).
func (in *Installer) Activate(target string) (string, error) {
	dir, err := in.VersionDir(target)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("update: version %s not assembled", target)
	}
	var prev string
	if cur, err := os.Readlink(in.currentLink()); err == nil {
		prev = filepath.Base(cur)
		_ = os.Remove(in.prevLink())
		if err := os.Symlink(cur, in.prevLink()); err != nil {
			return "", fmt.Errorf("update: anchor rollback: %w", err)
		}
	}
	next := filepath.Join("versions", target)
	tmp := filepath.Join(in.Root, ".current.tmp")
	_ = os.Remove(tmp)
	if err := os.Symlink(next, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, in.currentLink()); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("update: activate: %w", err)
	}
	return prev, nil
}

// Rollback re-points `current` at the anchored previous tree.
func (in *Installer) Rollback() (string, error) {
	prev, err := os.Readlink(in.prevLink())
	if err != nil {
		return "", fmt.Errorf("update: nothing to roll back to: %w", err)
	}
	tmp := filepath.Join(in.Root, ".current.tmp")
	_ = os.Remove(tmp)
	if err := os.Symlink(prev, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, in.currentLink()); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("update: rollback: %w", err)
	}
	return filepath.Base(prev), nil
}

// Current resolves the active version ("" when unset).
func (in *Installer) Current() string {
	cur, err := os.Readlink(in.currentLink())
	if err != nil {
		return ""
	}
	return filepath.Base(cur)
}

// AwaitHealthy polls ready until true or the timeout. ready must be a
// LOCAL signal (file, socket, exit code) — never the network.
func (in *Installer) AwaitHealthy(ctx context.Context, ready func() bool) error {
	deadline := time.Now().Add(in.healthTimeout())
	for {
		if ready() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("update: new version failed health check")
		}
	}
}

func unzipAll(src, dst string) error {
	// Reuse the delta overlay machinery with an empty base.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	z, err := openZip(src)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		if err := unzipOne(f, dst); err != nil {
			return err
		}
	}
	return nil
}
