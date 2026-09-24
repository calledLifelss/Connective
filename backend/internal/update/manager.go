package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"connective/backend/internal/persistence"
)

// Check policy.
const (
	CheckInterval   = 6 * time.Hour
	backoffBase     = 5 * time.Minute
	backoffMax      = 24 * time.Hour
	minCheckSpacing = 60 * time.Second
)

// Logger is the minimal logging surface the manager needs.
type Logger interface {
	Info(f string, a ...any)
	Warn(f string, a ...any)
	Error(f string, a ...any)
}

// CheckCache persists query memory so the UI never triggers network
// traffic just by opening the Dashboard.
type CheckCache struct {
	LastCheckUnix   int64  `json:"lastCheckUnix"`
	LastSuccessUnix int64  `json:"lastSuccessUnix"`
	LastSeenVersion string `json:"lastSeenVersion,omitempty"`
	Channel         string `json:"channel,omitempty"`
	Dismissed       string `json:"dismissedVersion,omitempty"`
	FailCount       int    `json:"failCount"`
}

// UpdateManager owns the whole flow: check → download → verify → stage →
// install → restart, with cancellation, backoff and rollback. It speaks
// only to ReleaseProvider, so sources are interchangeable.
type Manager struct {
	Provider ReleaseProvider
	Keys     TrustedKeys

	// DataDir roots staging (updates/) and the check cache.
	DataDir string

	// DaemonExe is the running connectived binary (os.Executable).
	// Production resolves the install root + updater from it; empty
	// means "no updater here" (tests without the hook below).
	DaemonExe string

	// Spawn launches the updater detached. Nil = the platform default
	// (pkexec/UAC elevation). Tests override it to capture argv.
	Spawn func(bin string, args []string, dataDir string) error

	CurrentVersion func() string // default: CurrentVersion const
	Channel        func() string // daemon: settings snapshot
	AutoCheck      func() bool

	Log     Logger
	OnEvent func(Status)

	Now func() time.Time

	// TestApply performs activation in-process against TestRoot with a
	// simulated health signal. Production leaves this false and hands
	// activation to the updater process. Never self-replaces in tests.
	TestApply bool
	TestRoot  string

	Downloader *Downloader
	Applier    DeltaApplier

	mu        sync.Mutex
	status    Status
	cancel    context.CancelFunc
	cache     CheckCache
	store     *persistence.Store
	pending   *Release  // checked, verified release
	artifact  *Artifact // selected payload
	staged    string    // verified artifact path
	assembled string    // assembled target tree
	lastPoll  time.Time
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Manager) current() string {
	if m.CurrentVersion != nil {
		return m.CurrentVersion()
	}
	return CurrentVersion
}

func (m *Manager) channel() string {
	if m.Channel != nil {
		if c := m.Channel(); ValidChannel(c) {
			return c
		}
	}
	return ChannelStable
}

func (m *Manager) platform() (string, string) {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindows, ArchX8664
	default:
		arch := ArchX8664
		if runtime.GOARCH == "arm64" {
			arch = ArchAARCH64
		}
		return PlatformLinux, arch
	}
}

func (m *Manager) log() Logger { return m.Log }

func (m *Manager) updatesDir() string {
	return filepath.Join(m.DataDir, "updates")
}

// Init loads the persisted check cache (missing = fresh defaults).
func (m *Manager) Init() error {
	st, err := persistence.New(m.DataDir)
	if err != nil {
		return err
	}
	m.store = st
	var c CheckCache
	if _, err := st.Load(persistence.DocUpdateCache, &c); err != nil {
		return err
	}
	m.cache = c
	m.status = Status{State: StateIdle, Channel: m.channel(), Current: m.current()}
	m.reconcileBoot()
	return nil
}

