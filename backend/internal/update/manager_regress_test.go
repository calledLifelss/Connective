package update

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// blockingProvider parks Check until released, so tests can observe
// manager behavior while a network phase is genuinely in flight.
type blockingProvider struct {
	entered chan struct{}
	release chan struct{}
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{entered: make(chan struct{}), release: make(chan struct{})}
}

func (p *blockingProvider) Name() string { return "blocking" }
func (p *blockingProvider) Check(ctx context.Context, q Query) (*Release, error) {
	close(p.entered)
	<-p.release
	return nil, ErrNoUpdate("released")
}
func (p *blockingProvider) OpenArtifact(ctx context.Context, a Artifact) (io.ReadCloser, error) {
	return nil, errors.New("no artifacts")
}

// H4: a check in flight must never hold m.mu across the network call.
// Status and Cancel have to answer promptly (the IPC client gives
// handlers 10s), and a cancel must stick — RunCheck may not resurrect
// the check afterwards.
func TestCheckDoesNotBlockStatusOrCancel(t *testing.T) {
	bp := newBlockingProvider()
	m := &Manager{
		Provider:       bp,
		DataDir:        t.TempDir(),
		Channel:        func() string { return ChannelStable },
		CurrentVersion: func() string { return "0.2.0" },
	}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	done := make(chan Status, 1)
	go func() { done <- m.Check(context.Background(), true) }()
	<-bp.entered

	statusCh := make(chan Status, 1)
	go func() { statusCh <- m.Status() }()
	select {
	case got := <-statusCh:
		if got.State != StateChecking {
			t.Fatalf("mid-check state = %s", got.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Status blocked behind an in-flight check")
	}

	cancelCh := make(chan Status, 1)
	go func() { cancelCh <- m.Cancel() }()
	select {
	case got := <-cancelCh:
		if got.State != StateCancelled {
			t.Fatalf("cancel mid-check = %s", got.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel blocked behind an in-flight check")
	}

	close(bp.release)
	select {
	case got := <-done:
		if got.State != StateCancelled {
			t.Fatalf("RunCheck resurrected a cancelled check: %s", got.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("check did not finish after release")
	}
}

// cancelWhileApply flips the manager to cancelled from inside the
// applier (the exact window Install guards: assemble succeeds but the
// user already backed out). ctx is untouched, so Assemble completes
// and the guard must discard its result instead of handing off.
type cancelWhileApply struct {
	m   *Manager
	saw chan struct{}
}

func (a *cancelWhileApply) Apply(ctx context.Context, currentDir, deltaPath, outDir string) error {
	if err := CopyTree(currentDir, outDir); err != nil {
		return err
	}
	a.m.mu.Lock()
	a.m.setState(StateCancelled, "")
	a.m.mu.Unlock()
	close(a.saw)
	return nil
}

// blockingApplier parks until released (or ctx dies), simulating a
// slow assemble under the real Cancel path.
type blockingApplier struct {
	entered chan struct{}
	release chan struct{}
}

func (a *blockingApplier) Apply(ctx context.Context, currentDir, deltaPath, outDir string) error {
	close(a.entered)
	select {
	case <-a.release:
	case <-ctx.Done():
	}
	return nil
}

// prodManager builds a production-shaped manager (TestApply=false,
// live install root, hooked spawn) with a delta+full release published.
// Returns the manager, the provider dir and the live install root.
func prodManager(t *testing.T, applier DeltaApplier) (*Manager, string, *Installer) {
	t.Helper()
	pub, priv, id := testKeys(t)
	current := "0.2.0"
	m, dir := testManager(t, &current, pub, id)
	m.TestApply = false
	m.Applier = applier

	binDir := t.TempDir()
	fakeDaemon := filepath.Join(binDir, "connectived")
	if err := os.WriteFile(fakeDaemon, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, updaterExeName()), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.DaemonExe = fakeDaemon

	live := &Installer{Root: filepath.Join(m.DataDir, "updates", "versions-root")}
	cur, _ := live.VersionDir("0.2.0")
	writeTree(t, cur, map[string]string{"connective": "v1", "keep": "k"})
	if _, err := live.Activate("0.2.0"); err != nil {
		t.Fatal(err)
	}

	fullSHA := putZip(t, dir, "full.zip", map[string]string{
		"connective": "v2-full-" + repeat("0123456789abcdef", 512),
		"extra-blob": repeat("0123456789abcdef", 512),
	})
	fullSt, _ := os.Stat(filepath.Join(dir, "full.zip"))
	deltaSHA := putZip(t, dir, "delta.zip", map[string]string{"connective": "v2-delta"})
	deltaSt, _ := os.Stat(filepath.Join(dir, "delta.zip"))
	fullSize, deltaSize := fullSt.Size(), deltaSt.Size()
	if deltaSize >= fullSize {
		deltaSize = fullSize / 2
		if deltaSize <= 0 {
			deltaSize = 1
		}
	}
	mm := Manifest{
		Schema: ManifestSchema, Version: "0.2.1", Channel: ChannelStable,
		MinVersion: "0.2.0", Platform: testPlatform(), Arch: testArch(),
		Artifacts: []Artifact{
			{Type: ArtifactFull, Filename: "full.zip", Size: fullSize, SHA256: fullSHA, URL: "full.zip"},
			{Type: ArtifactDelta, Filename: "delta.zip", Size: deltaSize, SHA256: deltaSHA, URL: "delta.zip", FromVersion: "0.2.0"},
		},
	}
	publish(t, dir, mm, id, priv)
	return m, dir, live
}

// stagingInstall runs Check+Download then Install in the background,
// returning a channel with the final status.
func stagingInstall(t *testing.T, m *Manager) chan Status {
	t.Helper()
	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s (%s)", got.State, got.Error)
	}
	if info := m.Status().Info; info == nil || info.ArtifactType != ArtifactDelta {
		t.Fatalf("delta should be selected, got %+v", info)
	}
	if got := m.Download(ctx); got.State != StateUpdateAvailable {
		t.Fatalf("download: %s (%s)", got.State, got.Error)
	}
	out := make(chan Status, 1)
	go func() { out <- m.Install(ctx) }()
	return out
}

// waitStatus waits for an Install result with a hard deadline.
func waitStatus(t *testing.T, ch chan Status) Status {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(20 * time.Second):
		t.Fatal("install did not finish")
		return Status{}
	}
}

// H5 (real Cancel): cancel during a parked assemble must abandon the
// install — no updater spawn, no pending contract, state cancelled.
func TestCancelDuringStagingNeverHandsOff(t *testing.T) {
	ap := &blockingApplier{entered: make(chan struct{}), release: make(chan struct{})}
	m, _, _ := prodManager(t, ap)
	var spawned bool
	m.Spawn = func(bin string, args []string, dd string, onExit func(error)) error {
		spawned = true
		return nil
	}

	out := stagingInstall(t, m)
	<-ap.entered
	if got := m.Cancel(); got.State != StateCancelled {
		t.Fatalf("cancel during staging = %s", got.State)
	}
	close(ap.release)
	got := waitStatus(t, out)
	if got.State != StateCancelled {
		t.Fatalf("install after cancel = %s (%s)", got.State, got.Error)
	}
	if spawned {
		t.Fatal("cancelled staging must never spawn the updater")
	}
	if _, err := os.Stat(pendingPath(m.DataDir)); !os.IsNotExist(err) {
		t.Fatal("cancelled staging must not write a pending contract")
	}
}

// H5 (guard with a completed assemble): if the state moved while the
// tree was being built, the assembled dir is discarded, not handed off.
func TestAssembledTreeDiscardedWhenCancelledMidFlight(t *testing.T) {
	m, _, _ := prodManager(t, nil) // default ZipOverlayApplier
	ca := &cancelWhileApply{m: m, saw: make(chan struct{})}
	m.Applier = ca
	var spawned bool
	m.Spawn = func(bin string, args []string, dd string, onExit func(error)) error {
		spawned = true
		return nil
	}

	out := stagingInstall(t, m)
	<-ca.saw
	got := waitStatus(t, out)
	if got.State != StateCancelled {
		t.Fatalf("install = %s (%s)", got.State, got.Error)
	}
	if spawned {
		t.Fatal("cancelled staging must never spawn the updater")
	}
	// The assemble completed: its tree under stage-root must be gone.
	stageRoot := filepath.Join(m.DataDir, "updates", "stage-root")
	if _, err := os.Stat(filepath.Join(stageRoot, "versions", "0.2.1")); !os.IsNotExist(err) {
		t.Fatal("assembled tree must be discarded after a mid-flight cancel")
	}
	// …and so must the verified download it was built from.
	if got := m.Status(); got.Staged {
		t.Fatal("abandoned staging must not report a staged artifact")
	}
}

// Cancelling while the downloaded artifact is being verified must
// discard the bytes: no staged flag, no file. The old flow kept the
// file and attempted an impossible cancelled→update-available
// transition, leaving a staged artifact behind a backed-out state.
func TestCancelDuringVerifyDiscardsStagedFile(t *testing.T) {
	m, _, _ := prodManager(t, nil)
	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s (%s)", got.State, got.Error)
	}

	prev := verifyHash
	verifyHash = func(path, want string) error {
		if st := m.Cancel(); st.State != StateCancelled {
			t.Errorf("cancel during verify = %s, want cancelled", st.State)
		}
		return prev(path, want)
	}
	t.Cleanup(func() { verifyHash = prev })

	got := m.Download(ctx)
	if got.State != StateCancelled {
		t.Fatalf("download after cancel = %s (%s), want cancelled", got.State, got.Error)
	}
	if got.Staged {
		t.Fatal("cancelled download must not report a staged artifact")
	}
	m.mu.Lock()
	staged := m.staged
	m.mu.Unlock()
	if staged != "" {
		t.Fatalf("staged path kept after cancel: %s", staged)
	}
	entries, err := os.ReadDir(filepath.Join(m.updatesDir(), "staging-dl"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("staging dir must be empty, found %d entries", len(entries))
	}
}

// M1: production delta assembles against the LIVE install root (the
// stage root has no current link) and lands a delta overlay, not a
// silent full fallback.
func TestProductionDeltaUsesLiveInstallRoot(t *testing.T) {
	m, _, _ := prodManager(t, nil)
	m.Spawn = func(bin string, args []string, dd string, onExit func(error)) error { return nil }

	got := waitStatus(t, stagingInstall(t, m))
	if got.State != StateRestarting {
		t.Fatalf("install: %s (%s)", got.State, got.Error)
	}
	var pend PendingUpdate
	if err := readJSON(pendingPath(m.DataDir), &pend); err != nil {
		t.Fatalf("pending not written: %v", err)
	}
	if pend.Staged == "" {
		t.Fatal("pending missing staged tree")
	}
	bin, err := os.ReadFile(filepath.Join(pend.Staged, "connective"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bin) != "v2-delta" {
		t.Fatalf("staged tree = %q, want delta content (full fallback)", bin)
	}
	keep, err := os.ReadFile(filepath.Join(pend.Staged, "keep"))
	if err != nil || string(keep) != "k" {
		t.Fatalf("delta base overlay lost untouched files: %q %v", keep, err)
	}
}

// M8: an updater that dies while we are waiting on it becomes a
// visible failure instead of a UI frozen on "restarting".
func TestUpdaterExitWhileRestartingFails(t *testing.T) {
	pub, priv, id := testKeys(t)
	current := "0.3.0"
	m, dir := testManager(t, &current, pub, id)
	m.TestApply = false
	binDir := t.TempDir()
	fakeDaemon := filepath.Join(binDir, "connectived")
	if err := os.WriteFile(fakeDaemon, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, updaterExeName()), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.DaemonExe = fakeDaemon
	var onExit func(error)
	m.Spawn = func(bin string, args []string, dd string, oe func(error)) error {
		onExit = oe
		return nil
	}
	sha := putZip(t, dir, "full.zip", map[string]string{"connective": "v2"})
	st, _ := os.Stat(filepath.Join(dir, "full.zip"))
	publish(t, dir, fullManifest(t, dir, "0.3.1", sha, st.Size()), id, priv)

	ctx := context.Background()
	if got := m.Check(ctx, true); got.State != StateUpdateAvailable {
		t.Fatalf("check: %s (%s)", got.State, got.Error)
	}
	if got := m.Download(ctx); got.State != StateUpdateAvailable {
		t.Fatalf("download: %s (%s)", got.State, got.Error)
	}
	if got := m.Install(ctx); got.State != StateRestarting {
		t.Fatalf("install: %s (%s)", got.State, got.Error)
	}
	if onExit == nil {
		t.Fatal("spawn never received the exit callback")
	}

	onExit(errors.New("exit status 1"))
	got := m.Status()
	if got.State != StateFailed || got.Error == "" {
		t.Fatalf("early updater exit = %s (%q), want failed with a message", got.State, got.Error)
	}

	// A withdrawn contract (no pending file) makes the exit irrelevant:
	// the updater died after we already cancelled — stay as we are.
	m2, _ := testManager(t, &current, pub, id)
	m2.mu.Lock()
	m2.status.State = StateRestarting // direct: the transition table has no idle→restarting
	m2.mu.Unlock()
	m2.onUpdaterExit(errors.New("boom"))
	if st := m2.Status().State; st != StateRestarting {
		t.Fatalf("exit without a pending contract = %s", st)
	}
}

// M5: the backoff shift is clamped, so a long failure streak still
// cools down instead of shifting into garbage.
func TestCoolingClampsHugeFailCount(t *testing.T) {
	m := &Manager{}
	base := time.Now()
	m.cache.FailCount = 40
	m.cache.LastCheckUnix = base.Unix() - 100 // 100s ago
	if !m.cooling(base) {
		t.Fatal("40 failures must still cool down (clamped to backoff max)")
	}
	m.cache.FailCount = 1
	if !m.cooling(base) {
		t.Fatal("first failure backoff (5m) must cover a 100s-old check")
	}
	if m.cooling(base.Add(10 * time.Minute)) {
		t.Fatal("first failure backoff must expire after 5m")
	}
}

// L1: progress derives a real speed (and therefore ETA) from observed
// byte deltas instead of staying pinned at zero.
func TestProgressSpeedFromByteDeltas(t *testing.T) {
	m := &Manager{}
	dl := m.downloader()
	dl.OnProgress(0, 100000)
	time.Sleep(350 * time.Millisecond)
	dl.OnProgress(10000, 100000)
	m.mu.Lock()
	p := m.status.Progress
	m.mu.Unlock()
	if p.SpeedBps <= 0 {
		t.Fatalf("speed = %d, want > 0", p.SpeedBps)
	}
	if p.ETASeconds <= 0 {
		t.Fatalf("eta = %d, want > 0", p.ETASeconds)
	}
	if p.Percent != 10 {
		t.Fatalf("percent = %v, want 10", p.Percent)
	}
}

// M4: a file:// retry restarts from scratch — the leftover partial
// temp file must not poison the hash or shrink the copy.
func TestFileAttemptDiscardsPartialTemp(t *testing.T) {
	dir := t.TempDir()
	content := []byte(repeat("0123456789abcdef", 500)) // 8000 bytes
	src := filepath.Join(dir, "asset.zip")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	fileURL := "file:///" + filepath.ToSlash(src)
	if !filepath.IsAbs(src) {
		t.Fatal("need abs path")
	}
	u, err := url.Parse(fileURL)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	tmp, err := os.CreateTemp(stage, "artifact-*.part")
	if err != nil {
		t.Fatal(err)
	}
	defer tmp.Close()
	if _, err := tmp.Write(content[:100]); err != nil { // stale partial
		t.Fatal(err)
	}
	dl := &Downloader{}
	done, total, sum, err := dl.attempt(context.Background(), u, tmp, 100, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if done != int64(len(content)) || total != int64(len(content)) {
		t.Fatalf("done=%d total=%d, want %d/%d", done, total, len(content), len(content))
	}
	if sum != sha256Of(content) {
		t.Fatal("resumed file attempt produced the wrong hash")
	}
	if st, _ := tmp.Stat(); st.Size() != int64(len(content)) {
		t.Fatalf("temp size = %d, want %d", st.Size(), len(content))
	}
}

// L3: extraction honors recorded unix modes; a mode-less (legacy)
// member keeps the historical 0755 so binaries never lose exec.
func TestExtractionHonorsRecordedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix modes are not observable on windows")
	}
	dir := t.TempDir()
	path, _ := writeZipWithMode(t, dir, map[string]os.FileMode{
		"docs.conf": 0o640,
		"legacy":    0, // legacy Create → reported 0666 → default 0755
	})
	out := t.TempDir()
	z, err := openZip(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if err := unzipOne(f, out); err != nil {
			t.Fatal(err)
		}
	}
	st, _ := os.Stat(filepath.Join(out, "docs.conf"))
	if got := st.Mode().Perm(); got != 0o640 {
		t.Fatalf("recorded mode 0640 extracted as %o", got)
	}
	st, _ = os.Stat(filepath.Join(out, "legacy"))
	if got := st.Mode().Perm(); got != 0o755 {
		t.Fatalf("legacy member extracted as %o, want 0755", got)
	}
}

// writeZipWithMode builds a zip whose members record the given modes
// (0 = legacy CreateHeader-less path reporting 0666).
func writeZipWithMode(t *testing.T, dir string, members map[string]os.FileMode) (string, string) {
	t.Helper()
	p := filepath.Join(dir, "modes.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, mode := range members {
		if mode == 0 {
			w, err := zw.Create(name) // legacy: no mode recorded
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			continue
		}
		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.SetMode(mode)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
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
	return p, sha256Of(raw)
}

// L4: a delta carrying the target listing drops files the new version
// deleted (and prunes the empty dirs they leave behind).
func TestDeltaListingPrunesDeletedFiles(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{
		"connective":       "v1",
		"keep":             "k",
		"gone/old.txt":     "bye",
		"removed-only.txt": "stale",
	})
	out := t.TempDir()
	deltaDir := t.TempDir()
	delta, _ := writeZipWithListing(t, deltaDir, map[string]string{
		"connective": "v2",
	}, []string{"connective", "keep"})
	if err := (ZipOverlayApplier{}).Apply(context.Background(), base, delta, out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "connective")); string(got) != "v2" {
		t.Fatalf("overlay missing: %q", got)
	}
	for _, gone := range []string{"removed-only.txt", filepath.Join("gone", "old.txt")} {
		if _, err := os.Stat(filepath.Join(out, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s must be pruned", gone)
		}
	}
	if st, err := os.Stat(filepath.Join(out, "gone")); !os.IsNotExist(err) && st.IsDir() {
		t.Fatal("empty directories must be pruned")
	}
	if _, err := os.Stat(filepath.Join(out, "keep")); err != nil {
		t.Fatalf("listed file lost: %v", err)
	}
}

// An unsafe listing path is rejected before anything is pruned.
func TestDeltaListingUnsafePathRejected(t *testing.T) {
	base := t.TempDir()
	writeTree(t, base, map[string]string{"a": "1"})
	out := t.TempDir()
	deltaDir := t.TempDir()
	delta, _ := writeZipWithListing(t, deltaDir, map[string]string{"a": "2"}, []string{"a", "../evil"})
	if err := (ZipOverlayApplier{}).Apply(context.Background(), base, delta, out); err == nil {
		t.Fatal("listing with traversal must be rejected")
	}
}

// writeZipWithListing builds a delta zip: changed files plus the
// .connective-delta-files target listing.
func writeZipWithListing(t *testing.T, dir string, files map[string]string, listing []string) (string, string) {
	t.Helper()
	p := filepath.Join(dir, "delta.zip")
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
	lw, err := zw.Create(DeltaListingName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lw.Write([]byte(strings.Join(listing, "\n"))); err != nil {
		t.Fatal(err)
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
	return p, sha256Of(raw)
}
