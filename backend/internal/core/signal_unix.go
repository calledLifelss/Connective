//go:build !windows

package core

import (
	"os"
	"syscall"
)

// terminatedSignal is the graceful-shutdown signal (SIGTERM on unix).
func terminatedSignal() os.Signal { return syscall.SIGTERM }

// deathSignal returns a SysProcAttr that kills the child if Connective's
// supervisor dies for any reason (including SIGKILL, which no userspace
// cleanup can intercept). This guarantees no orphaned core processes and
// no lingering tunnels after a crash — a privacy requirement, not just
// hygiene: traffic must never flow outside the supervised lifecycle.
func deathSignal() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
