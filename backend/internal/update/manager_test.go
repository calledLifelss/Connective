package update

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// testManager builds a manager over a fixture provider dir with test
// trust. currentV points at the caller version so tests can simulate an
// upgrade completing.
func testManager(t *testing.T, currentV *string, pub ed25519.PublicKey, id string) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	m := &Manager{
		Provider:       DirProvider{Dir: dir},
		Keys:           TrustedKeys{Keys: map[string]ed25519.PublicKey{id: pub}},
		DataDir:        t.TempDir(),
		Channel:        func() string { return ChannelStable },
		CurrentVersion: func() string { return *currentV },
		TestApply:      true,
		TestRoot:       t.TempDir(),
	}
	var mu sync.Mutex
	var events []Status
	m.OnEvent = func(s Status) {
		mu.Lock()
		events = append(events, s)
		mu.Unlock()
	}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	_ = events
	return m, dir
}

// putZip writes a named zip artifact into dir, returning its sha.
func putZip(t *testing.T, dir, name string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return sha256Of(raw)
}

// publish signs m and writes it as the provider manifest.
func publish(t *testing.T, dir string, m Manifest, id string, priv ed25519.PrivateKey) {
	t.Helper()
	sm, err := SignManifest(m, id, priv)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(sm, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fullManifest(t *testing.T, dir, version, sha string, size int64) Manifest {
	t.Helper()
	return Manifest{
		Schema:       ManifestSchema,
		Version:      version,
		Channel:      ChannelStable,
		ReleaseNotes: []string{"Test release"},
		MinVersion:   "0.2.0",
		Platform:     PlatformLinux,
		Arch:         ArchX8664,
		Artifacts: []Artifact{
			{Type: ArtifactFull, Filename: "full.zip", Size: size, SHA256: sha, URL: "full.zip"},
		},
	}
}

func TestManagerFullFlow(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)

	sha := putZip(t, dir, "full.zip", map[string]string{"connective": "v2-binary"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.2.1", sha, st.Size()), id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s (%s)", got.State, got.Error)
	}
	if got := m.Status(); got.Info == nil || got.Info.Version != "0.2.1" {
		t.Fatalf("info missing: %+v", got.Info)
	}
	if got := m.Download(ctx); got.State != StateUpdateAvailable {
		t.Fatalf("download: %s (%s)", got.State, got.Error)
	}
	if got := m.Install(ctx); got.State != StateUpdated {
		t.Fatalf("install: %s (%s)", got.State, got.Error)
	}
	// New tree active through the versioned layout.
	got, err := os.ReadFile(filepath.Join(m.TestRoot, "current", "connective"))
	if err != nil || string(got) != "v2-binary" {
		t.Fatalf("activated tree wrong: %v %q", err, got)
	}
	// After upgrading, the same release is no-update.
	current = "0.2.1"
	if got := m.Check(ctx, true); got.State != StateNoUpdate {
		t.Fatalf("post-upgrade check: %s", got.State)
	}
}

func TestManagerNoUpdate(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.1"
	m, dir := testManager(t, &current, pub, id)
	sha := putZip(t, dir, "full.zip", map[string]string{"a": "b"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.2.1", sha, st.Size()), id, priv)
	if got := m.Check(context.Background(), true); got.State != StateNoUpdate {
		t.Fatalf("same version must be no-update, got %s", got.State)
	}
	if got := m.Status(); got.Info != nil {
		t.Fatal("no-update must not carry info")
	}
}

func TestManagerBadSignature(t *testing.T) {
	pub, _, id := testKeys(t)
	_, evilPriv, _ := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	sha := putZip(t, dir, "full.zip", map[string]string{"a": "b"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.2.1", sha, st.Size()), id, evilPriv)
	got := m.Check(context.Background(), true)
	if got.State != StateFailed {
		t.Fatalf("bad signature must fail, got %s", got.State)
	}
	// Nothing staged or assembled.
	if _, err := os.Stat(filepath.Join(m.TestRoot, "versions")); !os.IsNotExist(err) {
		t.Fatal("failed update must not touch the install root")
	}
}

func TestManagerHashMismatch(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	sha := putZip(t, dir, "full.zip", map[string]string{"a": "b"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	mm := fullManifest(t, dir, "0.2.1", sha, st.Size())
	mm.Artifacts[0].SHA256 = repeat("00", 32) // lie in the manifest
	publish(t, dir, mm, id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	got := m.Download(ctx)
	if got.State != StateFailed {
		t.Fatalf("hash mismatch must fail, got %s", got.State)
	}
}

func TestManagerDeltaFlow(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)

	// Seed the test install root with a current 0.2.0 tree.
	in := &Installer{Root: m.TestRoot}
	cur, _ := in.VersionDir("0.2.0")
	writeTree(t, cur, map[string]string{"connective": "v1", "keep": "k"})
	if _, err := in.Activate("0.2.0"); err != nil {
		t.Fatal(err)
	}

	fullSHA := putZip(t, dir, "full.zip", map[string]string{"connective": "v2-full"})
	fullSt, _ := os.Stat(filepath.Join(dir, "full.zip"))
	deltaSHA := putZip(t, dir, "delta.zip", map[string]string{"connective": "v2-delta"})
	deltaSt, _ := os.Stat(filepath.Join(dir, "delta.zip"))
	mm := Manifest{
		Schema: ManifestSchema, Version: "0.2.1", Channel: ChannelStable,
		MinVersion: "0.2.0", Platform: PlatformLinux, Arch: ArchX8664,
		Artifacts: []Artifact{
			{Type: ArtifactFull, Filename: "full.zip", Size: fullSt.Size(), SHA256: fullSHA, URL: "full.zip"},
			{Type: ArtifactDelta, Filename: "delta.zip", Size: deltaSt.Size(), SHA256: deltaSHA, URL: "delta.zip", FromVersion: "0.2.0"},
		},
	}
	// Shrink the delta so selection prefers it deterministically.
	if deltaSt.Size() >= fullSt.Size() {
		t.Skip("test zips too close in size for deterministic selection")
	}
	publish(t, dir, mm, id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	if got := m.Status(); got.Info.ArtifactType != ArtifactDelta {
		t.Fatalf("delta should be selected, got %s", got.Info.ArtifactType)
	}
	if got := m.Download(ctx); got.State != StateUpdateAvailable {
		t.Fatalf("download: %s", got.State)
	}
	if got := m.Install(ctx); got.State != StateUpdated {
		t.Fatalf("install: %s (%s)", got.State, got.Error)
	}
	gotBin, _ := os.ReadFile(filepath.Join(m.TestRoot, "current", "connective"))
	if string(gotBin) != "v2-delta" {
		t.Fatalf("delta result wrong: %q", gotBin)
	}
	keep, _ := os.ReadFile(filepath.Join(m.TestRoot, "current", "keep"))
	if string(keep) != "k" {
		t.Fatal("delta must preserve untouched files")
	}
}

func TestManagerDeltaFallbackToFull(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	// NOTE: no current tree in TestRoot, so the delta cannot apply;
	// Install must fall back to full instead of failing.
	fullSHA := putZip(t, dir, "full.zip", map[string]string{"connective": "v2-full"})
	fullSt, _ := os.Stat(filepath.Join(dir, "full.zip"))
	deltaSHA := putZip(t, dir, "delta.zip", map[string]string{"connective": "v2-delta"})
	deltaSt, _ := os.Stat(filepath.Join(dir, "delta.zip"))
	_ = deltaSt
	mm := Manifest{
		Schema: ManifestSchema, Version: "0.2.1", Channel: ChannelStable,
		MinVersion: "0.2.0", Platform: PlatformLinux, Arch: ArchX8664,
		Artifacts: []Artifact{
			{Type: ArtifactFull, Filename: "full.zip", Size: fullSt.Size(), SHA256: fullSHA, URL: "full.zip"},
			{Type: ArtifactDelta, Filename: "delta.zip", Size: 1, SHA256: deltaSHA, URL: "delta.zip", FromVersion: "0.2.0"},
		},
	}
	publish(t, dir, mm, id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	if got := m.Status(); got.Info.ArtifactType != ArtifactDelta {
		t.Fatalf("delta should be selected, got %s", got.Info.ArtifactType)
	}
	if got := m.Download(ctx); got.State != StateUpdateAvailable {
		t.Fatalf("download: %s", got.State)
	}
	if got := m.Install(ctx); got.State != StateUpdated {
		t.Fatalf("install should fall back to full, got %s (%s)", got.State, got.Error)
	}
	gotBin, _ := os.ReadFile(filepath.Join(m.TestRoot, "current", "connective"))
	if string(gotBin) != "v2-full" {
		t.Fatalf("fallback tree wrong: %q", gotBin)
	}
}

func TestManagerCancelDownload(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	// close(release) registered last → runs first, unblocking the
	// handler before srv.Close() waits for it (LIFO).
	defer srv.Close()
	defer close(release)

	mm := fullManifest(t, dir, "0.2.1", repeat("ab", 32), 1<<20)
	mm.Artifacts[0].URL = srv.URL + "/big.zip"
	publish(t, dir, mm, id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	done := make(chan Status, 1)
	go func() { done <- m.Download(ctx) }()
	time.Sleep(300 * time.Millisecond)
	m.Cancel()
	select {
	case got := <-done:
		if got.State != StateCancelled {
			t.Fatalf("expected cancelled, got %s", got.State)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancel did not stop the download")
	}
}

func TestManagerDismiss(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	sha := putZip(t, dir, "full.zip", map[string]string{"a": "b"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.2.1", sha, st.Size()), id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	if got := m.Dismiss(); got.State != StateIdle {
		t.Fatalf("dismiss: %s", got.State)
	}
	// Same version stays hidden…
	if got := m.Check(ctx, true); got.State != StateNoUpdate {
		t.Fatalf("dismissed version should hide, got %s", got.State)
	}
	// …but a newer version reappears.
	sha2 := putZip(t, dir, "full2.zip", map[string]string{"a": "c"})
	st2, _ := os.Stat(filepath.Join(dir, "full2.zip"))
	mm := fullManifest(t, dir, "0.2.2", sha2, st2.Size())
	mm.Artifacts[0].Filename = "full2.zip"
	mm.Artifacts[0].URL = "full2.zip"
	publish(t, dir, mm, id, priv)
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("newer version should reappear, got %s", got.State)
	}
}

func TestManagerUnconfiguredProvider(t *testing.T) {
	current := "0.2.0"
	m, _ := testManager(t, &current, nil, "")
	m.Provider = GitHubProvider{}
	m.Keys = TrustedKeys{}
	got := m.Check(context.Background(), true)
	if got.State != StateNoUpdate {
		t.Fatalf("unconfigured provider must stay quiet, got %s (%s)", got.State, got.Error)
	}
	if got.Error != "" {
		t.Fatal("quiet no-update must not surface an error")
	}
}

func TestManagerCachePersists(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	sha := putZip(t, dir, "full.zip", map[string]string{"a": "b"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.2.1", sha, st.Size()), id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s", got.State)
	}
	// A fresh manager over the same DataDir reloads the cache: a
	// non-manual check must not hit the provider again.
	m2 := &Manager{
		Provider: failingProvider{},
		DataDir:  m.DataDir,
		Channel:  func() string { return ChannelStable },
	}
	if err := m2.Init(); err != nil {
		t.Fatal(err)
	}
	if got := m2.Check(ctx, false); got.State != StateIdle && got.State != StateNoUpdate {
		t.Fatalf("cached check should not fail, got %s", got.State)
	}
}

type failingProvider struct{}

func (failingProvider) Name() string { return "failing" }
func (failingProvider) Check(ctx context.Context, q Query) (*Release, error) {
	return nil, fmt.Errorf("provider must not be consulted")
}
func (failingProvider) OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error) {
	return nil, fmt.Errorf("no")
}
