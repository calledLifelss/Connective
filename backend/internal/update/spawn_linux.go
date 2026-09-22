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
// lifetime: double-fork semantics via Setsid + Start (no Wait), so the
// updater survives the app closing to finish the install. Output goes
// to updates/updater.log, never the caller's pipes.
func spawnUpdaterDetached(bin string, args []string, dataDir string) error {
	argv := args
	name := bin
	if isProdRoot(rootOf(args)) && !platform.IsElevated() {
		runner := platform.HelperRunner()
		full := append([]string{}, runner[1:]...)
		full = append(full, bin)
		full = append(full, args[1:]...)
		name, argv = runner[0], full
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
	cmd := exec.Command(name, argv[1:]...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("update: start installer: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
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
