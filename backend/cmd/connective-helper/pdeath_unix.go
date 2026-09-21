//go:build !windows

package main

import "syscall"

// armDeathSig makes the upcoming core (via syscall.Exec below) die if
// its supervisor chain goes away. Survives the Exec.
func armDeathSig() {
	_, _, _ = syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGTERM), 0)
}
