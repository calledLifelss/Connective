//go:build !windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

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
