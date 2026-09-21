package update

import (
	"context"
	"fmt"
	"io"
)

// Query scopes a release lookup: newest release on channel for this
// platform/arch that the caller's version may upgrade to.
type Query struct {
	Channel        string
	Platform       string
	Arch           string
	CurrentVersion string
}

// Release is one provider answer: a validated manifest plus its source.
type Release struct {
	Manifest SignedManifest
	Source   string // human-readable origin (dir path, URL, …)
}

// ReleaseProvider abstracts the update source. The UpdateManager only
// speaks this interface, so GitHub can replace the test provider later
// without touching manager, verification, staging or UI.
type ReleaseProvider interface {
	// Name identifies the provider in logs (never credentials).
	Name() string
	// Check returns the newest applicable signed release, or
	// ErrNoUpdate when nothing applies.
	Check(ctx context.Context, q Query) (*Release, error)
	// OpenArtifact opens a verified manifest's artifact payload.
	OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error)
}

// ErrNoUpdate signals "nothing to install" — not a failure. UI shows
// nothing; managers cache the check and stay quiet.
type noUpdate struct{ reason string }

func (e *noUpdate) Error() string { return "update: no update: " + e.reason }

// ErrNoUpdate builds the sentinel; IsNoUpdate tests for it.
func ErrNoUpdate(reason string) error { return &noUpdate{reason: reason} }

// IsNoUpdate reports whether err is the no-update sentinel.
func IsNoUpdate(err error) bool {
	_, ok := err.(*noUpdate)
	return ok
}

// OpenArtifactFunc adapts OpenArtifact implementations that only need
// the artifact (used by providers whose Check already resolved URLs).
type OpenArtifactFunc func(ctx context.Context, a Artifact) (io.ReadCloser, error)

// Platform constants for queries.
const (
	PlatformLinux   = "linux"
	PlatformWindows = "windows"
	ArchX8664       = "x86_64"
	ArchAARCH64     = "aarch64"
)

// MatchQuery reports whether m serves q (channel/platform/arch equal,
// target newer than current and current above the manifest minimum).
func MatchQuery(m Manifest, q Query) error {
	if m.Channel != q.Channel {
		return ErrNoUpdate(fmt.Sprintf("channel %q != %q", m.Channel, q.Channel))
	}
	if m.Platform != q.Platform {
		return ErrNoUpdate(fmt.Sprintf("platform %q != %q", m.Platform, q.Platform))
	}
	if m.Arch != q.Arch {
		return ErrNoUpdate(fmt.Sprintf("arch %q != %q", m.Arch, q.Arch))
	}
	if err := Acceptable(q.CurrentVersion, m.Version, m.MinVersion); err != nil {
		// An older-or-equal version is "no update", not a failure;
		// genuinely invalid manifests stay errors.
		if _, perr := ParseVersion(m.Version); perr != nil {
			return perr
		}
		return ErrNoUpdate(err.Error())
	}
	return nil
}
