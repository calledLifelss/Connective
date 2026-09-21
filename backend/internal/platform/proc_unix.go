//go:build !windows

package platform

import (
	"errors"
	"syscall"
)

// errUnreachable marks a PID that is alive but beyond our signals
// (root-owned elevated core: EPERM). Only the privileged helper can
// end it.
var errUnreachable = errors.New("process alive but not signalable")

// ErrUnreachable reports whether ProcessGone found a live process.
func ErrUnreachable(err error) bool { return err == errUnreachable }

// ProcessGone probes pid with signal 0: nil when reaped.
func ProcessGone(pid int) error {
	err := syscall.Kill(pid, 0)
	if err == nil || err == syscall.EPERM {
		return errUnreachable
	}
	if err == syscall.ESRCH {
		return nil
	}
	return err
}
