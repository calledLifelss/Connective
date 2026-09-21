package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isTrustedBinary reports whether path is safe to execute, and in
// particular safe to elevate via the UAC helper. Policy (Windows
// adaptation of the unix rules):
// absolute path, existing regular .exe, never under temp/download
// locations, parent chain contains no temp dir. Unix permission bits
// are meaningless on Windows (Go reports 0666 almost everywhere), so
// DACL granularity is intentionally left to a future x/sys-based check
// (documented in docs/WINDOWS.md); the location + identity policy
// below is what blocks settings.json-driven arbitrary execution today.
func isTrustedBinary(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("not absolute: %q", path)
	}
	lower := strings.ToLower(path)
	tmp := os.Getenv("TEMP")
	if tmp == "" {
		tmp = os.Getenv("TMP")
	}
	badDirs := []string{"\\temp\\", "\\tmp\\", "\\downloads\\", "$recycle.bin\\"}
	if tmp != "" {
		badDirs = append(badDirs, strings.ToLower(tmp))
	}
	for _, d := range badDirs {
		if strings.Contains(lower, strings.ToLower(d)) {
			return fmt.Errorf("untrusted location: %q", path)
		}
	}
	if !strings.HasSuffix(lower, ".exe") {
		return fmt.Errorf("not an executable: %q", path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %q", path)
	}
	return nil
}

// TrustedBinary validates path for execution (see isTrustedBinary).
func TrustedBinary(path string) error { return isTrustedBinary(path) }
