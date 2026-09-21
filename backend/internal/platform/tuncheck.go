package platform

import (
	"fmt"
	"os"
	"strings"
)

// OwnTunName is Connective's TUN device. Everything else tunnel-like is
// treated as foreign.
const OwnTunName = "connective0"

// foreignTunError formats the user-facing safe-failure message. It must
// stay stable: the Flutter friendlyError() maps on the "Another VPN/TUN"
// prefix and shows it verbatim.
func foreignTunError(names []string) error {
	return fmt.Errorf("Another VPN/TUN interface is active (%s). "+
		"Connective cannot safely initialize its TUN connection in the current network state. "+
		"Disconnect the other VPN first, or turn off TUN in Routing settings for proxy-only mode.",
		strings.Join(names, ", "))
}

// CheckSafeForTun fails when a foreign tunnel is up. Callers must abort
// the connect before starting the core so we never enter a routing
// conflict. Proxy-only mode (TUN disabled) does not need this check.
func CheckSafeForTun() error {
	if others := OtherTunInterfaces(); len(others) > 0 {
		return foreignTunError(others)
	}
	return nil
}

// isVirtualNoise reports interfaces that are never VPN tunnels:
// container/bridge plumbing that must not block a connect.
func isVirtualNoise(lower string) bool {
	for _, p := range []string{"lo", "docker", "veth", "virbr", "br-", "lxcbr", "mpqemubr"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// looksLikeTunnelName is the name-heuristic layer. It catches common VPN
// device names; the tun_flags sysfs check below catches TUN/TAP devices
// regardless of name.
func looksLikeTunnelName(lower string) bool {
	return strings.HasPrefix(lower, "tun") ||
		strings.HasPrefix(lower, "tap") ||
		strings.HasPrefix(lower, "utun") ||
		strings.HasPrefix(lower, "wg") ||
		strings.HasPrefix(lower, "tailscale") ||
		strings.HasPrefix(lower, "nordlynx") ||
		strings.HasPrefix(lower, "proton") ||
		strings.Contains(lower, "tun")
}

// hasTunFlags reports whether the kernel marks this interface as a
// TUN/TAP device (definitive, name-independent).
func hasTunFlags(name string) bool {
	if _, err := os.Stat("/sys/class/net/" + name + "/tun_flags"); err == nil {
		return true
	}
	return false
}

// OtherTunInterfaces lists up network interfaces that look like someone
// else's tunnel (tun/tap/utun/wg, excluding our own device). Used to
// warn before connecting into a conflict instead of failing cryptically
// at route-application time.
func OtherTunInterfaces() []string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if name == OwnTunName {
			continue
		}
		lower := strings.ToLower(name)
		if isVirtualNoise(lower) {
			continue
		}
		if !looksLikeTunnelName(lower) && !hasTunFlags(name) {
			continue
		}
		state, err := os.ReadFile("/sys/class/net/" + name + "/operstate")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(state)) == "up" ||
			strings.TrimSpace(string(state)) == "unknown" {
			out = append(out, name)
		}
	}
	return out
}
