package platform

import (
	"context"
	"os"
	"strings"
)

// IsElevated reports whether the process runs with administrative
// rights. `fltmc` (filter-manager control) succeeds only elevated and
// needs no arguments; anything else — including its absence — reads as
// non-elevated (fail safe: we prompt rather than assume).
func IsElevated() bool {
	out, err := OsRunner{}.Run(context.Background(), "fltmc")
	if err != nil {
		return false
	}
	return !strings.Contains(out, "Access is denied")
}

// HelperRunner returns the elevation prefix. There is no pkexec/sudo
// on Windows: elevation goes through a UAC prompt via PowerShell's
// Start-Process -Verb RunAs -Wait, which shows the standard consent UI
// for the exact helper binary and arguments (never a shell string).
// Tests override via CONNECTIVE_HELPER_RUNNER (split on spaces).
func HelperRunner() []string {
	if v := os.Getenv("CONNECTIVE_HELPER_RUNNER"); v != "" {
		return splitFields(v)
	}
	// Placeholder elements replaced per-invocation by ElevateCommand.
	return []string{"powershell", "-NoProfile", "-Command"}
}

// ElevateCommand builds the PowerShell elevation invocation for an
// already-validated helper path and argument list. Single-quoted
// arguments prevent injection; the helper itself re-validates.
func ElevateCommand(helper string, args []string) (string, []string) {
	var quoted []string
	quoted = append(quoted, "'"+strings.ReplaceAll(helper, "'", "''")+"'")
	for _, a := range args {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "''")+"'")
	}
	ps := "Start-Process " + strings.Join(quoted, " ") + " -Verb RunAs -Wait"
	return "powershell", []string{"-NoProfile", "-Command", ps}
}

// HelperExeName is the helper binary filename.
func HelperExeName() string { return "connective-helper.exe" }

// CoreExeName is the core binary filename.
func CoreExeName() string { return "sing-box.exe" }
