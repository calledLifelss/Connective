package routing

import (
	"context"
	"fmt"
	"net"
	"strings"

	"connective/backend/internal/platform"
)

// DefaultRoute returns the system default route's interface IP and
// gateway by parsing `route print -4` (unprivileged, always truthful).
func DefaultRoute() (iface, gateway string, err error) {
	entries, err := ipv4Table()
	if err != nil {
		return "", "", err
	}
	best, ok := longestMatch(entries, net.ParseIP("8.8.8.8"))
	if !ok {
		return "", "", fmt.Errorf("routing: no default route found")
	}
	// Interface is reported as its IP; resolve a friendly name when
	// possible, but the IP itself is the truthful identifier.
	return best.Iface.String(), best.Gateway.String(), nil
}

// EgressIface returns the interface IP the stack would use for ip, by
// longest-prefix match over the real IPv4 table. This honors
// sing-box auto_route additions the same way `route print` shows them.
func EgressIface(ip string) (string, error) {
	dst := net.ParseIP(strings.TrimSpace(ip))
	if dst == nil {
		return "", fmt.Errorf("routing: bad ip %q", ip)
	}
	entries, err := ipv4Table()
	if err != nil {
		return "", err
	}
	best, ok := longestMatch(entries, dst)
	if !ok {
		return "", fmt.Errorf("routing: no route to %q", ip)
	}
	return best.Iface.String(), nil
}

// VerifyTUNDefault confirms effective egress: via the TUN device when
// expectTUN, anywhere else otherwise. Same call shape as unix (takes
// the TUN name); the name resolves to its interface address first,
// because Windows wintun adapters have no stable short ifname for the
// route table. An literal IP passes through untouched (tests).
func VerifyTUNDefault(tun string, expectTUN bool) error {
	addr := tun
	if net.ParseIP(tun) == nil {
		var err error
		addr, err = TunAddress(tun)
		if err != nil {
			return err
		}
	}
	iface, err := EgressIface("8.8.8.8")
	if err != nil {
		return err
	}
	isTUN := iface == addr
	if expectTUN && !isTUN {
		return fmt.Errorf("routing: egress via %q, expected TUN %q", iface, addr)
	}
	if !expectTUN && isTUN {
		return fmt.Errorf("routing: egress unexpectedly via TUN %q", addr)
	}
	return nil
}

// TunAddress resolves a TUN interface name to its IPv4 address via
// `netsh interface ip show addresses`.
func TunAddress(name string) (string, error) {
	out, err := platform.DefaultRunner.Run(context.Background(),
		"netsh", "interface", "ip", "show", "addresses", "name="+name)
	if err != nil {
		return "", fmt.Errorf("routing: tun address: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "IP Address") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if ip := net.ParseIP(strings.TrimSpace(parts[1])); ip != nil {
					return ip.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("routing: no address on %q", name)
}

// ipv4Table fetches `route print -4` and parses it (see routetable.go).
func ipv4Table() ([]routeEntry, error) {
	out, err := platform.DefaultRunner.Run(context.Background(),
		"route", "print", "-4")
	if err != nil {
		return nil, fmt.Errorf("routing: route print: %w", err)
	}
	return parseIPv4Table(out)
}
