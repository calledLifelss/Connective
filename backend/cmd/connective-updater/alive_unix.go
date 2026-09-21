//go:build unix

package main

import (
	"os"
	"syscall"
)

// processAlive probes liveness with signal 0 (no effect on the target).
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