// reconcileBoot consumes leftover handoff files (see handoff.go) and
// seeds one boot announcement: updated when the updater's result
// matches this build, failed when it reports failure. A result-less
// pending is cleared silently (updater never ran — retry is offered
// fresh instead of wedging).
func (m *Manager) reconcileBoot() {
	seed := reconcileBoot(m.DataDir)
	if seed == nil {
		return
	}
	m.pending = &Release{Manifest: SignedManifest{
		Manifest: Manifest{Version: seed.version},
	}}
	m.artifact = nil
	m.staged = ""
	m.assembled = ""
	if seed.ok {
		m.cache.LastSeenVersion = seed.version
		m.saveCache()
		m.setState(StateUpdated, "")
		if m.log() != nil {
			m.log().Info("update: %s active after restart", seed.version)
		}
		return
	}
	m.setState(StateFailed, ifEmpty(seed.errMsg, failedSpawnError()))
	if m.log() != nil {
		m.log().Warn("update: installer reported failure: %v", seed.errMsg)
	}
}

func ifEmpty(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func (m *Manager) saveCache() {
	if m.store == nil {
		return
	}
	_ = m.store.Save(persistence.DocUpdateCache, &m.cache)
}

func (m *Manager) setState(to string, errMsg string) {
	from := m.status.State
	if terr := Transition(from, to); terr != nil {
		if m.log() != nil {
			m.log().Warn("update: %v", terr)
		}
		return
	}
	m.status.State = to
	m.status.Error = errMsg
	if m.OnEvent != nil {
		// Synchronous delivery: spawning a goroutine per event
		// reorders the stream (a stale snapshot can overwrite a newer
		// one). Broadcast itself is non-blocking, so holding mu here
		// cannot stall the manager.
		m.OnEvent(m.snapshot())
	}
}

func (m *Manager) snapshot() Status {
	s := m.status
	if m.pending != nil {
		pm := m.pending.Manifest.Manifest
		s.Info = &Info{
			Version:      pm.Version,
			Channel:      pm.Channel,
			ReleaseDate:  pm.ReleaseDate,
			ReleaseNotes: pm.ReleaseNotes,
			MinVersion:   pm.MinVersion,
		}
		if m.artifact != nil {
			s.Info.SizeBytes = m.artifact.Size
			s.Info.ArtifactType = m.artifact.Type
		}
	}
	s.Channel = m.channel()
	s.Current = m.current()
	s.LastCheckUnix = m.cache.LastCheckUnix
	s.Staged = m.staged != ""
	return s
}

// Status returns a UI-ready snapshot.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshot()
}

// cooldown reports whether failures demand silence (manual checks bypass).
func (m *Manager) cooling(now time.Time) bool {
	if m.cache.FailCount <= 0 {
		return false
	}
	backoff := backoffBase << (m.cache.FailCount - 1)
	if backoff > backoffMax {
		backoff = backoffMax
	}
	return now.Unix()-m.cache.LastCheckUnix < int64(backoff.Seconds())
}

// Check queries the provider (manual bypasses cache/backoff/spacing).
func (m *Manager) Check(ctx context.Context, manual bool) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if !manual {
		if now.Sub(m.lastPoll) < minCheckSpacing {
			return m.snapshot()
		}
		if now.Unix()-m.cache.LastCheckUnix < int64(CheckInterval.Seconds()) {
			return m.snapshot()
		}
		if m.cooling(now) {
			return m.snapshot()
		}
	}
	m.lastPoll = now
	if m.status.State != StateIdle && m.status.State != StateNoUpdate &&
		m.status.State != StateUpdated && m.status.State != StateCancelled &&
		m.status.State != StateFailed && m.status.State != StateRolledBack &&
		m.status.State != StateUpdateAvailable {
		return m.snapshot() // busy: never stack checks
	}
	m.setState(StateChecking, "")
	m.cache.LastCheckUnix = now.Unix()
	m.saveCache()

	plat, arch := m.platform()
	q := Query{Channel: m.channel(), Platform: plat, Arch: arch, CurrentVersion: m.current()}
	if m.log() != nil {
		m.log().Info("update: check via %s channel=%s %s/%s current=%s",
			m.Provider.Name(), q.Channel, q.Platform, q.Arch, q.CurrentVersion)
	}
	rel, err := m.Provider.Check(ctx, q)
	if err != nil {
		if IsNoUpdate(err) || err == ErrNotConfigured {
			m.setState(StateNoUpdate, "")
			if m.log() != nil {
				m.log().Info("update: no update (%v)", err)
			}
			return m.snapshot()
		}
		m.cache.FailCount++
		m.saveCache()
		m.setState(StateFailed, friendlyCheck(err))
		if m.log() != nil {
			m.log().Warn("update: check failed: %v", err)
		}
		return m.snapshot()
	}
	if err := m.Keys.VerifyManifest(&rel.Manifest); err != nil {
		m.cache.FailCount++
		m.saveCache()
		m.setState(StateFailed, "This update could not be verified and was not installed.")
		if m.log() != nil {
			m.log().Error("update: manifest trust failed: %v", err)
		}
		return m.snapshot()
	}
	art, err := SelectArtifact(m.current(), rel.Manifest.Manifest)
	if err != nil {
		m.setState(StateFailed, "This update could not be verified and was not installed.")
		if m.log() != nil {
			m.log().Error("update: artifact selection: %v", err)
		}
		return m.snapshot()
	}
	m.pending = rel
	m.artifact = art
	m.staged = ""
	m.assembled = ""
	m.cache.FailCount = 0
	m.cache.LastSuccessUnix = now.Unix()
	m.cache.LastSeenVersion = rel.Manifest.Manifest.Version
	m.saveCache()
	if m.cache.Dismissed == rel.Manifest.Manifest.Version {
		m.setState(StateNoUpdate, "")
	} else {
		m.setState(StateUpdateAvailable, "")
	}
	if m.log() != nil {
		m.log().Info("update: %s available (%s %d bytes) via %s",
			rel.Manifest.Manifest.Version, art.Type, art.Size, rel.Source)
	}
	return m.snapshot()
}

