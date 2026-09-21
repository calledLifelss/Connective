// Package routing owns routing posture: global vs rule-based mode,
// LAN bypass, and verification that core-applied routes are in effect.
// sing-box applies the data-plane routes (auto_route); this package
// selects the mode rendered into the config and validates the result.
// Data-plane manipulation lands in the helper + generated sing-box config (see ARCHITECTURE.md).
package routing

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNotImplemented is returned by operations needing the phase-2
// privileged helper.
var ErrNotImplemented = errors.New("routing: data-plane changes need the privileged helper")

// Mode is the routing posture. Every visible mode maps to real config
// output in configgen; no UI-only modes exist.
type Mode string

const (
	// ModeGlobal routes (almost) everything through the proxy.
	ModeGlobal Mode = "global"
	// ModeRules bypasses LAN/private and applies rule sets.
	ModeRules Mode = "rules"
)

// Manager selects and verifies routing state.
type Manager interface {
	// Apply renders mode into the active core config generation inputs.
	Apply(Mode) error
	// Current returns the active mode.
	Current() Mode
	// Verify checks the OS route table matches expectations.
	Verify() error
}

// Stub is a placeholder Manager.
type Stub struct{ mode Mode }

// NewStub returns a Stub starting in ModeGlobal.
func NewStub() *Stub { return &Stub{mode: ModeGlobal} }

// Apply implements Manager.
func (s *Stub) Apply(m Mode) error {
	if m != ModeGlobal && m != ModeRules {
		return errors.New("routing: unknown mode")
	}
	s.mode = m
	return nil
}

// Current implements Manager.
func (s *Stub) Current() Mode { return s.mode }

// Verify implements Manager.
func (s *Stub) Verify() error { return ErrNotImplemented }

// DefaultRoute returns the system default route's interface and gateway
// by reading /proc/net/route (unprivileged, always truthful).
func DefaultRoute() (iface, gateway string, err error) {
	raw, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(raw), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 3 && f[1] == "00000000" && f[2] != "00000000" {
			return f[0], hexIP(f[2]), nil
		}
	}
	return "", "", fmt.Errorf("routing: no default route found")
}

// EgressIface returns the interface the kernel would actually use for
// destination ip, equivalent to `ip route get` and therefore honoring
// policy rules. This is the only honest check under sing-box >= 1.14,
// whose auto_route installs table-2022 policy routing instead of
// replacing the main-table default.
func EgressIface(ip string) (string, error) {
	out, err := exec.Command("ip", "route", "get", ip).Output()
	if err != nil {
		return "", fmt.Errorf("routing: ip route get: %w", err)
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("routing: no dev in %q", strings.TrimSpace(string(out)))
}

// VerifyTUNDefault confirms effective egress: via the TUN device when
// expectTUN, anywhere else otherwise. It probes a public IP so the check
// reflects the real forwarding decision, not table cosmetics.
func VerifyTUNDefault(tunName string, expectTUN bool) error {
	iface, err := EgressIface("8.8.8.8")
	if err != nil {
		return err
	}
	isTUN := iface == tunName
	if expectTUN && !isTUN {
		return fmt.Errorf("routing: egress via %q, expected TUN %q", iface, tunName)
	}
	if !expectTUN && isTUN {
		return fmt.Errorf("routing: egress unexpectedly via TUN %q", tunName)
	}
	return nil
}

// hexIP decodes the little-endian hex greed from /proc/net/route.
func hexIP(h string) string {
	if len(h) != 8 {
		return h
	}
	var b [4]byte
	for i := 0; i < 4; i++ {
		var v byte
		for j := 0; j < 2; j++ {
			c := h[i*2+j]
			var d byte
			switch {
			case c >= '0' && c <= '9':
				d = c - '0'
			case c >= 'a' && c <= 'f':
				d = c - 'a' + 10
			case c >= 'A' && c <= 'F':
				d = c - 'A' + 10
			default:
				return h
			}
			v = v*16 + d
		}
		b[3-i] = v
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}
