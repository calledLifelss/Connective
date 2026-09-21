// Package dns owns DNS posture: system vs custom vs proxy-routed
// resolution, and leak verification for the selected routing mode.
// sing-box applies the data plane (dns.servers/rules in generated
// config); leak checks and resolv.conf handling land in phase 2.
package dns

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
)

// ErrNotImplemented is returned by operations needing the phase-2
// privileged helper.
var ErrNotImplemented = errors.New("dns: resolver manipulation lands with the privileged helper")

// Mode is the DNS posture.
type Mode string

const (
	// ModeSystem keeps the OS resolver untouched.
	ModeSystem Mode = "system"
	// ModeCustom uses user-supplied upstream servers.
	ModeCustom Mode = "custom"
	// ModeProxyAware routes all DNS through the proxy (anti-leak).
	ModeProxyAware Mode = "proxy-aware"
)

// Config is the desired DNS state.
type Config struct {
	Mode    Mode
	Servers []string // ModeCustom upstreams
}

// Manager selects and verifies DNS state.
type Manager interface {
	Apply(Config) error
	Current() Config
	// VerifyNoLeak checks no plaintext DNS escapes outside the tunnel
	// when the mode requires it.
	VerifyNoLeak() error
}

// Stub is a placeholder Manager.
type Stub struct{ cfg Config }

// NewStub returns a Stub starting in ModeSystem.
func NewStub() *Stub { return &Stub{cfg: Config{Mode: ModeSystem}} }

// Apply implements Manager.
func (s *Stub) Apply(c Config) error {
	if c.Mode != ModeSystem && c.Mode != ModeCustom && c.Mode != ModeProxyAware {
		return errors.New("dns: unknown mode")
	}
	s.cfg = c
	return nil
}

// Current implements Manager.
func (s *Stub) Current() Config { return s.cfg }

// VerifyNoLeak implements Manager.
func (s *Stub) VerifyNoLeak() error { return ErrNotImplemented }

// ResolvConfHash snapshots /etc/resolv.conf so the daemon can prove it
// neither changed the system resolver on connect nor left modifications
// behind on disconnect.
func ResolvConfHash() (string, error) {
	raw, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// HashBytes snapshots arbitrary resolver state (Windows).
func HashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
