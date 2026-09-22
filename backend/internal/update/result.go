package update

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fmt"
)

// WriteResult records the updater's report for the next app boot to
// consume (see reconcileBoot). Empty path = no reporting (manual runs).
func WriteResult(path, version string, ok bool, errMsg, prev string) error {
	if path == "" {
		return nil
	}
	res := ResultUpdate{Version: version, OK: ok, Error: errMsg, Prev: prev, AtUnix: time.Now().Unix()}
	return writeJSON(path, res)
}

// WriteCurrentTxt flips the Windows version pointer (the installed
// layout addresses trees through current.txt until the launcher stub
// lands). Atomic replace in the same dir; the version is validated.
func WriteCurrentTxt(root, version string) error {
	if _, err := ParseVersion(version); err != nil {
		return fmt.Errorf("update: bad version %q", version)
	}
	if strings.ContainsAny(version, `/\`) {
		return fmt.Errorf("update: unsafe version %q", version)
	}
	tmp := filepath.Join(root, ".current.txt.tmp")
	if err := os.WriteFile(tmp, []byte(version), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, "current.txt"))
}

// ReadCurrentTxt returns the Windows pointer ("" when unset).
func ReadCurrentTxt(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "current.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
