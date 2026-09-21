package update

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ghFixture serves a fake releases API + asset bytes. No live GitHub in
// unit tests, ever.
type ghFixture struct {
	t        *testing.T
	releases []ghRelease
	bodies   map[string][]byte // asset URL path → bytes
	status   int
	seenAuth []string
}

func (f *ghFixture) serve() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.seenAuth = append(f.seenAuth, r.Header.Get("Authorization"))
		if strings.HasSuffix(r.URL.Path, "/releases") || strings.HasSuffix(r.URL.Path, "/releases/") {
			if f.status != 0 {
				if f.status == 403 {
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("X-RateLimit-Reset", "1790000000")
				}
				w.WriteHeader(f.status)
				return
			}
			raw, _ := json.Marshal(f.releases)
			w.Header().Set("Content-Type", "application/json")
			w.Write(raw)
			return
		}
		if b, ok := f.bodies[r.URL.Path]; ok {
			w.Write(b)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func ghProvider(srv *httptest.Server) GitHubProvider {
	return GitHubProvider{Owner: "o", Repo: "r", APIBase: srv.URL, Client: srv.Client()}
}

// signedRelease builds one release entry with a real signed manifest;
// asset bodies are registered on the fixture (host-rewritten at serve).
func (f *ghFixture) signedRelease(tag string, pre bool, m Manifest, priv ed25519.PrivateKey, id string, extraAssets ...string) ghRelease {
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, _ := json.Marshal(sm)
	manPath := "/dl/" + tag + "/update-manifest.json"
	f.bodies[manPath] = raw
	sigPath := "/dl/" + tag + "/update-manifest.json.sig"
	f.bodies[sigPath] = []byte(sm.Signature + "\n")
	assets := []ghAsset{
		{Name: GitHubManifestAsset},
		{Name: GitHubSigAsset},
	}
	for _, n := range extraAssets {
		assets = append(assets, ghAsset{Name: n})
		if _, ok := f.bodies["/dl/"+tag+"/"+n]; !ok {
			f.bodies["/dl/"+tag+"/"+n] = []byte("asset:" + n)
		}
	}
	return ghRelease{TagName: tag, Prerelease: pre, Body: "notes for " + tag, Assets: assets}
}

// rewrite points asset URLs at the live test server.
func (f *ghFixture) rewrite(srv *httptest.Server) {
	for i, r := range f.releases {
		for j, a := range r.Assets {
			if a.BrowserDownloadURL == "" {
				// Derive from the registered body path when possible.
				for p := range f.bodies {
					if strings.HasSuffix(p, "/"+a.Name) && strings.Contains(p, r.TagName) {
						r.Assets[j].BrowserDownloadURL = srv.URL + p
					}
				}
			}
		}
		f.releases[i] = r
	}
}

func linuxManifest(version string, arts ...Artifact) Manifest {
	return linuxChannelManifest(ChannelStable, version, arts...)
}

func linuxChannelManifest(channel, version string, arts ...Artifact) Manifest {
	return Manifest{
		Schema: ManifestSchema, Version: version, Channel: channel,
		MinVersion: "0.2.0", Platform: PlatformLinux, Arch: ArchX8664,
		Artifacts: arts,
	}
}

func fulArt(name, url string) Artifact {
	return Artifact{Type: ArtifactFull, Filename: name, Size: 100, SHA256: repeat("ab", 32), URL: url}
}

func testQuery() Query {
	return Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
}

func TestGitHubStablePicksFinal(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.releases = []ghRelease{
		fx.signedRelease("v0.3.0-beta.1", true, linuxManifest("0.3.0-beta.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
		fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
		fx.signedRelease("v0.2.0", false, linuxManifest("0.2.0", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	p := ghProvider(srv)
	rel, err := p.Check(context.Background(), testQuery())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Manifest.Manifest.Version != "0.2.1" {
		t.Fatalf("stable must take newest final, got %s", rel.Manifest.Manifest.Version)
	}
	// Relative artifact URLs resolve to release download URLs.
	if !strings.HasPrefix(rel.Manifest.Manifest.Artifacts[0].URL, srv.URL) {
		t.Fatalf("artifact URL unresolved: %q", rel.Manifest.Manifest.Artifacts[0].URL)
	}
}

func TestGitHubBetaTakesPrerelease(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.releases = []ghRelease{
		fx.signedRelease("v0.3.0-beta.1", true, linuxChannelManifest(ChannelBeta, "0.3.0-beta.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
		fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	q := testQuery()
	q.Channel = ChannelBeta
	rel, err := ghProvider(srv).Check(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Manifest.Manifest.Version != "0.3.0-beta.1" {
		t.Fatalf("beta must take prerelease, got %s", rel.Manifest.Manifest.Version)
	}
}

func TestGitHubSameAndOlder(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.releases = []ghRelease{
		fx.signedRelease("v0.2.0", false, linuxManifest("0.2.0", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
		fx.signedRelease("v0.1.0", false, linuxManifest("0.1.0", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); !IsNoUpdate(err) {
		t.Fatalf("nothing newer must be no-update, got %v", err)
	}
}

func TestGitHubSkipsBadReleases(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	// Newest has no manifest asset; older good one must win.
	bad := ghRelease{TagName: "v0.9.0", Body: "broken", Assets: []ghAsset{{Name: "full.zip", BrowserDownloadURL: "http://x/full.zip"}}}
	fx.releases = []ghRelease{
		bad,
		fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	rel, err := ghProvider(srv).Check(context.Background(), testQuery())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Manifest.Manifest.Version != "0.2.1" {
		t.Fatalf("should fall back to manifest-carrying release, got %s", rel.Manifest.Manifest.Version)
	}
}

func TestGitHubMissingAsset(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.releases = []ghRelease{
		fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("ghost.zip", "ghost.zip")), priv, id /* no ghost asset */),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); err == nil {
		t.Fatal("manifest referencing a missing asset must fail")
	}
}

func TestGitHubMalformedManifest(t *testing.T) {
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.bodies["/dl/v0.2.1/update-manifest.json"] = []byte("{not json")
	fx.bodies["/dl/v0.2.1/update-manifest.json.sig"] = []byte("aa")
	fx.releases = []ghRelease{{
		TagName: "v0.2.1",
		Assets: []ghAsset{
			{Name: GitHubManifestAsset, BrowserDownloadURL: "PLACEHOLDER"},
			{Name: GitHubSigAsset, BrowserDownloadURL: "PLACEHOLDER"},
		},
	}}
	srv := fx.serve()
	defer srv.Close()
	for i, r := range fx.releases {
		for j := range r.Assets {
			name := r.Assets[j].Name
			fx.releases[i].Assets[j].BrowserDownloadURL = srv.URL + "/dl/v0.2.1/" + name
		}
	}
	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); err == nil {
		t.Fatal("malformed manifest must fail")
	}
}

func TestGitHubAPIFailure(t *testing.T) {
	fx := &ghFixture{t: t, bodies: map[string][]byte{}, status: 500}
	srv := fx.serve()
	defer srv.Close()
	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); err == nil {
		t.Fatal("API 500 must fail")
	}
}

func TestGitHubRateLimit(t *testing.T) {
	fx := &ghFixture{t: t, bodies: map[string][]byte{}, status: 403}
	srv := fx.serve()
	defer srv.Close()
	_, err := ghProvider(srv).Check(context.Background(), testQuery())
	if !IsRateLimit(err) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if got := friendlyCheck(err); !strings.Contains(got, "limit") {
		t.Fatalf("unfriendly rate message: %q", got)
	}
}

func TestGitHubArchMismatch(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	m := linuxManifest("0.2.1", fulArt("full.zip", "full.zip"))
	m.Arch = ArchAARCH64
	fx.releases = []ghRelease{
		fx.signedRelease("v0.2.1", false, m, priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); !IsNoUpdate(err) {
		t.Fatalf("arch mismatch must be no-update, got %v", err)
	}
}

func TestGitHubSigMismatch(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	rel := fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip")
	fx.releases = []ghRelease{rel}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)
	// Publish a stale detached signature disagreeing with the embedded one.
	fx.bodies["/dl/v0.2.1/update-manifest.json.sig"] = []byte(repeat("ff", 64) + "\n")

	if _, err := ghProvider(srv).Check(context.Background(), testQuery()); err == nil {
		t.Fatal("detached signature mismatch must fail")
	}
}

func TestGitHubAnonymousByDefault(t *testing.T) {
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	srv := fx.serve()
	defer srv.Close()
	_, _ = ghProvider(srv).Check(context.Background(), testQuery())
	for _, h := range fx.seenAuth {
		if h != "" {
			t.Fatalf("anonymous provider sent credentials: %q", h)
		}
	}
}

func TestGitHubOpenArtifact(t *testing.T) {
	_, priv, id := testKeys(t)
	fx := &ghFixture{t: t, bodies: map[string][]byte{}}
	fx.releases = []ghRelease{
		fx.signedRelease("v0.2.1", false, linuxManifest("0.2.1", fulArt("full.zip", "full.zip")), priv, id, "full.zip"),
	}
	srv := fx.serve()
	defer srv.Close()
	fx.rewrite(srv)

	p := ghProvider(srv)
	rel, err := p.Check(context.Background(), testQuery())
	if err != nil {
		t.Fatal(err)
	}
	rc, err := p.OpenArtifact(context.Background(), rel.Manifest.Manifest.Artifacts[0])
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	buf := make([]byte, 64)
	n, _ := rc.Read(buf)
	if string(buf[:n]) != "asset:full.zip" {
		t.Fatalf("wrong asset bytes: %q", buf[:n])
	}
}

func TestTagVersion(t *testing.T) {
	for tag, want := range map[string]string{"v0.2.1": "0.2.1", "V1.0.0": "1.0.0", "0.2.0": "0.2.0"} {
		got, err := tagVersion(tag)
		if err != nil || got != want {
			t.Errorf("tagVersion(%q) = %q, %v", tag, got, err)
		}
	}
	if _, err := tagVersion("not-a-version"); err == nil {
		t.Error("garbage tag must fail")
	}
}
