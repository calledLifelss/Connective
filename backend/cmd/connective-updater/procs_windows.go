package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// anyNamedAlive asks tasklist per image name (stdlib + OS tools only).
// Unparseable output fails safe (assumed alive, caller retries).
func anyNamedAlive(names []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, n := range names {
		out, err := exec.CommandContext(ctx, "tasklist",
			"/FI", "IMAGENAME eq "+n, "/FO", "CSV", "/NH").CombinedOutput()
		if err != nil {
			return true
		}
		s := string(out)
		if !strings.Contains(s, "INFO: No tasks") && strings.Contains(s, n) {
			return true
		}
	}
	return false
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
