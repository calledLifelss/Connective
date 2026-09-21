package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestTunCleanupRefusesForeign(t *testing.T) {
	for _, foreign := range []string{"tun0", "wg0", "tailscale0", "eth0"} {
		err := tunCleanup([]string{"--if", foreign})
		if err == nil {
			t.Fatalf("tun-cleanup --if %s must be refused", foreign)
		}
		if !strings.Contains(err.Error(), "foreign") {
			t.Fatalf("error must say foreign, got %q", err)
		}
	}
}

func TestTunCleanupRequiresFlag(t *testing.T) {
	if err := tunCleanup(nil); err == nil {
		t.Fatalf("missing --if must fail")
	}
}

func TestStopCoreRequiresPid(t *testing.T) {
	if err := stopCore(nil); err == nil {
		t.Fatalf("missing --pid must fail")
	}
}

func TestStopCoreRefusesBadPids(t *testing.T) {
	for _, bad := range []string{"abc", "0", "1", "-5", "12a", "1.5", ""} {
		if err := stopCore([]string{"--pid", bad}); err == nil {
			t.Fatalf("stop-core --pid %q must be refused", bad)
		}
	}
}

func TestStopCoreRefusesSelf(t *testing.T) {
	if err := stopCore([]string{"--pid", strconv.Itoa(os.Getpid())}); err == nil {
		t.Fatalf("stop-core must refuse the helper's own pid")
	}
}

func TestStopCoreGonePidSucceeds(t *testing.T) {
	// A reaped short-lived child is guaranteed ESRCH (no reuse that fast).
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skip("no true binary")
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	if err := stopCore([]string{"--pid", strconv.Itoa(pid)}); err != nil {
		t.Fatalf("gone pid must succeed, got %v", err)
	}
}

func TestStopCoreRefusesLiveForeignProcess(t *testing.T) {
	// PID-reuse guard: a live, signalable, non-core process ("sleep")
	// must be refused AND left alive.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skip("no sleep binary")
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	pid := cmd.Process.Pid
	err := stopCore([]string{"--pid", strconv.Itoa(pid)})
	if err == nil {
		t.Fatalf("stop-core must refuse a live non-core pid")
	}
	if !strings.Contains(err.Error(), "does not look like") {
		t.Fatalf("refusal must name the guard, got %q", err)
	}
	if e := syscall.Kill(pid, 0); e != nil {
		t.Fatalf("foreign process must be left alive, kill-0: %v", e)
	}
}
