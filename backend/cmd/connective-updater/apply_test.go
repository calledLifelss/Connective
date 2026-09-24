package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connective/backend/internal/update"
)

// applyTree builds a fake assembled tree for version v.
func applyTree(t *testing.T, root, v string) {
	t.Helper()
	dir := filepath.Join(root, "versions", v)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"connective", "connectived", "sing-box"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if isWindows() {
		for _, n := range []string{"connective.exe", "connectived.exe", "sing-box.exe", "wintun.dll"} {
			if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Full apply: no app running → activate + result ok + pointer flipped.
func TestApplyActivatesAndReports(t *testing.T) {
	oldWait := waitExit
	waitExit = func([]string, time.Duration) error { return nil }
	defer func() { waitExit = oldWait }()
	root := t.TempDir()
	applyTree(t, root, "9.9.9")
	res := filepath.Join(t.TempDir(), "result.json")
	err := run([]string{"apply", "--install-root", root, "--version", "9.9.9",
		"--result-file", res, "--wait-timeout", "5s"})
	if err != nil {
		t.Fatal(err)
	}
	in := &update.Installer{Root: root}
	if got := in.Current(); got != "9.9.9" {
		t.Fatalf("current = %q", got)
	}
	raw, err := os.ReadFile(res)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Version string `json:"version"`
		OK      bool   `json:"ok"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if !r.OK || r.Version != "9.9.9" {
		t.Fatalf("result = %s", string(raw))
	}
}

// Staged promotion: the updater copies the daemon-staged tree into
// the install root before activation (the daemon cannot write there).
func TestApplyPromotesStaged(t *testing.T) {
	oldWait := waitExit
	waitExit = func([]string, time.Duration) error { return nil }
	defer func() { waitExit = oldWait }()
	staged := filepath.Join(t.TempDir(), "versions", "9.9.9")
	applyTree(t, filepath.Dir(filepath.Dir(staged)), "9.9.9")
	root := t.TempDir()
	res := filepath.Join(t.TempDir(), "result.json")
	err := run([]string{"apply", "--install-root", root, "--version", "9.9.9",
		"--staged-dir", staged, "--result-file", res, "--wait-timeout", "5s"})
	if err != nil {
		t.Fatal(err)
	}
	in := &update.Installer{Root: root}
	if got := in.Current(); got != "9.9.9" {
		t.Fatalf("current = %q", got)
	}
	raw, err := os.ReadFile(res)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Version string `json:"version"`
		OK      bool   `json:"ok"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if !r.OK || r.Version != "9.9.9" {
		t.Fatalf("result = %s", string(raw))
	}
}

// Missing tree must fail with a failed result (never a silent hang).
func TestApplyMissingTreeFails(t *testing.T) {
	oldWait := waitExit
	waitExit = func([]string, time.Duration) error { return nil }
	defer func() { waitExit = oldWait }()
	root := t.TempDir()
	res := filepath.Join(t.TempDir(), "result.json")
	err := run([]string{"apply", "--install-root", root, "--version", "9.9.9",
		"--result-file", res, "--wait-timeout", "1s"})
	if err == nil {
		t.Fatal("expected error for missing tree")
	}
	raw, err := os.ReadFile(res)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.OK || r.Error == "" {
		t.Fatalf("result = %s", string(raw))
	}
}

// waitForNames honors scripted aliveness and times out honestly.
func TestWaitForNames(t *testing.T) {
	calls := 0
	err := waitForNames(func([]string) bool {
		calls++
		return calls < 2
	}, []string{"x"}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
	err = waitForNames(func([]string) bool { return true }, []string{"x"}, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout")
	}
}
