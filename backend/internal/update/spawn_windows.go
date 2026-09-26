package update

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// spawnUpdaterDetached launches the updater elevated (Program Files
// writes need admin) without blocking: PowerShell Start-Process with
// -Verb RunAs prompts once via UAC and the call returns immediately.
// The updater works after the app closes; its output goes to
// updates/updater.log via the updater itself.
//
// Two quoting/observation traps are avoided here:
//
//   - The argument list is pre-quoted with Windows (CommandLineToArgvW)
//     rules and passed as ONE string. Start-Process joins -ArgumentList
//     array elements with spaces WITHOUT re-quoting, which used to split
//     the default root "C:\Program Files\Connective" into two argv
//     entries; the updater then failed flag parsing before defining its
//     fail handler, wrote no result.json, and the update silently did
//     nothing while the UI said "restarting".
//   - PowerShell is watched asynchronously (Start + Wait in a goroutine)
//     instead of being waited on inline: a declined UAC prompt or an
//     updater that dies immediately reaches onExit as an error instead
//     of leaving the UI on "restarting" until the next boot.
func spawnUpdaterDetached(bin string, args []string, dataDir string, onExit func(error)) error {
	if err := os.MkdirAll(filepath.Join(dataDir, "updates"), 0o700); err != nil {
		return err
	}
	// argv[0] is the binary itself; -FilePath already carries it.
	quoted := make([]string, 0, len(args))
	for _, a := range args[1:] {
		quoted = append(quoted, winQuote(a))
	}
	ps := fmt.Sprintf(
		"$p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -PassThru;"+
			" $p.WaitForExit(); exit $p.ExitCode",
		psQuote(bin), psQuote(strings.Join(quoted, " ")))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-Command", ps)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("update: start installer: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil && out.Len() > 0 {
			msg := strings.TrimSpace(out.String())
			if len(msg) > 200 {
				msg = msg[:200]
			}
			err = fmt.Errorf("%w: %s", err, msg)
		}
		if onExit != nil {
			onExit(err)
		}
	}()
	return nil
}
