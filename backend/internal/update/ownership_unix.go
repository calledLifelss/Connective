//go:build !windows

package update

import (
	"os"
	"path/filepath"
	"syscall"
)

// adoptOwner hands an elevated writer's file to the owner of its
// directory. result.json is written by the root updater inside the
// user's data dir: a root-owned 0600 file there was unreadable by the
// daemon's reconcileBoot, so the boot announcement never happened (and
// the file leaked forever). No-op when owners already match.
func adoptOwner(path string) {
	dirSt, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return
	}
	dirStat, ok := dirSt.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	fSt, err := os.Stat(path)
	if err != nil {
		return
	}
	fStat, ok := fSt.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	if fStat.Uid == dirStat.Uid && fStat.Gid == dirStat.Gid {
		return
	}
	_ = os.Chown(path, int(dirStat.Uid), int(dirStat.Gid))
}
