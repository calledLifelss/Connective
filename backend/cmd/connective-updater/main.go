// Command connective-updater performs update activation outside the
// running application: verify staged tree → atomically switch versions →
// launch the new build → health-check → roll back on failure.
//
// Flow: Connective exits → updater installs → updater launches
// Connective → updater exits. The updater survives the main process
// exiting because the daemon spawns it detached with explicit paths;
// it never trusts the provider for locations (root is an operator flag,
// confined by the installer).
//
// Usage:
//
//	connective-updater apply --install-root DIR --version X.Y.Z \
//	  --launch /path/to/new/connective [--health-timeout 60s]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		return fmt.Errorf("usage: connective-updater apply --install-root DIR --version VER [--launch BIN]")
	}
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	root := fs.String("install-root", "", "versioned install root")
	version := fs.String("version", "", "target version (must be assembled)")
	launch := fs.String("launch", "", "new binary to launch after activation")
	health := fs.Duration("health-timeout", 60*time.Second, "new-version health deadline")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *root == "" || *version == "" {
		return fmt.Errorf("install-root and version are required")
	}
	in := &update.Installer{Root: *root, HealthTimeout: *health}
	prev, err := in.Activate(*version)
	if err != nil {
		return err
	}
	fmt.Printf("activated %s (previous %s)\n", *version, prev)

	ready := func() bool { return in.Current() == *version }
	if *launch != "" {
		cmd := exec.Command(*launch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("launch: %w", err)
		}
		// The child outlives us by design (daemon re-parenting);
		// health is observed through the install root + process liveness.
		pid := cmd.Process.Pid
		ready = func() bool {
			return processAlive(pid) && in.Current() == *version
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
	fmt.Printf("update to %s healthy\n", *version)
	return nil
}
