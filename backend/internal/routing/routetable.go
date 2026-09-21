package routing

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// routeEntry is one IPv4 route-table row. Portable: parsing and
// longest-prefix match are pure logic over `route print -4` text,
// shared by the Windows implementation and its unit tests.
type routeEntry struct {
	Dest    uint32
	Mask    uint32
	Gateway net.IP
	Iface   net.IP
	Metric  int
}

// parseIPv4Table parses `route print -4` IPv4 Route Table rows.
func parseIPv4Table(out string) ([]routeEntry, error) {
	var entries []routeEntry
	inTable := false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Network Destination") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "=") {
			if len(entries) > 0 {
				break
			}
			continue
		}
		f := strings.Fields(trimmed)
		if len(f) < 5 {
			continue
		}
		dest := parseIPv4(f[0])
		mask := parseIPv4(f[1])
		gw := net.ParseIP(f[2])
		iface := net.ParseIP(f[3])
		metric, _ := strconv.Atoi(f[4])
		if dest == nil || mask == nil || iface == nil {
			continue
		}
		if gw == nil {
			// "On-link": directly attached, gateway is the interface.
			if !strings.EqualFold(f[2], "On-link") {
				continue
			}
			gw = iface
		}
		entries = append(entries, routeEntry{
			Dest: ipToUint32(dest), Mask: ipToUint32(mask),
			Gateway: gw, Iface: iface, Metric: metric,
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("routing: empty IPv4 route table")
	}
	return entries, nil
}

// longestMatch implements longest-prefix match with metric tie-break
// (lowest metric wins), mirroring the stack's selection.
func longestMatch(entries []routeEntry, dst net.IP) (routeEntry, bool) {
	d := ipToUint32(dst.To4())
	if dst.To4() == nil {
		return routeEntry{}, false
	}
	best := -1
	bestBits := -1
	bestMetric := 0
	for i, e := range entries {
		if d&e.Mask != e.Dest {
			continue
		}
		bits := maskBits(e.Mask)
		if bits > bestBits || (bits == bestBits && e.Metric < bestMetric) {
			best, bestBits, bestMetric = i, bits, e.Metric
		}
	}
	if best < 0 {
		return routeEntry{}, false
	}
	return entries[best], true
}

func parseIPv4(s string) net.IP {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil {
		return nil
	}
	return ip.To4()
}

func ipToUint32(ip net.IP) uint32 {
	v := ip.To4()
	if v == nil {
		return 0
	}
	return uint32(v[0])<<24 | uint32(v[1])<<16 | uint32(v[2])<<8 | uint32(v[3])
}

func maskBits(m uint32) int {
	n := 0
	for i := 31; i >= 0; i-- {
		if m&(1<<uint(i)) == 0 {
			break
		}
		n++
	}
	return n
}
