//go:build windows

package core

import (
	"os"
	"syscall"
)

// terminatedSignal: Windows has no SIGTERM; Kill is used directly.
func terminatedSignal() os.Signal { return os.Kill }

// deathSignal: no parent-death signal on Windows; orphan reaping relies
// on the Win32 job object path (see CoreAdminManager equivalent).
func deathSignal() *syscall.SysProcAttr { return nil }
