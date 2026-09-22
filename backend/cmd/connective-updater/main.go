// Command connective-updater performs update activation outside the
// running application: wait for the app to close → sanity-check the
// staged tree → atomically switch versions → report for the next boot.
//
// Flow: app stages + spawns updater, user closes the app, updater
// installs, user starts the app again (which announces updated/failed
// from the updater's report). The updater never touches the network
// and never trusts the provider: all locations come from argv.
//
// Usage:
//
//	connective-updater apply --install-root DIR --version X.Y.Z
//	  --result-file PATH [--wait-timeout 15m] [--launch BIN]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"connective/backend/internal/update"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "connective-updater:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "apply" {
		return fmt.Errorf("usage: connective-updater apply --install-root DIR --version VER [--result-file PATH] [--launch BIN]")
	}
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	return runApply(fs, args[1:])
}

// waitExit polls for app processes (stubbed in tests: the dev machine
// itself runs connective builds).
var waitExit = waitForExit

// launchAndCheck starts the new build and health-checks it, rolling
// back on failure. Used with --launch (tests/future); production v1
// lets the user start the app normally instead of launching a GUI
// from a privileged process.
func launchAndCheck(in *update.Installer, launch, version string) error {
	ready := func() bool { return in.Current() == version }
	pid := 0
	if launch != "" {
		cmd := startDetached(launch)
		if cmd == nil {
			return fmt.Errorf("launch: unsupported platform")
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("launch: %w", err)
		}
		pid = cmd.Process.Pid
		ready = func() bool {
			return processAlive(pid) && in.Current() == version
		}
		_ = cmd.Process.Release()
	}
	ctx := context.Background()
	if err := in.AwaitHealthy(ctx, ready); err != nil {
		fmt.Println("health check failed, rolling back")
		if _, rerr := in.Rollback(); rerr != nil {
			return fmt.Errorf("%v (rollback also failed: %v)", err, rerr)
		}
		return err
	}
	fmt.Printf("update to %s healthy\n", version)
	_ = time.Now
	return nil
}
