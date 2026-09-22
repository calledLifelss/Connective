package update

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Production update handoff: the daemon assembles the new tree, then a
// detached connective-updater owns activation (the daemon never
// self-replaces and never blocks a state forever).
//
//	<datadir>/updates/pending.json   written before spawn, consumed at boot
//	<datadir>/updates/result.json    written by the updater, consumed at boot
//
// pending without a result means the updater never ran (auth declined,
// killed): boot clears it so the user can retry instead of wedging.
// A result seeds one boot announcement (updated/failed) and is consumed.
const (
	pendingFile = "pending.json"
	resultFile  = "result.json"
	// updaterWait bounds how long the updater waits for the app to
	// close before failing honestly (the UI tells the user to close).
	updaterWaitTimeout = 15 * time.Minute
)

// PendingUpdate is the spawn contract: everything the updater needs,
// so it never reads user environment for locations.
type PendingUpdate struct {
	Version string `json:"version"`
	Root    string `json:"root"`
	Updater string `json:"updater"`
	AtUnix  int64  `json:"atUnix"`
}

// ResultUpdate is the updater's report, consumed once at daemon boot.
type ResultUpdate struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Prev    string `json:"prev,omitempty"`
	AtUnix  int64  `json:"atUnix"`
}

func pendingPath(dataDir string) string {
	return filepath.Join(dataDir, "updates", pendingFile)
}

func resultPath(dataDir string) string {
	return filepath.Join(dataDir, "updates", resultFile)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// updaterExeName is the updater binary filename per platform.
func updaterExeName() string {
	if runtime.GOOS == "windows" {
		return "connective-updater.exe"
	}
	return "connective-updater"
}

// isProdRoot reports whether root is a real install location (with a
// versions/ tree or the Windows current.txt pointer) rather than the
// dev/test versions-root scratch dir.
func isProdRoot(root string) bool {
	if st, err := os.Stat(filepath.Join(root, "versions")); err == nil && st.IsDir() {
		return true
	}
	if _, err := os.Stat(filepath.Join(root, "current.txt")); err == nil {
		return true
	}
	return false
}

// resolveInstallRoot finds the tree the updater must activate:
//   - Linux production: /opt/connective, but only when the running
//     daemon lives there (dev builds from a checkout must never
//     touch /opt — they fall back to versions-root scratch).
//   - Windows production: the install dir (versions/ + current.txt
//     beside it), found by walking up from the daemon binary.
//   - everywhere else: <datadir>/updates/versions-root (dev/test).
func resolveInstallRoot(daemonExe, dataDir string) string {
	if runtime.GOOS == "windows" {
		if dir := walkUpForWindowsRoot(daemonExe); dir != "" {
			return dir
		}
		return filepath.Join(dataDir, "updates", "versions-root")
	}
	const optRoot = "/opt/connective"
	if daemonExe != "" && strings.HasPrefix(daemonExe, optRoot+"/") {
		if isProdRoot(optRoot) {
			return optRoot
		}
	}
	return filepath.Join(dataDir, "updates", "versions-root")
}

// walkUpForWindowsRoot climbs from the daemon binary looking for the
// install dir (a dir containing versions/ with current.txt beside it).
func walkUpForWindowsRoot(daemonExe string) string {
	dir := filepath.Dir(daemonExe)
	for i := 0; i < 4 && dir != "" && dir != filepath.Dir(dir); i++ {
		if st, err := os.Stat(filepath.Join(dir, "versions")); err == nil && st.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "current.txt")); err == nil {
				return dir
			}
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// resolveUpdaterBin locates connective-updater: on Windows production
// first under <root>/updater (the installer layout), then next to the
// daemon, then PATH. Empty means "cannot update here".
func resolveUpdaterBin(daemonExe, root string) string {
	name := updaterExeName()
	if runtime.GOOS == "windows" && root != "" {
		if p := filepath.Join(root, "updater", name); isFile(p) {
			return p
		}
	}
	if daemonExe != "" {
		if p := filepath.Join(filepath.Dir(daemonExe), name); isFile(p) {
			return p
		}
	}
	if p, err := lookPath(name); err == nil {
		return p
	}
	return ""
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func lookPath(name string) (string, error) { return exec.LookPath(name) }

// spawnArgs builds the updater command line. Everything travels in
// argv so the (possibly elevated) updater never reads user env.
func spawnArgs(updaterBin, root, version, resultFile string) []string {
	return []string{
		updaterBin, "apply",
		"--install-root", root,
		"--version", version,
		"--result-file", resultFile,
		"--wait-timeout", updaterWaitTimeout.String(),
	}
}

// reconcileBoot consumes leftover update files at daemon startup and
// reports a one-shot status seed (nil = nothing to announce):
//   - a result announces updated/failed once, then is consumed;
//   - a result-less pending means the updater never ran (auth
//     declined, killed): cleared so the next check offers a retry
//     instead of wedging on "already installed".
func reconcileBoot(dataDir string) *bootSeed {
	var res ResultUpdate
	if err := readJSON(resultPath(dataDir), &res); err == nil && res.Version != "" {
		os.Remove(resultPath(dataDir))
		os.Remove(pendingPath(dataDir))
		return &bootSeed{version: res.Version, ok: res.OK, errMsg: res.Error}
	}
	var pend PendingUpdate
	if err := readJSON(pendingPath(dataDir), &pend); err == nil && pend.Version != "" {
		os.Remove(pendingPath(dataDir))
	}
	return nil
}

// bootSeed is a consumed result awaiting announcement.
type bootSeed struct {
	version string
	ok      bool
	errMsg  string
}

// friendlySpawnError keeps elevation cancels calm: a declined auth
// prompt is "not now", never a scary failure.
func friendlySpawnError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "cancel") || strings.Contains(msg, "dismiss") ||
		strings.Contains(msg, "auth") || strings.Contains(msg, "elevation") {
		return "The installer was not started. Your current version is untouched — try again when ready."
	}
	if len(msg) > 220 {
		msg = msg[:220] + "…"
	}
	return "The installer could not be started (" + msg + "). Your current version is untouched."
}

// failedSpawnError is the terminal honest message when the updater
// itself reports failure or never reports.
func failedSpawnError() string {
	return "The update could not be installed. Your current version is still safe."
}
