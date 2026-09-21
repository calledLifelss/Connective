package routing

import (
	"net"
	"testing"
)

// Portable: real `route print -4` fixture exercising parsing,
// longest-prefix match, and metric tie-break.
const fixtureRoutePrint = `===========================================================================
Interface List
  7...Intel Ethernet
 21...connective0
===========================================================================

IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0      192.168.1.1    192.168.1.50     25
        127.0.0.0        255.0.0.0         On-link         127.0.0.1    331
      192.168.1.0    255.255.255.0         On-link      192.168.1.50    281
     198.18.0.0    255.255.0.0         On-link       198.18.0.1     15
===========================================================================
Persistent Routes:
  None
`

func TestParseIPv4Table(t *testing.T) {
	entries, err := parseIPv4Table(fixtureRoutePrint)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries", len(entries))
	}
}

func TestLongestMatch(t *testing.T) {
	entries, err := parseIPv4Table(fixtureRoutePrint)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dst  string
		want string // egress interface IP
	}{
		{"8.8.8.8", "192.168.1.50"},     // default
		{"192.168.1.9", "192.168.1.50"}, // LAN, not default
		{"198.18.0.7", "198.18.0.1"},    // TUN range wins over default
		{"127.0.0.5", "127.0.0.1"},      // loopback
	}
	for _, c := range cases {
		best, ok := longestMatch(entries, net.ParseIP(c.dst))
		if !ok {
			t.Fatalf("no match for %s", c.dst)
		}
		if best.Iface.String() != c.want {
			t.Errorf("%s: egress %s, want %s", c.dst, best.Iface, c.want)
		}
	}
}

func TestMetricTieBreak(t *testing.T) {
	mk := func(iface string, metric int) routeEntry {
		return routeEntry{
			Dest: 0, Mask: 0,
			Gateway: net.ParseIP("192.168.1.1"),
			Iface:   net.ParseIP(iface), Metric: metric,
		}
	}
	entries := []routeEntry{mk("10.0.0.2", 50), mk("10.0.0.3", 10)}
	best, ok := longestMatch(entries, net.ParseIP("8.8.8.8"))
	if !ok || best.Iface.String() != "10.0.0.3" {
		t.Fatalf("lowest metric must win: %+v %v", best, ok)
	}
}

func TestParseEmptyTable(t *testing.T) {
	if _, err := parseIPv4Table("nothing here"); err == nil {
		t.Fatal("empty table must fail")
	}
}
