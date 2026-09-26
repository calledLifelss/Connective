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

// The picker must report the full executable basename, not the
// kernel-truncated comm: the core matches process_name against the exe
// basename, so a truncated id ("chromium-browse") can never match and
// the app silently stays on the VPN.
func TestListPrefersExeOverTruncatedComm(t *testing.T) {
	root := t.TempDir()
	mk := func(pid, comm, cmdline, exe string) {
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if comm != "" {
			if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if cmdline != "" {
			if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline+"\x00--flag\x00"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if exe != "" {
			if err := os.Symlink(exe, filepath.Join(dir, "exe")); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Long executable: comm is truncated, exe and argv[0] are not.
	mk("10", "chromium-browse", "/usr/lib64/chromium-browser/chromium-browser", "/usr/lib64/chromium-browser/chromium-browser")
	// No exe link (unreadable): argv[0] basename wins over comm.
	mk("11", "mycmd", "/opt/tools/my-long-tool-name", "")
	// Bare comm only: last resort, reported as-is.
	mk("12", "shorty", "", "")
	old := procRoot
	procRoot = root
	defer func() { procRoot = old }()
	apps := List()
	byID := map[string]App{}
	for _, a := range apps {
		byID[a.ID] = a
	}
	if _, ok := byID["chromium-browser"]; !ok {
		t.Fatalf("want full exe basename, got %v", apps)
	}
	if _, ok := byID["chromium-browse"]; ok {
		t.Fatalf("truncated comm must not leak through: %v", apps)
	}
	if _, ok := byID["my-long-tool-name"]; !ok {
		t.Fatalf("want argv[0] fallback, got %v", apps)
	}
	if _, ok := byID["shorty"]; !ok {
		t.Fatalf("want comm last resort, got %v", apps)
	}
}
