package update

import (
	"context"
	"fmt"
	"io"
)

// GitHubProvider will serve releases from GitHub Releases once the user
// creates the repository and configures its identity. It is an
// intentional skeleton: the method set mirrors what the real provider
// will implement, but no network code or repository identity lives here
// yet. The UpdateManager treats it like any other ReleaseProvider.
type GitHubProvider struct {
	// Owner and Repo are empty until configured (post-task step).
	// No fake production URL is ever synthesized from empties.
	Owner string
	Repo  string
}

// ErrNotConfigured is returned until repository identity is wired.
var ErrNotConfigured = fmt.Errorf("update: github provider not configured (no repository yet)")

// Name for logs (never credentials — there are none).
func (p GitHubProvider) Name() string {
	if p.Owner == "" || p.Repo == "" {
		return "github:(unconfigured)"
	}
	return "github:" + p.Owner + "/" + p.Repo
}

// Check resolves the newest release for q. Until configured it reports
// not-configured so the manager can stay quiet instead of erroring the UI.
func (p GitHubProvider) Check(ctx context.Context, q Query) (*Release, error) {
	if p.Owner == "" || p.Repo == "" {
		return nil, ErrNotConfigured
	}
	// Wiring point (future task): GET /repos/{owner}/{repo}/releases,
	// filter by channel tag, pick platform/arch assets, build Manifest.
	return nil, fmt.Errorf("update: github provider: fetch not implemented")
}

// OpenArtifact streams a release asset.
func (p GitHubProvider) OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error) {
	if p.Owner == "" || p.Repo == "" {
		return nil, ErrNotConfigured
	}
	return nil, fmt.Errorf("update: github provider: fetch not implemented")
}
