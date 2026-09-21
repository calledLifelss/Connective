// Package platform abstracts OS differences (Linux first, Windows
// later) and the privilege boundary: the UI and daemon run unprivileged;
// TUN/routing/firewall operations execute in a small privileged helper
// authorized per-action via polkit (Linux) so the whole app never runs
// as root. The helper protocol is implemented in cmd/connective-helper;
// this package fixes the
// abstraction and data directories now.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// OS returns the runtime operating system.
func OS() string { return runtime.GOOS }

// IsLinux reports whether this is the primary target platform.
func IsLinux() bool { return runtime.GOOS == "linux" }

// DataDir returns the per-user state directory, creating it:
// ~/.local/share/connective on Linux, %LOCALAPPDATA%/Connective on
// Windows (roaming profiles must not carry sockets/caches).
// CONNECTIVE_DATA_DIR overrides both (tests and portable installs).
func DataDir() (string, error) {
	if override := os.Getenv("CONNECTIVE_DATA_DIR"); override != "" {
		if err := os.MkdirAll(override, 0o700); err != nil {
			return "", err
		}
		return override, nil
	}
	var dir string
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = home
		}
		dir = filepath.Join(base, "Connective")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share", "connective")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// IsWindows reports whether this is the Windows target.
func IsWindows() bool { return runtime.GOOS == "windows" }

// SocketPath returns the daemon IPC socket path.
func SocketPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "connectived.sock"), nil
}
