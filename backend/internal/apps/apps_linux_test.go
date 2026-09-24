//go:build !windows

package apps

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests drive List() through a fake /proc tree. On Windows,
// List() inventories via tasklist instead and ignores procRoot, so
// they are Linux-only by construction.
func TestListFakeProc(t *testing.T) {
	root := t.TempDir()
	mk := func(pid, comm string) {
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("1", "firefox")
	mk("2", "firefox")
	mk("3", "Discord")
	mk("notapid", "ignored")
	old := procRoot
	procRoot = root
	defer func() { procRoot = old }()
	apps := List()
	if len(apps) != 2 {
		t.Fatalf("want 2 distinct apps, got %v", apps)
	}
	if apps[0].ID != "firefox" || apps[0].Count != 2 {
		t.Fatalf("most-instances first: %+v", apps)
	}
	if apps[1].ID != "discord" || apps[1].Count != 1 {
		t.Fatalf("second: %+v", apps)
	}
}

func TestListMissingProcRootEmpty(t *testing.T) {
	old := procRoot
	procRoot = filepath.Join(t.TempDir(), "nope")
	defer func() { procRoot = old }()
	if got := List(); len(got) != 0 {
		t.Fatalf("unreadable table should be empty, got %v", got)
	}
}
