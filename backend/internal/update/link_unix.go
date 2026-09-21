//go:build !windows

package update

import "os"

// replaceLink atomically swaps newLink into place at dest.
func replaceLink(newLink, dest string) error {
	return os.Rename(newLink, dest)
}
