package apps

import (
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

func TestParseTasklistLine(t *testing.T) {
	cases := map[string]string{
		`"firefox.exe","1234","Console","1","200,000 K"`: "firefox.exe",
		`"System Idle Process","0","Services","0","8 K"`: "System Idle Process",
		``:              "",
		`garbage`:       "",
		`"unterminated`: "",
	}
	for in, want := range cases {
		if got := parseTasklistLine(in); got != want {
			t.Errorf("parseTasklistLine(%q) = %q, want %q", in, got, want)
		}
	}
}
