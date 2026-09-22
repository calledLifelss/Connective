//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// anyNamedAlive scans /proc (stdlib only) for a live process whose
// comm or argv[0] basename equals one of names, excluding self.
func anyNamedAlive(names []string) bool {
	self := os.Getpid()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return true // fail safe: assume alive, caller retries
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pidNum, err := strconv.Atoi(e.Name())
		if err != nil || pidNum == self {
			continue
		}
		if commMatches(e.Name(), want) || cmdlineMatches(e.Name(), want) {
			return true
		}
	}
	return false
}

func commMatches(pid string, want map[string]bool) bool {
	raw, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
	if err != nil {
		return false
	}
	return want[strings.TrimSpace(string(raw))]
}

func cmdlineMatches(pid string, want map[string]bool) bool {
	raw, err := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	return want[filepath.Base(parts[0])]
}

// waitForExit polls until no wanted process is alive or timeout hits.
func waitForExit(names []string, timeout time.Duration) error {
	return waitForNames(anyNamedAlive, names, timeout)
}

// waitForNames is the injectable core (tests script aliveness).
func waitForNames(alive func([]string) bool, names []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !alive(names) {
			return nil
		}
		if time.Now().After(deadline) {
			return errStillRunning(names)
		}
		time.Sleep(2 * time.Second)
	}
}

func errStillRunning(names []string) error {
	return fmt.Errorf("still running after wait: %s", strings.Join(names, ", "))
}
