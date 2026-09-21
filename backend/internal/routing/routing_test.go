package routing

import "testing"

func TestDefaultRouteLive(t *testing.T) {
	iface, gw, err := DefaultRoute()
	if err != nil {
		t.Skipf("no default route here: %v", err)
	}
	if iface == "" || gw == "" {
		t.Fatalf("empty route: %q %q", iface, gw)
	}
	t.Logf("default via %s dev %s", gw, iface)
}

func TestHexIP(t *testing.T) {
	// 192.168.1.1 little-endian hex as stored in /proc/net/route.
	if got := hexIP("0101A8C0"); got != "192.168.1.1" {
		t.Fatalf("got %q", got)
	}
	if got := hexIP("bogus"); got != "bogus" {
		t.Fatalf("bad input should pass through, got %q", got)
	}
}

func TestVerifyTUNDefaultLive(t *testing.T) {
	// Proxy-mode expectation: egress must NOT be the TUN device.
	if err := VerifyTUNDefault("connective0", false); err != nil {
		t.Fatalf("proxy-mode check: %v", err)
	}
	// TUN expectation fails honestly while disconnected.
	if err := VerifyTUNDefault("connective0", true); err == nil {
		t.Fatalf("expected failure while disconnected")
	}
}

func TestEgressIface(t *testing.T) {
	iface, err := EgressIface("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if iface != "lo" {
		t.Fatalf("loopback should egress via lo, got %q", iface)
	}
	iface, err = EgressIface("8.8.8.8")
	if err != nil || iface == "" {
		t.Fatalf("public egress undetermined: %q %v", iface, err)
	}
	t.Logf("public egress via %s", iface)
}
