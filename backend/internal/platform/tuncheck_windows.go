package platform

import (
	"context"
	"strings"
)

// OtherTunInterfaces lists up network interfaces that look like someone
// else's tunnel on Windows. Enumeration uses `netsh interface show
// interface` (present on every supported Windows, no extra tools):
// only Connected tunnel-like adapters count. Our own device
// (OwnTunName) is excluded; loopback and well-known virtual noise
// (Hyper-V switches, container vNICs) never block a connect.
//
// A second layer consults the wintun driver bindings via
// `wintun list` when the helper ships it; absence of that tool is not
// an error — the name heuristic plus admin-state check is the gate,
// exactly like the sysfs tun_flags layer on Linux.
func OtherTunInterfaces() []string {
	out, err := DefaultRunner.Run(context.Background(),
		"netsh", "interface", "show", "interface")
	if err != nil {
		return nil
	}
	var found []string
	for _, line := range strings.Split(out, "\n") {
		state, name := parseNetshInterfaceLine(line)
		if name == "" || state != "connected" {
			continue
		}
		if strings.EqualFold(name, OwnTunName) {
			continue
		}
		lower := strings.ToLower(name)
		if isWindowsVirtualNoise(lower) {
			continue
		}
		if looksLikeTunnelName(lower) || isWintunBound(name) {
			found = append(found, name)
		}
	}
	return found
}

// isWindowsVirtualNoise excludes Hyper-V/container/virtual plumbing
// that must never block a connect.
func isWindowsVirtualNoise(lower string) bool {
	for _, p := range []string{
		"loopback", "bluetooth", "vethernet", "hyper-v", "wsl",
		"container", "br-", "npcap",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isWintunBound reports whether name has a wintun driver binding. It
// shells to the bundled wintun helper when present; any failure or
// absence means "unknown", never "foreign" — the name heuristic above
// remains the deciding layer.
func isWintunBound(name string) bool {
	out, err := DefaultRunner.Run(context.Background(),
		"wintun", "list")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), name) {
			return true
		}
	}
	return false
}
