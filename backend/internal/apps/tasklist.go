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
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First CSV field is the quoted image name: "firefox.exe","1234",...
		if !strings.HasPrefix(line, "\"") {
			continue
		}
		end := strings.Index(line[1:], "\"")
		if end <= 0 {
			continue
		}
		if name := strings.TrimSpace(line[1 : 1+end]); name != "" {
			names = append(names, name)
		}
	}
	return names
}
