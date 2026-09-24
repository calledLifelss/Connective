package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAppID(t *testing.T) {
	cases := map[string]string{
		"Firefox":                        "firefox",
		"  discord.EXE  ":                "discord",
		"/usr/bin/spotify":               "spotify",
		`C:\Program Files\App\Slack.exe`: "slack",
		"my-app_v2.1+x":                  "my-app_v2.1+x",
		"":                               "",
		"   ":                            "",
		".exe":                           "",
		"has space":                      "",
		"evil;rm":                        "",
		"a/b/../c":                       "c",
		"UPPER.EXE":                      "upper",
	}
	for in, want := range cases {
		if got := NormalizeAppID(in); got != want {
			t.Errorf("NormalizeAppID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeListDedupesAndCaps(t *testing.T) {
	in := []string{"Firefox", "firefox", "FIREFOX.EXE", "", "  ", "discord", "has space", "slack"}
	got := NormalizeList(in, 3)
	if len(got) != 3 || got[0] != "firefox" || got[1] != "discord" || got[2] != "slack" {
		t.Fatalf("bad normalize: %v", got)
	}
	if got := NormalizeList(nil, 0); len(got) != 0 {
		t.Fatalf("nil should yield empty, got %v", got)
	}
}

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
