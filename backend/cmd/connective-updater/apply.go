package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"connective/backend/internal/update"
)

// appNames are the processes that must be gone before activation
// (the updater itself is connective-updater and never matches).
func appNames() []string {
	if isWindows() {
		return []string{"connective.exe", "connectived.exe"}
	}
	return []string{"connective", "connectived"}
}

// requiredTreeFiles must exist in an assembled tree before it may be
// activated (guards against truncated/corrupt assemblies bricking the
// next start; content was hash-verified at download).
func requiredTreeFiles() []string {
	if isWindows() {
		return []string{"connective.exe", "connectived.exe", "sing-box.exe", "wintun.dll"}
	}
	return []string{"connective", "connectived", "sing-box"}
}

func runApply(fs *flag.FlagSet, args []string) error {
	root := fs.String("install-root", "", "versioned install root")
	version := fs.String("version", "", "target version (must be assembled)")
	launch := fs.String("launch", "", "new binary to launch after activation")
	health := fs.Duration("health-timeout", 60*time.Second, "new-version health deadline")
	resultFile := fs.String("result-file", "", "report path (JSON) for the next app boot")
	waitTO := fs.Duration("wait-timeout", 15*time.Minute, "how long to wait for the app to close")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *root == "" || *version == "" {
		return fmt.Errorf("install-root and version are required")
	}
	fail := func(err error) error {
		if *resultFile != "" {
			_ = update.WriteResult(*resultFile, *version, false, err.Error(), "")
		}
		return err
	}
	in := &update.Installer{Root: *root, HealthTimeout: *health}

	// 1. The app must be closed: locked files (Windows) and the live
	// tree cannot be replaced underneath it.
	if err := waitExit(appNames(), *waitTO); err != nil {
		return fail(fmt.Errorf("app did not close for install: %w", err))
	}
	// 2. Sanity: never activate a partial tree.
	if err := checkTree(*root, *version); err != nil {
		return fail(err)
	}
	// 3. Atomic activation (+ Windows pointer/shortcuts).
	prev, err := in.Activate(*version)
	if err != nil {
		return fail(err)
	}
	if isWindows() {
		if err := update.WriteCurrentTxt(*root, *version); err != nil {
			return fail(err)
		}
		if err := rewriteShortcuts(*root, *version); err != nil {
			return fail(err)
		}
	}
	fmt.Printf("activated %s (previous %s)\n", *version, prev)
	if err := update.WriteResult(*resultFile, *version, true, "", prev); *resultFile != "" && err != nil {
		return fmt.Errorf("report result: %w", err)
	}

	// 4. Optional launch + health (tests/future); production v1 lets
	// the user start the app normally (no elevated GUI, no display
	// assumptions from a privileged process).
	if *launch == "" {
		return nil
	}
	return launchAndCheck(in, *launch, *version)
}

// checkTree verifies the assembled version dir holds the required
// executables (present + executable on unix).
func checkTree(root, version string) error {
	dir, err := (&update.Installer{Root: root}).VersionDir(version)
	if err != nil {
		return err
	}
	for _, name := range requiredTreeFiles() {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return fmt.Errorf("update: assembled %s missing %s", version, name)
		}
		if !isWindows() && st.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("update: assembled %s not executable: %s", version, name)
		}
	}
	return nil
}

func isWindows() bool { return runtime.GOOS == "windows" }
