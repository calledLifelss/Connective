package update

// adoptOwner is a no-op on Windows: elevated processes keep the user's
// SID there, so a result.json written under the data dir was always
// readable by the daemon.
func adoptOwner(path string) {}
