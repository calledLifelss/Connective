package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Release asset contract: the manifest (self-contained signature) plus
// an optional detached signature copy for external verifiers.
const (
	GitHubManifestAsset = "update-manifest.json"
	GitHubSigAsset      = "update-manifest.json.sig"
)

// DefaultAPIBase is the public GitHub API. Tests override it.
const DefaultAPIBase = "https://api.github.com"

// ErrNotConfigured is returned until repository identity is wired.
var ErrNotConfigured = fmt.Errorf("update: github provider not configured (no repository yet)")

// GitHubProvider serves releases from GitHub Releases. Reads are
// anonymous by default (public repository); Token is optional and comes
// only from the operator environment, never from the app or repo.
type GitHubProvider struct {
	Owner string
	Repo  string
	// APIBase overrides the API root (tests). Empty = DefaultAPIBase.
	APIBase string
	// Client performs HTTP (default: 30s timeout). Never logs Token.
	Client *http.Client
	// Token is an optional bearer token (empty = anonymous).
	Token string
}

// RateLimitError signals GitHub throttling; managers render it calmly
// and back off instead of alarming the user.
type RateLimitError struct {
	Reset time.Time
}

func (e *RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return "update: github request limit reached, trying again later"
	}
	return fmt.Sprintf("update: github request limit reached, retry after %s",
		e.Reset.Format(time.RFC3339))
}

