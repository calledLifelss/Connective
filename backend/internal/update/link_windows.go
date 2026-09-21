package update

import "os"

// replaceLink swaps newLink into place at dest. Windows rename cannot
// replace an existing link, so the stale one is removed first. The
// updater performs activation only while the app is closed, bounding
// the window to microseconds with no reader.
func replaceLink(newLink, dest string) error {
	_ = os.Remove(dest)
	return os.Rename(newLink, dest)
}
