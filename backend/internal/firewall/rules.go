package firewall

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// OwnedPrefix owns every firewall object we create. Cleanup deletes by
// this prefix and never touches foreign rules.
const OwnedPrefix = "Connective "

// DefaultOutbound is the stock Windows policy we restore on disable.
const DefaultOutbound = "blockinbound,allowoutbound"

// LAN cidrs + loopback always allowed under lockdown (mirrors the
// Linux nftables model: established, lo, LAN, DHCP, TUN, endpoints).
var lanCIDRs = []string{
	"127.0.0.0/8", "::1",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fe80::/10",
}

// AllowRule describes one owned allow rule (netsh argv form).
type AllowRule struct {
	Name   string
	Remote string // remoteip CIDR or address
	Proto  string // "" = any, else TCP/UDP
	Port   string // remoteport, "" = any
}

// BuildAllowRules composes the owned allow set: LAN/loopback, DHCP,
// and each explicit VPN endpoint (TCP+UDP). Pure function — unit
// tested, shared by the helper and the Manager.
func BuildAllowRules(allows []string) ([]AllowRule, error) {
	var out []AllowRule
	for _, cidr := range lanCIDRs {
		out = append(out, AllowRule{Name: "Allow " + cidr, Remote: cidr})
	}
	out = append(out, AllowRule{Name: "Allow DHCP", Remote: "255.255.255.255", Proto: "UDP", Port: "67"})
	for _, a := range allows {
		ip, port, ok := SplitIPPort(a)
		if !ok {
			return nil, fmt.Errorf("firewall: bad allow %q (want ip:port)", a)
		}
		// Hardening: only a parseable IP and numeric port reach netsh
		// arguments (the daemon formats them, but this runs elevated).
		parsed := net.ParseIP(ip)
		if parsed == nil {
			return nil, fmt.Errorf("firewall: bad allow %q (invalid ip)", a)
		}
		pnum, err := strconv.Atoi(port)
		if err != nil || pnum < 1 || pnum > 65535 {
			return nil, fmt.Errorf("firewall: bad allow %q (invalid port)", a)
		}
		ip = parsed.String()
		for _, proto := range []string{"TCP", "UDP"} {
			out = append(out, AllowRule{
				Name:   "Allow " + ip + " " + port + "/" + proto,
				Remote: ip, Proto: proto, Port: port,
			})
		}
	}
	return out, nil
}

// SplitIPPort splits "ip:port".
func SplitIPPort(a string) (ip, port string, ok bool) {
	i := strings.LastIndex(a, ":")
	if i <= 0 || i == len(a)-1 {
		return "", "", false
	}
	return a[:i], a[i+1:], true
}
