// Package apps lists currently-running desktop applications for the
// per-app split-tunnel picker. Stdlib only, read-only: it never starts,
// stops, or signals processes.
//
// Identity is the executable basename, normalized by NormalizeAppID
// (lowercase, no path, no ".exe"), shared with settings validation so
// the UI, the daemon, and the generated sing-box process_name rules all
// agree on what "firefox" means.
package apps

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// App is one distinct running program.
type App struct {
	ID    string `json:"id"`    // normalized exe basename ("firefox")
	Name  string `json:"name"`  // display name (original basename)
	Count int    `json:"count"` // live process instances
}

// MaxApps bounds IPC payloads and picker lists.
const MaxApps = 500

// NormalizeAppID maps user input or an observed process name to its
// canonical id: basename, lowercase, ".exe" stripped. Empty means
// "not a usable app entry" (blank, path-only, overlong, bad charset).
func NormalizeAppID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Tolerate full paths and Windows backslashes from any reporter.
	s = strings.ReplaceAll(s, "\\", "/")
	s = filepath.Base(s)
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == "/" {
		return ""
	}
	lower := strings.ToLower(s)
	lower = strings.TrimSuffix(lower, ".exe")
	if lower == "" {
		return ""
	}
	if len(lower) > 128 {
		return ""
	}
	for _, c := range lower {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '+') {
			return ""
		}
	}
	return lower
}

// NormalizeList dedupes ids (order-preserving), drops unusable entries,
// and caps the length so a hostile settings document cannot bloat the
// generated core config.
func NormalizeList(in []string, max int) []string {
	if max <= 0 {
		max = 200
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		id := NormalizeAppID(s)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if len(out) >= max {
			break
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

// List returns distinct running programs, most instances first (ties by
// name). It never fails loudly: an unreadable table yields an empty
// list, never an error that could block settings or connect flows.
func List() []App {
	var raw []string
	switch runtime.GOOS {
	case "windows":
		raw = windowsProcesses()
	default:
		raw = linuxProcesses()
	}
	counts := map[string]int{}
	display := map[string]string{}
	for _, name := range raw {
		id := NormalizeAppID(name)
		if id == "" {
			continue
		}
		counts[id]++
		if _, ok := display[id]; !ok {
			display[id] = strings.TrimSuffix(name, ".exe")
		}
	}
	apps := make([]App, 0, len(counts))
	for id, n := range counts {
		apps = append(apps, App{ID: id, Name: display[id], Count: n})
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Count != apps[j].Count {
			return apps[i].Count > apps[j].Count
		}
		return apps[i].Name < apps[j].Name
	})
	if len(apps) > MaxApps {
		apps = apps[:MaxApps]
	}
	if apps == nil {
		return []App{}
	}
	return apps
}

// procRoot is overridable for tests (fake /proc trees).
var procRoot = "/proc"

func linuxProcesses() []string {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		isPID := true
		for _, c := range name {
			if c < '0' || c > '9' {
				isPID = false
				break
			}
		}
		if !isPID {
			continue
		}
		// comm is the kernel short name, no args, one line.
		if raw, err := os.ReadFile(filepath.Join(procRoot, name, "comm")); err == nil {
			if s := strings.TrimSpace(string(raw)); s != "" {
				out = append(out, s)
				continue
			}
		}
		// Fallback: first argv[0] basename from cmdline (NUL-separated).
		if raw, err := os.ReadFile(filepath.Join(procRoot, name, "cmdline")); err == nil && len(raw) > 0 {
			arg0 := string(raw)
			if i := strings.IndexByte(arg0, 0); i >= 0 {
				arg0 = arg0[:i]
			}
			if base := filepath.Base(strings.TrimSpace(arg0)); base != "" && base != "." && base != "/" {
				out = append(out, base)
			}
		}
	}
	return out
}