// MaybeCheckOnStartup runs the background policy: auto-check enabled
// and the last check sufficiently old. Never blocks startup.
func (m *Manager) MaybeCheckOnStartup() {
	auto := true
	if m.AutoCheck != nil {
		auto = m.AutoCheck()
	}
	if !auto {
		return
	}
	m.mu.Lock()
	stale := m.now().Unix()-m.cache.LastCheckUnix >= int64(CheckInterval.Seconds())
	m.mu.Unlock()
	if !stale {
		return
	}
	go m.Check(context.Background(), false)
}

// Download fetches + verifies the selected artifact into staging.
func (m *Manager) Download(ctx context.Context) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != StateUpdateAvailable || m.pending == nil || m.artifact == nil {
		return m.snapshot()
	}
	m.setState(StateDownloading, "")
	art := *m.artifact
	dl := m.downloader()
	cctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()
	path, _, ferr := m.fetch(cctx, dl, art, filepath.Join(m.updatesDir(), "staging-dl"))
	m.mu.Lock()
	m.cancel = nil
	if ferr != nil {
		if cctx.Err() != nil {
			m.setState(StateCancelled, "")
		} else {
			m.setState(StateFailed, "The download failed and was discarded.")
			if m.log() != nil {
				m.log().Warn("update: download: %v", ferr)
			}
		}
		return m.snapshot()
	}
	m.setState(StateVerifying, "")
	m.mu.Unlock()
	verr := m.verifyStaged(path, art)
	m.mu.Lock()
	if verr != nil {
		m.setState(StateFailed, "This update could not be verified and was not installed.")
		return m.snapshot()
	}
	m.staged = path
	m.status.Progress = progressOf(art.Size, art.Size, 0)
	m.setState(StateUpdateAvailable, "")
	if m.log() != nil {
		m.log().Info("update: %s verified (%d bytes), awaiting install",
			art.Filename, art.Size)
	}
	return m.snapshot()
}

// downloader returns the configured downloader with live progress wired
// to status events.
func (m *Manager) downloader() *Downloader {
	dl := m.Downloader
	if dl == nil {
		dl = &Downloader{}
	}
	dl.OnProgress = func(done, total int64) bool {
		m.mu.Lock()
		m.status.Progress = progressOf(done, total, 0)
		emit := m.snapshot()
		m.mu.Unlock()
		if m.OnEvent != nil {
			m.OnEvent(emit)
		}
		return true
	}
	return dl
}

