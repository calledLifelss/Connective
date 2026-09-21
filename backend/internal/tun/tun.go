// Package tun owns the TUN device lifecycle: create, address, destroy,
// stale-device cleanup. The actual device is configured by the core
// (sing-box tun inbound with auto_route) while this package verifies
// state and cleans up leftovers. Full netlink management lives in the privileged helper + generated sing-box config (see ARCHITECTURE.md § Privileges).
package tun

import "errors"

// ErrNotImplemented is returned by every reserved interface.
var ErrNotImplemented = errors.New("tun: full implementation lives in the helper + generated sing-box config (needs privileged helper + core binary)")

// Config describes the desired device.
type Config struct {
	Name    string
	Address string
	MTU     int
}

// Manager controls one TUN device.
type Manager interface {
	// Up ensures the device exists with the given config.
	Up(Config) error
	// Down removes the device.
	Down() error
	// Exists reports whether the device is present.
	Exists() (bool, error)
	// Name returns the interface name.
	Name() string
}

// Stub is a placeholder Manager that always reports ErrNotImplemented.
// It keeps the wiring compile-safe without pretending to work.
type Stub struct{ name string }

// NewStub returns a Stub for name.
func NewStub(name string) *Stub { return &Stub{name: name} }

// Name implements Manager.
func (s *Stub) Name() string { return s.name }

// Up implements Manager.
func (s *Stub) Up(Config) error { return ErrNotImplemented }

// Down implements Manager.
func (s *Stub) Down() error { return ErrNotImplemented }

// Exists implements Manager.
func (s *Stub) Exists() (bool, error) { return false, ErrNotImplemented }
