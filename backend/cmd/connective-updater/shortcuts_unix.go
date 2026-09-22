//go:build !windows

package main

// rewriteShortcuts is a Windows-only step (Linux launches through the
// /opt/current symlink chain, no shortcuts to maintain).
func rewriteShortcuts(root, version string) error { return nil }
