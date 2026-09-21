package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isTrustedBinary reports whether path is safe to execute, and in
// particular safe to elevate via the privileged helper. Policy:
// absolute path, existing regular file, not world-writable, parent not
// world-writable, never under temp dirs. A malicious or sloppy
// settings.json must not turn the elevation prompt into arbitrary
// root execution.
func isTrustedBinary(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("not absolute: %q", path)
	}
	for _, tmp := range []string{"/tmp/", "/var/tmp/", "/dev/shm/"} {
		if strings.HasPrefix(path, tmp) || path == strings.TrimSuffix(tmp, "/") {
			return fmt.Errorf("temp dirs are not executable locations: %q", path)
		}
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %q", path)
	}
	if fi.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("world-writable binary refused: %q", path)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("cannot resolve dir of %q: %w", path, err)
	}
	di, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("cannot stat dir of %q: %w", path, err)
	}
	if di.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("world-writable directory refused: %q", parent)
	}
	return nil
}

// TrustedBinary validates path for execution (see isTrustedBinary).
func TrustedBinary(path string) error { return isTrustedBinary(path) }