// verifyStaged hash-checks a staged artifact, discarding bad bytes.
func (m *Manager) verifyStaged(path string, art Artifact) error {
	if verr := VerifySHA256(path, art.SHA256); verr != nil {
		os.Remove(path)
		if m.log() != nil {
			m.log().Error("update: hash mismatch, staged artifact discarded")
		}
		return verr
	}
	return nil
}

// fetchVerified downloads art and hash-verifies it, discarding bad bytes.
// Callers must NOT hold m.mu (progress callbacks take it).
func (m *Manager) fetchVerified(ctx context.Context, dl *Downloader, art Artifact) (string, error) {
	path, _, err := m.fetch(ctx, dl, art, filepath.Join(m.updatesDir(), "staging-dl"))
	if err != nil {
		return "", err
	}
	if err := m.verifyStaged(path, art); err != nil {
		return "", err
	}
	return path, nil
}

// fetch downloads via HTTP(S) or provider stream for local URLs.
func (m *Manager) fetch(ctx context.Context, dl *Downloader, art Artifact, stageDir string) (string, string, error) {
	if strings.Contains(art.URL, "://") && !strings.HasPrefix(art.URL, "file://") {
		return dl.Fetch(ctx, art.URL, stageDir, art.Size)
	}
	rc, err := m.Provider.OpenArtifact(ctx, art)
	if err != nil {
		return "", "", err
	}
	defer rc.Close()
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return "", "", err
	}
	tmp, err := os.CreateTemp(stageDir, "artifact-*.part")
	if err != nil {
		return "", "", err
	}
	name := tmp.Name()
	h := sha256.New()
	var done int64
	buf := make([]byte, 64*1024)
	cap := dl.maxBytes()
	for {
		select {
		case <-ctx.Done():
			tmp.Close()
			os.Remove(name)
			return "", "", ctx.Err()
		default:
		}
		n, rerr := rc.Read(buf)
		if n > 0 {
			if done+int64(n) > cap {
				tmp.Close()
				os.Remove(name)
				return "", "", fmt.Errorf("update: artifact exceeds %d bytes", cap)
			}
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				os.Remove(name)
				return "", "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if dl.OnProgress != nil {
				dl.OnProgress(done, art.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			os.Remove(name)
			return "", "", rerr
		}
	}
	tmp.Close()
	return name, hex.EncodeToString(h.Sum(nil)), nil
}

