//go:build !windows

package platform

import "os"

// IsElevated reports whether the process runs privileged (root).
func IsElevated() bool { return os.Geteuid() == 0 }

// HelperRunner returns the elevation prefix. Default is pkexec
// (interactive polkit prompt). Tests override via
// CONNECTIVE_HELPER_RUNNER, e.g. "sudo -n", after caching credentials.
func HelperRunner() []string {
	if v := os.Getenv("CONNECTIVE_HELPER_RUNNER"); v != "" {
		return splitFields(v)
	}
	return []string{"pkexec"}
}

// HelperExeName is the helper binary filename.
func HelperExeName() string { return "connective-helper" }

// CoreExeName is the core binary filename.
func CoreExeName() string { return "sing-box" }
