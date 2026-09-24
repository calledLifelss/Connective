package apps

import (
	"os/exec"
	"runtime"
	"strings"
)

// windowsProcesses lists image names via the inbox tasklist utility
// (CSV, no header). No PS parsing, no cgo, read-only.
func windowsProcesses() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if name := parseTasklistLine(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// parseTasklistLine extracts the image name from one `tasklist /FO CSV
// /NH` row: the first quoted field ("firefox.exe","1234",...). Pure
// for unit tests (no process execution).
func parseTasklistLine(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "\"") {
		return ""
	}
	end := strings.Index(line[1:], "\"")
	if end <= 0 {
		return ""
	}
	return strings.TrimSpace(line[1 : 1+end])
}
