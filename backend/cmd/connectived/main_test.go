package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connective/backend/internal/connection"
	"connective/backend/internal/ipc"
	"connective/backend/internal/logging"
	"connective/backend/internal/persistence"
	"connective/backend/internal/servers"
	"connective/backend/internal/settings"
	"connective/backend/internal/subscriptions"
)

func testDaemon(t *testing.T) *Daemon {
	t.Helper()
	store, err := persistence.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Daemon{
		log:      logging.New(logging.DEBUG, 100),
		store:    store,
		dataDir:  t.TempDir(),
		machine:  connection.NewMachine(),
		settings: settings.Defaults(),
		sel:      selection{Auto: true},
		ipc:      ipc.NewServer(),
	}
}

func call(t *testing.T, d *Daemon, h func(json.RawMessage) (any, error), payload any) any {
	t.Helper()
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	out, err := h(raw)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return out
}

func callErr(t *testing.T, d *Daemon, h func(json.RawMessage) (any, error), payload any) error {
	t.Helper()
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	_, err := h(raw)
	return err
}

const testVLESS = "vless://11111111-2222-4333-8444-555555555555@10.0.0.1:443?encryption=none#A"

func TestServerCRUD(t *testing.T) {
	d := testDaemon(t)
	added := call(t, d, d.hAddServer, map[string]string{"link": testVLESS}).(*servers.Server)
	if added.Address != "10.0.0.1" {
		t.Fatalf("bad add: %+v", added)
	}
	if err := callErr(t, d, d.hAddServer, map[string]string{"link": "http://x"}); err == nil {
		t.Fatalf("expected import rejection")
	}

	// Update preserves test results.
	added.LatencyMs = 42
	upd := *added
	upd.Name = "Renamed"
	got := call(t, d, d.hUpdateServer, map[string]any{"server": upd}).(*servers.Server)
	if got.Name != "Renamed" || got.LatencyMs != 42 {
		t.Fatalf("bad update: %+v", got)
	}

	// Duplicate.
	dup := call(t, d, d.hDuplicateServer, map[string]string{"id": added.ID}).(*servers.Server)
	if dup.ID == added.ID || !strings.Contains(dup.Name, "copy") {
		t.Fatalf("bad duplicate: %+v", dup)
	}

	// Export round-trips.
	exp := call(t, d, d.hExportServer, map[string]string{"id": added.ID}).(map[string]string)
	if !strings.HasPrefix(exp["link"], "vless://") {
		t.Fatalf("bad export: %v", exp)
	}

	// Remove duplicate (local); removing the original while active fails.
	d.machine.SetServer(added.ID)
	_ = d.machine.Transition(connection.StSelecting, "t")
	_ = d.machine.Transition(connection.StConnecting, "t")
	_ = d.machine.Transition(connection.StStartingCore, "t")
	_ = d.machine.Transition(connection.StInitializingTUN, "t")
	_ = d.machine.Transition(connection.StApplyingRouting, "t")
	_ = d.machine.Transition(connection.StConnected, "t")
	if err := callErr(t, d, d.hRemoveServer, map[string]string{"id": added.ID}); err == nil {
		t.Fatalf("expected active-server guard")
	}
	call(t, d, d.hRemoveServer, map[string]string{"id": dup.ID})
	if len(d.serverList) != 1 {
		t.Fatalf("expected 1 server left, got %d", len(d.serverList))
	}
}

func TestSubscriptionOwnedProtected(t *testing.T) {
	d := testDaemon(t)
	added := call(t, d, d.hAddServer, map[string]string{"link": testVLESS}).(*servers.Server)
	added.SubscriptionID = "sub1"
	if err := callErr(t, d, d.hRemoveServer, map[string]string{"id": added.ID}); err == nil {
		t.Fatalf("expected subscription-owned guard")
	}
}

func TestSubscriptionEditList(t *testing.T) {
	d := testDaemon(t)
	call(t, d, d.hAddSubscription, map[string]string{"name": "N", "url": "https://example.com/s"})
	list := call(t, d, d.hListSubscriptions, nil)
	raw, _ := json.Marshal(list)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil || len(items) != 1 {
		t.Fatalf("bad list: %v %v", items, err)
	}
	id := items[0]["id"].(string)
	off := false
	call(t, d, d.hEditSubscription, map[string]any{"id": id, "name": "R", "enabled": off})
	if d.subs[0].Name != "R" || d.subs[0].Enabled {
		t.Fatalf("bad edit: %+v", d.subs[0])
	}
	if err := callErr(t, d, d.hEditSubscription, map[string]any{"id": id, "url": "ftp://x"}); err == nil {
		t.Fatalf("expected url validation")
	}
}

