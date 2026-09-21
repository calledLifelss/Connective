//go:build !windows

package platform

import (
	"os"
	"strings"
)

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
