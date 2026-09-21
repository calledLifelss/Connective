//go:build windows

package main

// processAlive on Windows: FindProcess always succeeds, so liveness
// comes from the install-root health signal observed by the caller.
func processAlive(pid int) bool { return pid > 0 }