func TestUIClear(t *testing.T) {
	d := testDaemon(t)
	call(t, d, d.hUIUpdate, map[string]any{"expanded": map[string]bool{"a": true}})
	got := call(t, d, d.hUIGet, nil).(map[string]any)
	if got["expanded"] == nil {
		t.Fatalf("ui state not persisted: %v", got)
	}
	d.log.Info("x")
	call(t, d, d.hClearLogs, nil)
	if len(d.log.Recent(10, logging.DEBUG)) != 0 {
		t.Fatalf("logs not cleared")
	}
}

// TestRefreshWhileConnectedNoDeadlock is the regression test for the
// refreshSubscriptions self-deadlock (it called proxyURL while holding
// d.mu, wedging every later handler once connected). A subscription
// refresh while connected must complete, and concurrent state reads
// must answer.
func TestRefreshWhileConnectedNoDeadlock(t *testing.T) {
	d := testDaemon(t)
	// Fake a subscription served over HTTP.
	srv := newTestSubServer(t, testVLESS+"\n")
	sub := &subscriptions.Subscription{
		ID: "s1", Name: "S", URL: srv.URL, Enabled: true,
	}
	d.subs = []*subscriptions.Subscription{sub}
	// Drive the machine to connected (transitions only; no core).
	for _, st := range []connection.State{
		connection.StSelecting, connection.StConnecting,
		connection.StStartingCore, connection.StInitializingTUN,
		connection.StApplyingRouting, connection.StConnected,
	} {
		if err := d.machine.Transition(st, "test"); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 2)
	go func() {
		_, err := d.refreshSubscriptions("")
		done <- err
	}()
	go func() {
		_, err := d.hGetState(nil)
		done <- err
	}()
	timeout := time.After(20 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("refresh/state failed: %v", err)
			}
		case <-timeout:
			t.Fatalf("deadlock: refreshSubscriptions/state did not return")
		}
	}
}

func newTestSubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// processGone classifies a core PID for the elevated-stop fallback: our
// own live pid is "alive but signalable" (needs the helper only if Stop
// somehow left it), a reaped child is gone.
func TestProcessGone(t *testing.T) {
	if err := processGone(os.Getpid()); err != errUnreachable {
		t.Fatalf("live pid must be errUnreachable, got %v", err)
	}
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skip("no true binary")
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	if err := processGone(pid); err != nil {
		t.Fatalf("reaped pid must be gone, got %v", err)
	}
	// (pid <= 0 is rejected by ensureCoreGone before probing; pid 0
	// itself signals the process group and is not asserted here.)
}

// Explicit corePath always wins (fresh-install workaround must never
// override a deliberate user setting).
func TestSingBoxPathExplicit(t *testing.T) {
	d := testDaemon(t)
	d.settings.CorePath = "/opt/connective/sing-box"
	p, err := d.singBoxPath()
	if err != nil {
		t.Fatal(err)
	}
	if p != "/opt/connective/sing-box" {
		t.Fatalf("got %q", p)
	}
}

// With no corePath and no sing-box on PATH, the error must tell the
// user where to fix it (this is what fresh installs hit before the
// sibling-binary fallback existed).
func TestSingBoxPathMissing(t *testing.T) {
	d := testDaemon(t)
	t.Setenv("PATH", t.TempDir())
	if _, err := d.singBoxPath(); err == nil {
		t.Fatalf("expected not-found error")
	} else if !strings.Contains(err.Error(), "corePath") {
		t.Fatalf("error must mention corePath, got %q", err)
	}
}

// siblingBinary finds a bundled binary next to the daemon and ignores
// anything absent (fresh-install layout: /opt/connective/sing-box).
func TestSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "sing-box")
	if err := os.WriteFile(fake, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := siblingBinary(filepath.Join(dir, "connectived"), "sing-box"); got != fake {
		t.Fatalf("got %q, want %q", got, fake)
	}
	if got := siblingBinary(filepath.Join(dir, "connectived"), "nope"); got != "" {
		t.Fatalf("absent binary must yield empty, got %q", got)
	}
}

