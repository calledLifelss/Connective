package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connective/backend/internal/ipc"
	"connective/backend/internal/update"
)

// errUpdateNotReady is what handlers return before the manager exists
// (startup window). It used to be context.DeadlineExceeded, which read
// like a timeout to every caller.
var errUpdateNotReady = fmt.Errorf("update manager is not ready yet")

// Update provider selection. Empty (production today) means the GitHub
// provider skeleton, which reports "not configured" so checks stay
// quiet. Tests/dev point at a fixture directory:
//
//	CONNECTIVE_UPDATE_PROVIDER=dir:/path/to/fixtures
//
// Test installs apply in-process against a temp root:
//
//	CONNECTIVE_UPDATE_TEST_APPLY=1
//
// Production update source. Env overrides exist for tests/dev; an
// empty environment means the real GitHub repository, anonymously.
func updateProviderFromEnv() update.ReleaseProvider {
	spec := strings.TrimSpace(os.Getenv("CONNECTIVE_UPDATE_PROVIDER"))
	if rest, ok := strings.CutPrefix(spec, "dir:"); ok && rest != "" {
		return update.DirProvider{Dir: rest}
	}
	if rest, ok := strings.CutPrefix(spec, "file:"); ok && rest != "" {
		return update.DirProvider{Dir: rest}
	}
	return update.GitHubProvider{Owner: "calledLifelss", Repo: "Connective"}
}

// trustedKeysFromEnv loads {key_id: hexpub} trust roots. A dev/test
// file overrides the embedded production roots when set; otherwise the
// embedded release key applies. Absent everywhere means fail-closed
// verification. The path (never key material) may be logged, and a
// configured-but-broken file returns an error instead of silently
// degrading to "no keys" (every check would fail with a misleading
// "not configured" style message).
func trustedKeysFromEnv() (update.TrustedKeys, error) {
	path := strings.TrimSpace(os.Getenv("CONNECTIVE_UPDATE_TRUSTED_KEYS"))
	if path == "" {
		return update.DefaultTrustedKeys(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return update.TrustedKeys{}, fmt.Errorf("read trusted keys %s: %w", path, err)
	}
	var hexKeys map[string]string
	if err := json.Unmarshal(raw, &hexKeys); err != nil {
		return update.TrustedKeys{}, fmt.Errorf("parse trusted keys %s: %w", path, err)
	}
	keys := map[string]ed25519.PublicKey{}
	for id, hx := range hexKeys {
		b, err := hex.DecodeString(strings.TrimSpace(hx))
		if err != nil || len(b) != ed25519.PublicKeySize {
			continue
		}
		keys[id] = ed25519.PublicKey(b)
	}
	if len(keys) == 0 {
		return update.TrustedKeys{}, fmt.Errorf("trusted keys %s has no usable key", path)
	}
	return update.TrustedKeys{Keys: keys}, nil
}

// initUpdates builds the UpdateManager after settings load. The manager
// owns all update state; handlers below are thin async wrappers so slow
// network phases never block the 10s IPC call window — progress arrives
// as event.update broadcasts.
func (d *Daemon) initUpdates() {
	d.mu.Lock()
	dataDir := d.dataDir
	d.mu.Unlock()
	keys, keysErr := trustedKeysFromEnv()
	if keysErr != nil {
		// Fail closed, but say WHY: an unreadable trust root must not
		// masquerade as "no update configured".
		d.log.Error("update: trusted keys unavailable (checks will fail closed): %v", keysErr)
	}
	m := &update.Manager{
		Provider: updateProviderFromEnv(),
		Keys:     keys,
		DataDir:  dataDir,
		DaemonExe: func() string {
			exe, err := os.Executable()
			if err != nil {
				return ""
			}
			return exe
		}(),
		Channel: func() string {
			d.mu.Lock()
			defer d.mu.Unlock()
			return d.settings.UpdateChannel
		},
		AutoCheck: func() bool {
			d.mu.Lock()
			defer d.mu.Unlock()
			return d.settings.UpdateAutoCheck
		},
		Log: d.log,
		OnEvent: func(s update.Status) {
			// Defensive: events fire during Init, before (or without)
			// a live server. Broadcasting on nil would panic the daemon.
			if srv := d.ipc; srv != nil {
				srv.Broadcast(ipc.EventUpdate, s)
			}
		},
		// In-process activation is a test/dev path only: it requires a
		// fixture provider, so an unset CONNECTIVE_UPDATE_PROVIDER can
		// never divert a production install away from the updater.
		TestApply: os.Getenv("CONNECTIVE_UPDATE_TEST_APPLY") == "1" &&
			os.Getenv("CONNECTIVE_UPDATE_PROVIDER") != "",
		TestRoot: filepath.Join(dataDir, "updates", "test-root"),
	}
	if err := m.Init(); err != nil {
		d.log.Warn("update manager init failed: %v", err)
	}
	d.mu.Lock()
	d.upd = m
	d.mu.Unlock()
}

func (d *Daemon) updateManager() *update.Manager {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.upd
}

// hUpdateCheck starts a manual check asynchronously and returns the
// current snapshot; completion arrives via event.update. The guarded
// begin runs synchronously so the response already says "checking"
// (the old go-race could return the pre-check snapshot).
func (d *Daemon) hUpdateCheck(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return nil, errUpdateNotReady
	}
	if q, ok := m.StartCheck(true); ok {
		go m.RunCheck(context.Background(), q)
	}
	return m.Status(), nil
}

func (d *Daemon) hUpdateStatus(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return update.Status{State: update.StateIdle}, nil
	}
	return m.Status(), nil
}

func (d *Daemon) hUpdateDownload(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return nil, errUpdateNotReady
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		m.Download(ctx)
	}()
	return m.Status(), nil
}

func (d *Daemon) hUpdateCancel(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return update.Status{State: update.StateIdle}, nil
	}
	return m.Cancel(), nil
}

// hUpdateInstall stages and activates. Production hands activation to
// the updater process; test mode applies in-process. Slow phases run
// async with event.update progress.
func (d *Daemon) hUpdateInstall(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return nil, errUpdateNotReady
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		m.Install(ctx)
	}()
	return m.Status(), nil
}

func (d *Daemon) hUpdateDismiss(p json.RawMessage) (any, error) {
	m := d.updateManager()
	if m == nil {
		return update.Status{State: update.StateIdle}, nil
	}
	return m.Dismiss(), nil
}
