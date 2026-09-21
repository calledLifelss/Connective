package firewall

import (
	"strings"
	"testing"
)

// Portable: the owned rule model is pure logic shared by the helper
// and the Windows Manager.
func TestBuildAllowRules(t *testing.T) {
	rules, err := BuildAllowRules([]string{"203.0.113.7:443"})
	if err != nil {
		t.Fatal(err)
	}
	// LAN + loopback + DHCP + 2 endpoint rules (TCP/UDP).
	names := map[string]bool{}
	for _, r := range rules {
		names[r.Name] = true
		if r.Remote == "" {
			t.Fatalf("rule %q has no remote", r.Name)
		}
	}
	for _, want := range []string{"Allow 127.0.0.0/8", "Allow 10.0.0.0/8", "Allow DHCP", "Allow 203.0.113.7 443/TCP", "Allow 203.0.113.7 443/UDP"} {
		if !names[want] {
			t.Errorf("missing rule %q (have %v)", want, names)
		}
	}
	if _, err := BuildAllowRules([]string{"bogus"}); err == nil {
		t.Error("bad allow must fail")
	}
}

func TestOwnedPrefix(t *testing.T) {
	if !strings.HasPrefix(OwnedPrefix+"x", "Connective ") {
		t.Fatal("owned rules must carry the Connective prefix")
	}
}