// Install assembles and activates the verified download. Production
// hands activation to the updater process; TestApply performs it
// in-process against TestRoot with a simulated health signal.
func (m *Manager) Install(ctx context.Context) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != StateUpdateAvailable || m.pending == nil || m.artifact == nil || m.staged == "" {
		return m.snapshot()
	}
	m.setState(StateStaging, "")
	ver := m.pending.Manifest.Manifest.Version
	root := m.TestRoot
	if root == "" {
		root = filepath.Join(m.updatesDir(), "versions-root")
	}
	installer := &Installer{Root: root}
	kind := m.artifact.Type
	applier := m.Applier
	if applier == nil {
		applier = ZipOverlayApplier{}
	}
	// Snapshot shared pointers/paths under lock: the mutex is released
	// for the slow assemble/fetch below, and Check/Download/Cancel must
	// never mutate pending mid-install (guarded by state), but copying
	// avoids any future race if guards change.
	pending := m.pending
	staged := m.staged
	m.mu.Unlock()
	dir, aerr := installer.Assemble(ctx, ver, staged, kind, applier)
	if aerr != nil && kind == ArtifactDelta {
		// Delta inapplicable (no current tree, corrupt patch, …):
		// fall back to the full artifact instead of failing.
		if m.log() != nil {
			m.log().Warn("update: delta unusable (%v), falling back to full", aerr)
		}
		if full := fullArtifactOf(pending); full != nil {
			if fpath, ferr := m.fetchVerified(ctx, m.downloader(), *full); ferr == nil {
				m.mu.Lock()
				m.staged = fpath
				m.artifact = full
				m.mu.Unlock()
				kind = ArtifactFull
				dir, aerr = installer.Assemble(ctx, ver, fpath, kind, applier)
			} else {
				aerr = ferr
			}
		}
	}
	m.mu.Lock()
	if aerr != nil {
		m.setState(StateFailed, "The update could not be installed. Your current version is still safe.")
		if m.log() != nil {
			m.log().Error("update: assemble: %v", aerr)
		}
		return m.snapshot()
	}
	m.assembled = dir
	m.setState(StateInstalling, "")
	if !m.TestApply {
		// Production: hand activation to the detached updater and
		// move to restarting — the app must close so locked files
		// (Windows) and the running tree can be replaced. The
		// updater reports via updates/result.json, consumed at the
		// next boot (see reconcileBoot). Nothing here may block.
		if herr := m.handoff(dir, ver); herr != nil {
			os.Remove(pendingPath(m.DataDir))
			m.setState(StateFailed, friendlySpawnError(herr))
			if m.log() != nil {
				m.log().Error("update: installer handoff: %v", herr)
			}
			return m.snapshot()
		}
		m.setState(StateRestarting, "")
		if m.log() != nil {
			m.log().Info("update: %s staged, installer started — restart to finish", ver)
		}
		return m.snapshot()
	}
	m.setState(StateRestarting, "")
	m.mu.Unlock()
	prev, actErr := installer.Activate(ver)
	m.mu.Lock()
	if actErr != nil {
		m.setState(StateFailed, "The update could not be installed. Your current version is still safe.")
		return m.snapshot()
	}
	healthy := installer.AwaitHealthy(ctx, func() bool {
		return installer.Current() == ver
	})
	if healthy != nil {
		if _, rerr := installer.Rollback(); rerr != nil {
			m.setState(StateFailed, "The update could not be installed and rollback failed. See logs.")
			if m.log() != nil {
				m.log().Error("update: rollback failed: %v", rerr)
			}
			return m.snapshot()
		}
		m.setState(StateRolledBack, "")
		if m.log() != nil {
			m.log().Warn("update: health failed, rolled back to %s", prev)
		}
		return m.snapshot()
	}
	m.setState(StateUpdated, "")
	if m.log() != nil {
		m.log().Info("update: %s active (prev %s)", ver, prev)
	}
	return m.snapshot()
}

// handoff writes the updater contract and spawns the detached
// installer. dir is the assembled tree (kept for the log only).
func (m *Manager) handoff(dir, ver string) error {
	root := resolveInstallRoot(m.DaemonExe, m.DataDir)
	bin := resolveUpdaterBin(m.DaemonExe, root)
	if bin == "" {
		return fmt.Errorf("update: installer binary not found")
	}
	pend := PendingUpdate{Version: ver, Root: root, Updater: bin, AtUnix: m.now().Unix()}
	if err := writeJSON(pendingPath(m.DataDir), pend); err != nil {
		return err
	}
	spawn := m.Spawn
	if spawn == nil {
		spawn = spawnUpdaterDetached
	}
	return spawn(bin, spawnArgs(bin, root, ver, resultPath(m.DataDir)), m.DataDir)
}

// Cancel aborts an in-flight download/install step.
func (m *Manager) Cancel() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	switch m.status.State {
	case StateDownloading, StateVerifying, StateStaging, StateChecking:
		m.setState(StateCancelled, "")
	}
	return m.snapshot()
}

// Dismiss hides the available update until a NEWER version appears.
func (m *Manager) Dismiss() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != StateUpdateAvailable || m.pending == nil {
		return m.snapshot()
	}
	m.cache.Dismissed = m.pending.Manifest.Manifest.Version
	m.saveCache()
	m.setState(StateIdle, "")
	return m.snapshot()
}

func progressOf(done, total, speed int64) Progress {
	p := Progress{BytesDone: done, BytesTotal: total, SpeedBps: speed}
	if total > 0 {
		p.Percent = float64(done) * 100 / float64(total)
		if p.Percent > 100 {
			p.Percent = 100
		}
		if speed > 0 && done < total {
			p.ETASeconds = (total - done) / speed
		}
	}
	return p
}

// friendlyCheck keeps offline/no-network failures calm for the UI.
func friendlyCheck(err error) string {
	if IsRateLimit(err) {
		return "GitHub request limit reached. Trying again later."
	}
	msg := err.Error()
	if strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") {
		return "Couldn't check for updates. We'll try again later."
	}
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}
