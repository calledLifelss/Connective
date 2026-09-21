// Package routing owns routing posture: global vs rule-based mode,
// LAN bypass, and verification that core-applied routes are in effect.
// sing-box applies the data-plane routes (auto_route); this package
// selects the mode rendered into the config and validates the result.
// Data-plane manipulation lands in the helper + generated sing-box config (see ARCHITECTURE.md).
package routing

import "errors"

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
