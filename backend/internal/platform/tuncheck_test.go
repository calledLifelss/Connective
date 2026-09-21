package platform

import (
	"strings"
	"testing"
)

func TestOtherTunInterfaces(t *testing.T) {
	// Must never crash and never report loopback or our own device.
	found := OtherTunInterfaces()
	for _, name := range found {
		if name == "lo" || name == OwnTunName {
			t.Fatalf("must not report %q", name)
		}
	}
	t.Logf("foreign tunnels visible here: %v", found)
}

func TestVirtualNoiseExcluded(t *testing.T) {
	for _, n := range []string{"docker0", "veth1234", "virbr0", "br-abc", "lo"} {
		if !isVirtualNoise(strings.ToLower(n)) {
			t.Fatalf("%q should be treated as virtual noise", n)
		}
	}
	for _, n := range []string{"tun0", "wg0", "proton0", "nordlynx", "tailscale0"} {
		if isVirtualNoise(strings.ToLower(n)) {
			t.Fatalf("%q must not be excluded as noise", n)
		}
	}
}

func TestLooksLikeTunnelName(t *testing.T) {
	for _, n := range []string{"tun0", "tap0", "utun3", "wg0", "tailscale0", "proton0", "nordlynx", "mytun0"} {
		if !looksLikeTunnelName(strings.ToLower(n)) {
			t.Fatalf("%q should look like a tunnel", n)
		}
	}
	if looksLikeTunnelName("eth0") || looksLikeTunnelName("wlan0") {
		t.Fatalf("plain NICs must not look like tunnels")
	}
}

func TestForeignTunErrorMessage(t *testing.T) {
	err := foreignTunError([]string{"tun0"})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Another VPN/TUN interface is active") {
		t.Fatalf("message must carry the stable user-facing prefix, got %q", msg)
	}
	if !strings.Contains(msg, "tun0") {
		t.Fatalf("message must name the interface, got %q", msg)
	}
	if !strings.Contains(msg, "cannot safely initialize") {
		t.Fatalf("message must explain the safe failure, got %q", msg)
	}
}
