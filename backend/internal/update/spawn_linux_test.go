//go:build !windows

package update

import (
	"reflect"
	"testing"
)

// The pkexec reshape once dropped the updater binary and asked pkexec
// to run "apply" (log: "Cannot run program apply"), wedging the UI on
// restarting with no result ever written.
func TestElevateCommandKeepsBinary(t *testing.T) {
	bin := "/opt/connective/versions/0.4.0/connective-updater"
	args := spawnArgs(bin, "/opt/connective", "0.5.0", "/data/stage", "/data/result.json")
	prog, progArgs := elevateCommand(bin, args, []string{"pkexec"})
	if prog != "pkexec" {
		t.Fatalf("prog = %q", prog)
	}
	want := []string{bin, "apply", "--install-root", "/opt/connective",
		"--version", "0.5.0", "--result-file", "/data/result.json",
		"--wait-timeout", updaterWaitTimeout.String(), "--staged-dir", "/data/stage"}
	if !reflect.DeepEqual(progArgs, want) {
		t.Fatalf("progArgs = %q, want %q", progArgs, want)
	}
}

func TestElevateCommandMultiWordRunner(t *testing.T) {
	bin := "/opt/updater"
	args := []string{bin, "apply", "--version", "1.0.0"}
	prog, progArgs := elevateCommand(bin, args, []string{"sudo", "-n"})
	if prog != "sudo" {
		t.Fatalf("prog = %q", prog)
	}
	want := []string{"-n", bin, "apply", "--version", "1.0.0"}
	if !reflect.DeepEqual(progArgs, want) {
		t.Fatalf("progArgs = %q, want %q", progArgs, want)
	}
}

func TestUpdaterCommandDirect(t *testing.T) {
	bin := "/tmp/updater"
	args := []string{bin, "apply", "--version", "1.0.0"}
	prog, progArgs := updaterCommand(bin, args)
	if prog != bin {
		t.Fatalf("prog = %q", prog)
	}
	if !reflect.DeepEqual(progArgs, []string{"apply", "--version", "1.0.0"}) {
		t.Fatalf("progArgs = %q", progArgs)
	}
}
