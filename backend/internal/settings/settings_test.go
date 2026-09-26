package settings

import (
	"strings"
	"testing"
)

// B4: kill switch + split tunneling are mutually exclusive; the update
// path rejects the combination instead of silently rewriting it.
func TestConflictKillSwitchVsSplit(t *testing.T) {
	for _, mode := range []string{SplitBypass, SplitOnly} {
		s := Defaults()
		s.KillSwitch = true
		s.SplitMode = mode
		if err := s.Conflict(); err == nil {
			t.Fatalf("killswitch + split %q must conflict", mode)
		}
	}
	s := Defaults()
	s.KillSwitch = true
	s.SplitMode = SplitOff
	if err := s.Conflict(); err != nil {
		t.Fatalf("killswitch with split off must be fine: %v", err)
	}
	s = Defaults()
	s.KillSwitch = false
	s.SplitMode = SplitBypass
	if err := s.Conflict(); err != nil {
		t.Fatalf("split without killswitch must be fine: %v", err)
	}
}

// Load-time resolution: a legacy document carrying both loses split
// tunneling (kill switch is the stronger guarantee).
func TestValidateResolvesLegacyConflict(t *testing.T) {
	s := Defaults()
	s.KillSwitch = true
	s.SplitMode = SplitOnly
	s.SplitApps = []string{"firefox"}
	s.Validate()
	if s.SplitMode != SplitOff {
		t.Fatalf("split mode must resolve to off, got %q", s.SplitMode)
	}
}

func TestValidateNormalizesSplitApps(t *testing.T) {
	s := Defaults()
	s.SplitApps = []string{
		"C:\\Program Files\\Mozilla Firefox\\firefox.exe",
		"/usr/bin/Firefox", // duplicates firefox after normalization
		"discord",
		"",
		"bad name!",
	}
	s.Validate()
	want := []string{"firefox", "discord"}
	if len(s.SplitApps) != len(want) {
		t.Fatalf("got %v, want %v", s.SplitApps, want)
	}
	for i, w := range want {
		if s.SplitApps[i] != w {
			t.Fatalf("got %v, want %v", s.SplitApps, want)
		}
	}
}

func TestValidateCapsSplitApps(t *testing.T) {
	s := Defaults()
	for i := 0; i < MaxSplitApps+50; i++ {
		s.SplitApps = append(s.SplitApps, "app"+strings.Repeat("x", 3)+string(rune('a'+i%26))+string(rune('a'+i/26%26)))
	}
	s.Validate()
	if len(s.SplitApps) > MaxSplitApps {
		t.Fatalf("cap %d not enforced: %d entries", MaxSplitApps, len(s.SplitApps))
	}
}

func TestValidateRangeFixes(t *testing.T) {
	s := Settings{MixedPort: 99999, TestTimeoutMs: 1, RoutingMode: "nope",
		DNSMode: "nope", MTU: 1, SplitMode: "nope"}
	s.Validate()
	if s.MixedPort != 10808 || s.TestTimeoutMs != 5000 || s.RoutingMode != "global" ||
		s.DNSMode != "proxy-aware" || s.MTU != 9000 || s.SplitMode != SplitOff {
		t.Fatalf("bad fixes: %+v", s)
	}
}
