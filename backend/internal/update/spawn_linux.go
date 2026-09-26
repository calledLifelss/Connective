//go:build !windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"connective/backend/internal/platform"
)

// spawnUpdaterDetached launches the updater outside the daemon's
// lifetime: double-fork semantics via Setsid + Start (no Wait by
// default), so the updater survives the app closing to finish the
// install. Output goes to updates/updater.log, never the caller's
// pipes. When onExit is provided the child is reaped in a goroutine so
// a declined pkexec/polkit prompt (exit status non-zero, immediately)
// becomes a visible failure instead of a permanent "restarting".
func spawnUpdaterDetached(bin string, args []string, dataDir string, onExit func(error)) error {
	prog, progArgs := updaterCommand(bin, args)
	if isProdRoot(rootOf(args)) && !platform.IsElevated() {
		runner := platform.HelperRunner()
		if len(runner) == 0 {
			return fmt.Errorf("update: start installer: empty helper runner")
		}
		prog, progArgs = elevateCommand(bin, args, runner)
	}
	logPath := filepath.Join(dataDir, "updates", "updater.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(prog, progArgs...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("update: start installer: %w", err)
	}
	if onExit != nil {
		go func() {
			onExit(cmd.Wait())
		}()
		return nil
	}
	_ = cmd.Process.Release()
	return nil
}

// updaterCommand builds the direct invocation: run bin with its argv
// (argv[0] is the binary itself, so the process args are argv[1:]).
func updaterCommand(bin string, args []string) (string, []string) {
	return bin, args[1:]
}

// elevateCommand reshapes [bin, ...args] through a helper runner:
// [runner..., bin, ...args]. Pure for tests: an earlier version of the
// pkexec path dropped bin and asked pkexec to run "apply", which died
// with "No such file or directory" and wedged the UI on restarting.
func elevateCommand(bin string, args []string, runner []string) (string, []string) {
	progArgs := append([]string{}, runner[1:]...)
	progArgs = append(progArgs, bin)
	progArgs = append(progArgs, args[1:]...)
	return runner[0], progArgs
}

// rootOf extracts --install-root from a spawn argv (argv[0] is the binary).
func rootOf(argv []string) string {
	for i := 1; i+1 < len(argv); i++ {
		if argv[i] == "--install-root" {
			return argv[i+1]
		}
	}
	return ""
}
