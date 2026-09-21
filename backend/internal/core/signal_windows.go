package core

import (
	"os"
	"syscall"
)

// terminatedSignal is the graceful-shutdown signal. Windows has no
// POSIX signals: Process.Signal(SIGTERM) is unsupported, so the
// manager falls back to Kill after ShutdownTimeout (see Stop). The
// disconnect path still runs full cleanup (helper stop-core,
// route/DNS/firewall restore), so termination is always supervised.
// os.Kill here documents that Windows shutdown is immediate.
func terminatedSignal() os.Signal { return os.Kill }

// deathSignal has no Windows equivalent in stdlib (job objects would
// need x/sys). Orphan protection instead: Stop/ensureCoreGone always
// run, the elevated core is tracked by PID, and helper stop-core
// finishes what the daemon cannot signal. Documented in docs/WINDOWS.md.
func deathSignal() *syscall.SysProcAttr { return nil }

// silence unused import if the build trims syscall elsewhere.
var _ = syscall.SIGTERM
