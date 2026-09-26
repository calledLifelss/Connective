package update

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Production Install with a hooked spawn must write pending.json,
// invoke the updater exactly once, and land on restarting (never hang
// on installing).
func TestInstallHandoffSpawnsUpdater(t *testing.T) {
	dataDir := t.TempDir()
	m := &Manager{
		DataDir:   dataDir,
		DaemonExe: "",
		CurrentVersion: func() string {
			return "0.3.1"
		},
		Channel: func() string { return ChannelStable },
	}
	var gotBin string
	var gotArgs []string
	m.Spawn = func(bin string, args []string, dd string, onExit func(error)) error {
		gotBin, gotArgs = bin, args
		if dd != dataDir {
			t.Fatalf("spawn datadir = %q", dd)
		}
		return nil
	}
	// Fake an updater binary next to a fake daemon exe.
	binDir := t.TempDir()
	fakeDaemon := filepath.Join(binDir, "connectived")
	if err := os.WriteFile(fakeDaemon, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeUpdater := filepath.Join(binDir, updaterExeName())
	if err := os.WriteFile(fakeUpdater, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.DaemonExe = fakeDaemon

	if err := m.handoff(t.TempDir(), "0.3.1"); err != nil {
		t.Fatal(err)
	}
	if gotBin != fakeUpdater {
		t.Fatalf("spawn bin = %q, want %q", gotBin, fakeUpdater)
	}
	joined := ""
	for _, a := range gotArgs {
		joined += a + " "
	}
	for _, want := range []string{"apply", "--install-root", "--version", "0.3.1", "--result-file"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("spawn argv missing %q: %q", want, joined)
		}
	}
	var pend PendingUpdate
	if err := readJSON(pendingPath(dataDir), &pend); err != nil {
		t.Fatalf("pending not written: %v", err)
	}
	if pend.Version != "0.3.1" || pend.Updater != fakeUpdater {
		t.Fatalf("pending = %+v", pend)
	}
}

// Missing updater binary must fail fast with an honest error (never a
// wedged installing state).
func TestHandoffNoUpdaterFails(t *testing.T) {
	m := &Manager{DataDir: t.TempDir(), DaemonExe: filepath.Join(t.TempDir(), "connectived")}
	if err := m.handoff(t.TempDir(), "0.3.1"); err == nil {
		t.Fatal("expected error with no updater binary")
	}
}

// A result file with an unparsable version is consumed but announces
// nothing (the result dir is user-writable: never trust it blindly).
func TestReconcileBootBadVersion(t *testing.T) {
	dataDir := t.TempDir()
	if err := WriteResult(resultPath(dataDir), "not-a-version!!", true, "", ""); err != nil {
		t.Fatal(err)
	}
	m := &Manager{DataDir: dataDir, CurrentVersion: func() string { return "0.3.0" }}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	if m.Status().State != StateIdle {
		t.Fatalf("state = %q, want idle", m.Status().State)
	}
	if _, err := os.Stat(resultPath(dataDir)); !os.IsNotExist(err) {
		t.Fatal("bad result not consumed")
	}
}

// Full production-shaped flow (no test apply): install hands a staged
// tree to a hooked spawn and lands on restarting; cancelling from
// there withdraws the pending contract and frees the UI instead of
// wedging on restarting forever.
func TestCancelFromRestarting(t *testing.T) {
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
	var joined string
	m.Spawn = func(bin string, args []string, dd string, onExit func(error)) error {
		for _, a := range args {
			joined += a + " "
		}
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
	if !strings.Contains(joined, "--staged-dir") {
		t.Fatalf("spawn argv missing staged dir: %q", joined)
	}
	var pend PendingUpdate
	if err := readJSON(pendingPath(m.DataDir), &pend); err != nil {
		t.Fatalf("pending not written: %v", err)
	}
	if pend.Staged == "" {
		t.Fatalf("pending missing staged tree: %+v", pend)
	}
	if got := m.Cancel(); got.State != StateCancelled {
		t.Fatalf("cancel: %s", got.State)
	}
	if _, err := os.Stat(pendingPath(m.DataDir)); !os.IsNotExist(err) {
		t.Fatal("cancel must withdraw the pending contract")
	}
}

// A consumed ok-result seeds updated; failed seeds failed; both are
// one-shot (files removed).
func TestReconcileBootResult(t *testing.T) {
	dataDir := t.TempDir()
	if err := WriteResult(resultPath(dataDir), "0.3.1", true, "", "0.3.0"); err != nil {
		t.Fatal(err)
	}
	m := &Manager{DataDir: dataDir, CurrentVersion: func() string { return "0.3.1" }}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	if m.Status().State != StateUpdated {
		t.Fatalf("state = %q, want updated", m.Status().State)
	}
	if m.Status().Info == nil || m.Status().Info.Version != "0.3.1" {
		t.Fatalf("info = %+v", m.Status().Info)
	}
	if _, err := os.Stat(resultPath(dataDir)); !os.IsNotExist(err) {
		t.Fatal("result not consumed")
	}

	if err := WriteResult(resultPath(dataDir), "0.3.1", false, "boom", ""); err != nil {
		t.Fatal(err)
	}
	m2 := &Manager{DataDir: dataDir, CurrentVersion: func() string { return "0.3.0" }}
	if err := m2.Init(); err != nil {
		t.Fatal(err)
	}
	st := m2.Status()
	if st.State != StateFailed || st.Error != "boom" {
		t.Fatalf("state = %q err = %q", st.State, st.Error)
	}
}

// Stale pending without a result clears silently (updater never ran:
// auth declined or killed) so the next check retries cleanly.
func TestReconcileBootStalePending(t *testing.T) {
	dataDir := t.TempDir()
	pend := PendingUpdate{Version: "0.3.1", Root: "/tmp/x", Updater: "/tmp/y"}
	if err := writeJSON(pendingPath(dataDir), pend); err != nil {
		t.Fatal(err)
	}
	m := &Manager{DataDir: dataDir, CurrentVersion: func() string { return "0.3.0" }}
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	if m.Status().State != StateIdle {
		t.Fatalf("state = %q, want idle", m.Status().State)
	}
	if _, err := os.Stat(pendingPath(dataDir)); !os.IsNotExist(err) {
		t.Fatal("stale pending not cleared")
	}
}

// Assemble twice must succeed (idempotent retry after restart).
func TestAssembleIdempotent(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	zipDir := t.TempDir()
	zipPath, _ := writeZip(t, zipDir, map[string]string{"connective": "x"})
	ctx := context.Background()
	dir1, err := in.Assemble(ctx, "1.2.3", zipPath, ArtifactFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir2, err := in.Assemble(ctx, "1.2.3", zipPath, ArtifactFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dir1 != dir2 {
		t.Fatalf("%q != %q", dir1, dir2)
	}
}

// Windows pointer round-trips; bad versions rejected.
func TestCurrentTxt(t *testing.T) {
	root := t.TempDir()
	if err := WriteCurrentTxt(root, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if got := ReadCurrentTxt(root); got != "1.2.3" {
		t.Fatalf("current = %q", got)
	}
	if err := WriteCurrentTxt(root, "../evil"); err == nil {
		t.Fatal("expected unsafe version rejection")
	}
	if ReadCurrentTxt(t.TempDir()) != "" {
		t.Fatal("expected empty for missing pointer")
	}
}