// IsRateLimit reports whether err is throttling.
func IsRateLimit(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

func (p GitHubProvider) apiBase() string {
	if p.APIBase != "" {
		return strings.TrimSuffix(p.APIBase, "/")
	}
	return DefaultAPIBase
}

func (p GitHubProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Name for logs (identity only, never credentials).
func (p GitHubProvider) Name() string {
	if p.Owner == "" || p.Repo == "" {
		return "github:(unconfigured)"
	}
	return "github:" + p.Owner + "/" + p.Repo
}

// ghRelease mirrors the subset of the GitHub release API we consume.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Check discovers the newest applicable release: channel-appropriate,
// versioned, carrying a valid manifest matching platform/arch/minimum.
// Releases that cannot serve an update are skipped, newest first.
func (p GitHubProvider) Check(ctx context.Context, q Query) (*Release, error) {
	if p.Owner == "" || p.Repo == "" {
		return nil, ErrNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rels, err := p.listReleases(ctx)
	if err != nil {
		return nil, err
	}
	var skipped []string
	for _, r := range rels {
		if r.Draft {
			continue
		}
		if q.Channel == ChannelStable && r.Prerelease {
			continue // stable users never see prereleases
		}
		ver, err := tagVersion(r.TagName)
		if err != nil {
			skipped = append(skipped, r.TagName+" (untagged version)")
			continue
		}
		newer, err := IsNewerThan(q.CurrentVersion, ver)
		if err != nil || !newer {
			continue // older, same, or unparsable current
		}
		rel, rerr := p.releaseFrom(ctx, r, q)
		if rerr != nil {
			if IsNoUpdate(rerr) {
				skipped = append(skipped, r.TagName+" ("+rerr.Error()+")")
				continue
			}
			return nil, rerr
		}
		return rel, nil
	}
	if len(rels) == 0 {
		return nil, ErrNoUpdate("no releases published")
	}
	reason := "no applicable release"
	if len(skipped) > 0 {
		reason = "skipped: " + strings.Join(skipped, "; ")
	}
	return nil, ErrNoUpdate(reason)
}

// manifestCandidates prefers a platform-specific manifest
// (update-manifest-windows.json) and falls back to the shared
// update-manifest.json. Linux behavior is unchanged: it keeps reading
// the shared file (also published as update-manifest-linux.json by
// tooling that wants symmetry, which the lookup tolerates).
func manifestCandidates(platform string) []string {
	names := []string{"update-manifest-" + platform + ".json", GitHubManifestAsset}
	if platform == PlatformLinux {
		names = append(names, "update-manifest-linux.json")
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// releaseFrom validates one release's manifest and resolves artifacts.
func (p GitHubProvider) releaseFrom(ctx context.Context, r ghRelease, q Query) (*Release, error) {
	manURL := ""
	var sigURL string
	byName := map[string]ghAsset{}
	for _, a := range r.Assets {
		byName[a.Name] = a
	}
	for _, want := range manifestCandidates(q.Platform) {
		if a, ok := byName[want]; ok && a.BrowserDownloadURL != "" {
			manURL = a.BrowserDownloadURL
			if s, ok := byName[want+".sig"]; ok {
				sigURL = s.BrowserDownloadURL
			} else if s, ok := byName[GitHubSigAsset]; ok {
				sigURL = s.BrowserDownloadURL
			}
			break
		}
	}
	if manURL == "" {
		return nil, ErrNoUpdate("missing " + GitHubManifestAsset)
	}
	raw, err := p.getBytes(ctx, manURL, 1<<20)
	if err != nil {
		return nil, err
	}
	sm, err := ParseSignedManifest(raw)
	if err != nil {
		return nil, fmt.Errorf("update: release %s: %w", r.TagName, err)
	}
	if err := MatchQuery(sm.Manifest, q); err != nil {
		return nil, err
	}
	// Detached signature, when published, must agree with the embedded
	// one; a stale/foreign .sig fails closed.
	if sigURL != "" {
		sigRaw, err := p.getBytes(ctx, sigURL, 64*1024)
		if err != nil {
			return nil, fmt.Errorf("update: release %s sig: %w", r.TagName, err)
		}
		if strings.TrimSpace(string(sigRaw)) != strings.TrimSpace(sm.Signature) {
			return nil, fmt.Errorf("update: release %s: detached signature mismatch", r.TagName)
		}
	}
	// Resolve relative artifact references against release assets.
	fixed := sm.Manifest
	for i, a := range fixed.Artifacts {
		if strings.Contains(a.URL, "://") {
			continue
		}
		name := a.URL
		if name == "" {
			name = a.Filename
		}
		asset, ok := byName[name]
		if !ok || asset.BrowserDownloadURL == "" {
			return nil, fmt.Errorf("update: release %s: asset %q missing", r.TagName, name)
		}
		fixed.Artifacts[i].URL = asset.BrowserDownloadURL
	}
	sm.Manifest = fixed
	return &Release{Manifest: *sm, Source: p.Name() + "@" + r.TagName}, nil
}

// OpenArtifact streams a manifest artifact by filename (release asset)
// or absolute URL.
func (p GitHubProvider) OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error) {
	if p.Owner == "" || p.Repo == "" {
		return nil, ErrNotConfigured
	}
	if !strings.Contains(a.URL, "://") {
		return nil, fmt.Errorf("update: github provider needs resolved URLs (run Check first)")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	p.auth(req)
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: asset fetch: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if rl := rateLimit(resp); rl != nil {
			return nil, rl
		}
		return nil, fmt.Errorf("update: asset server returned %s", resp.Status)
	}
	return resp.Body, nil
}

// listReleases fetches the release index (newest first from the API).
func (p GitHubProvider) listReleases(ctx context.Context) ([]ghRelease, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", p.apiBase(), p.Owner, p.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	p.auth(req)
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: releases fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if rl := rateLimit(resp); rl != nil {
			return nil, rl
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("update: repository %s/%s not found", p.Owner, p.Repo)
		}
		return nil, fmt.Errorf("update: releases API returned %s", resp.Status)
	}
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var rels []ghRelease
	if err := json.Unmarshal(limited, &rels); err != nil {
		return nil, fmt.Errorf("update: bad releases response: %w", err)
	}
	return rels, nil
}

// getBytes fetches a small document with a size cap.
func (p GitHubProvider) getBytes(ctx context.Context, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	p.auth(req)
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if rl := rateLimit(resp); rl != nil {
			return nil, rl
		}
		return nil, fmt.Errorf("update: server returned %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, fmt.Errorf("update: document exceeds %d bytes", max)
	}
	return raw, nil
}

func (p GitHubProvider) auth(req *http.Request) {
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
}

// rateLimit maps throttling responses (403/429 with zero quota) to a
// typed error the manager renders calmly.
func rateLimit(resp *http.Response) error {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != 429 {
		return nil
	}
	if resp.Header.Get("X-RateLimit-Remaining") != "0" {
		return nil
	}
	var reset time.Time
	if n, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && n > 0 {
		reset = time.Unix(n, 0)
	}
	return &RateLimitError{Reset: reset}
}

// tagVersion strips a leading v ("v0.2.1" → 0.2.1).
func tagVersion(tag string) (string, error) {
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(t, "v")
	t = strings.TrimPrefix(t, "V")
	if _, err := ParseVersion(t); err != nil {
		return "", err
	}
	return t, nil
}
