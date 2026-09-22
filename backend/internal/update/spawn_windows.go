package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// spawnUpdaterDetached launches the updater elevated (Program Files
// writes need admin) without blocking: PowerShell Start-Process with
// -Verb RunAs prompts once via UAC and returns immediately. The
// updater works after the app closes; output goes to
// updates/updater.log via the updater itself.
func spawnUpdaterDetached(bin string, args []string, dataDir string) error {
	if err := os.MkdirAll(filepath.Join(dataDir, "updates"), 0o700); err != nil {
		return err
	}
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, psQuote(a))
	}
	ps := fmt.Sprintf(
		"Start-Process -FilePath %s -ArgumentList %s -Verb RunAs",
		psQuote(bin), strings.Join(quoted[1:], ","))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("update: start installer: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// psQuote single-quotes one PowerShell argument.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
