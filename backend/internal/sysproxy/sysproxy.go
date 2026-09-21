// Package sysproxy manages the OS-level system proxy settings
// (gsettings on GNOME, kwriteconfig on KDE Plasma, registry on Windows).
// Applied on connect in proxy mode, restored on disconnect. The desktop
// backends land in phase 2; the interface is fixed now so the state
// machine and settings can be built against it.
package sysproxy

import "errors"

// ErrNotImplemented is returned by every reserved interface.
var ErrNotImplemented = errors.New("sysproxy: desktop backends land in phase 2")

// Config is the desired system proxy state.
type Config struct {
	Enabled bool
	Host    string
	Port    int
	// Bypass are hosts that skip the proxy (LAN, loopback).
	Bypass []string
}

// Manager applies and restores system proxy settings.
type Manager interface {
	Apply(Config) error
	Restore() error
	Active() (bool, error)
}

// Stub is a placeholder Manager.
type Stub struct{ active bool }

// NewStub returns a Stub.
func NewStub() *Stub { return &Stub{} }

// Apply implements Manager.
func (s *Stub) Apply(Config) error { return ErrNotImplemented }

// Restore implements Manager.
func (s *Stub) Restore() error { return ErrNotImplemented }

// Active implements Manager.
func (s *Stub) Active() (bool, error) { return s.active, nil }
