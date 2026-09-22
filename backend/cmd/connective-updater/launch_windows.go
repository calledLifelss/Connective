package main

import (
	"os/exec"
)

// startDetached prepares a child that outlives the updater.
func startDetached(bin string) *exec.Cmd {
	cmd := exec.Command(bin)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd
}
