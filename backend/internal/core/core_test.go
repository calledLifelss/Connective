package core

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartStopSleep(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	m := New(Config{
		Binary:          "sleep",
		Args:            []string{"30"},
		StartupTimeout:  3 * time.Second,
		ShutdownTimeout: 2 * time.Second,
		Ready:           func() bool { return true },
		OnLog:           func(l string) { mu.Lock(); lines = append(lines, l); mu.Unlock() },
	})
	if err := m.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !m.Running() {
		t.Fatalf("should be running")
	}
	// Duplicate start must be a safe no-op.
	if err := m.Start(); err != nil {
		t.Fatalf("duplicate start: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if m.Running() || m.GetStatus() != StatusStopped {
		t.Fatalf("should be stopped")
	}
	// No orphan: second stop is fine.
	if err := m.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestMissingBinary(t *testing.T) {
	m := New(Config{Binary: "connective-definitely-missing-binary", Ready: func() bool { return true }})
	if err := m.Start(); err == nil {
		m.Stop()
		t.Fatalf("expected error for missing binary")
	}
}

// Pid tracks the supervised child: nonzero while running (usable by the
// daemon's elevated-core fallback), zero before start and after stop.
func TestPidLifecycle(t *testing.T) {
	m := New(Config{
		Binary:          "sleep",
		Args:            []string{"30"},
		StartupTimeout:  3 * time.Second,
		ShutdownTimeout: 2 * time.Second,
		Ready:           func() bool { return true },
	})
	if m.Pid() != 0 {
		t.Fatalf("pid before start must be 0, got %d", m.Pid())
	}
	if err := m.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if m.Pid() <= 0 {
		t.Fatalf("pid while running must be positive, got %d", m.Pid())
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if m.Pid() != 0 {
		t.Fatalf("pid after stop must be 0, got %d", m.Pid())
	}
}

// Stop must return promptly even for a child that ignores SIGTERM: the
// SIGKILL fallback plus a bounded reap wait. (The EPERM case — a
// root-owned elevated core — cannot be simulated unprivileged; it was
// verified live. This test pins the bound for the ignorant-child case.)
func TestStopBoundedOnIgnorantChild(t *testing.T) {
	m := New(Config{
		Binary:          "sh",
		Args:            []string{"-c", "trap '' TERM; sleep 30"},
		StartupTimeout:  3 * time.Second,
		ShutdownTimeout: 1 * time.Second,
		Ready:           func() bool { return true },
	})
	if err := m.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- m.Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("Stop hung past its grace + reap bound")
	}
}

func TestEarlyExit(t *testing.T) {
	m := New(Config{
		Binary:         "sh",
		Args:           []string{"-c", "exit 3"},
		StartupTimeout: 3 * time.Second,
		Ready:          func() bool { return false },
	})
	if err := m.Start(); err == nil {
		m.Stop()
		t.Fatalf("expected error for early exit")
	}
	if m.GetStatus() == StatusRunning {
		t.Fatalf("must not report running")
	}
}

func TestReadinessTimeout(t *testing.T) {
	m := New(Config{
		Binary:         "sleep",
		Args:           []string{"30"},
		StartupTimeout: 300 * time.Millisecond,
		Ready:          func() bool { return false },
	})
	defer m.Stop()
	if err := m.Start(); err == nil || !strings.Contains(err.Error(), "ready") {
		t.Fatalf("expected readiness timeout, got %v", err)
	}
}

func TestLogCapture(t *testing.T) {
	done := make(chan string, 10)
	m := New(Config{
		Binary: "sh",
		Args:   []string{"-c", "echo hello-from-core; sleep 30"},
		Ready:  func() bool { return true },
		OnLog:  func(l string) { done <- l },
	})
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	select {
	case line := <-done:
		if line != "hello-from-core" {
			t.Fatalf("bad log line %q", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no log captured")
	}
}