// B14/B15: settings.update rejects oversized or unnormalizable split
// lists instead of silently truncating/dropping them.
func TestUpdateSettingsSplitValidation(t *testing.T) {
	d := testDaemon(t)
	base := settings.Defaults()
	// Cap exceeded.
	base.SplitMode = settings.SplitBypass
	for i := 0; i <= settings.MaxSplitApps; i++ {
		base.SplitApps = append(base.SplitApps, "app")
	}
	if err := callErr(t, d, d.hUpdateSettings, base); err == nil {
		t.Fatal("oversized split list must be rejected")
	}
	// Unnormalizable entry.
	base = settings.Defaults()
	base.SplitMode = settings.SplitBypass
	base.SplitApps = []string{"firefox", "not a valid name"}
	err := callErr(t, d, d.hUpdateSettings, base)
	if err == nil || !strings.Contains(err.Error(), "invalid app entry") {
		t.Fatalf("invalid entry must be rejected, got %v", err)
	}
	// Valid list passes and stays normalized.
	base = settings.Defaults()
	base.SplitMode = settings.SplitBypass
	base.SplitApps = []string{"C:\\Program Files\\Mozilla Firefox\\firefox.exe"}
	out := call(t, d, d.hUpdateSettings, base)
	got := out.(settings.Settings)
	if len(got.SplitApps) != 1 || got.SplitApps[0] != "firefox" {
		t.Fatalf("split apps not normalized: %v", got.SplitApps)
	}
}

// B4: kill switch + split tunneling rejected by settings.update.
func TestUpdateSettingsConflict(t *testing.T) {
	d := testDaemon(t)
	s := settings.Defaults()
	s.KillSwitch = true
	s.SplitMode = settings.SplitOnly
	s.SplitApps = []string{"firefox"}
	err := callErr(t, d, d.hUpdateSettings, s)
	if err == nil || !strings.Contains(err.Error(), "kill switch") {
		t.Fatalf("conflict must be rejected with a clear error, got %v", err)
	}
}

// B2: applied markers persist and clear per-flag.
func TestAppliedStateLifecycle(t *testing.T) {
	d := testDaemon(t)
	d.setApplied(true, false)
	d.setApplied(false, true)
	d.mu.Lock()
	if !d.appliedKill || !d.appliedTun {
		d.mu.Unlock()
		t.Fatal("both flags must be set")
	}
	d.mu.Unlock()
	raw, err := os.ReadFile(d.appliedPath())
	if err != nil {
		t.Fatalf("marker must exist: %v", err)
	}
	var st struct {
		KillSwitch bool `json:"killSwitch"`
		Tun        bool `json:"tun"`
	}
	if err := json.Unmarshal(raw, &st); err != nil || !st.KillSwitch || !st.Tun {
		t.Fatalf("bad marker %q: %v", raw, err)
	}
	d.clearApplied(true, false)
	if _, err := os.Stat(d.appliedPath()); err != nil {
		t.Fatalf("marker must survive while tun still applied: %v", err)
	}
	d.clearApplied(false, true)
	if _, err := os.Stat(d.appliedPath()); !os.IsNotExist(err) {
		t.Fatalf("marker must be removed when nothing is applied: %v", err)
	}
}

// B3: a crash marker is reconciled (cleared) when the helper succeeds;
// no marker = no helper runs.
func TestReconcileApplied(t *testing.T) {
	d := testDaemon(t)
	// Point the helper at a path that fails TrustedBinary immediately:
	// cleanup must fail fast, never resolve a real elevated helper in
	// tests.
	d.settings.HelperPath = filepath.Join(t.TempDir(), "no-such-helper")
	// No marker: nothing happens, no error.
	d.reconcileApplied()
	// Marker for both, but no helper on PATH: cleanup fails, marker stays.
	d.setApplied(true, true)
	d.reconcileApplied() // helperPath fails -> flags kept
	if _, err := os.Stat(d.appliedPath()); err != nil {
		t.Fatalf("marker must survive a failed cleanup: %v", err)
	}
	d.mu.Lock()
	kill, tun := d.appliedKill, d.appliedTun
	d.mu.Unlock()
	if !kill || !tun {
		t.Fatalf("flags must be adopted from the marker: kill=%v tun=%v", kill, tun)
	}
}
