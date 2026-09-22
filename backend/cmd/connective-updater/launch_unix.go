//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// startDetached prepares a child that outlives the updater.
func startDetached(bin string) *exec.Cmd {
	cmd := exec.Command(bin)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	return cmd
}
