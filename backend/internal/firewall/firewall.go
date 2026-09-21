// Package firewall implements the kill switch: when enabled, unproxied
// outbound traffic is blocked at the packet filter so a dead tunnel can
// never leak. The toggle is meaningless without enforcement, so this
// package exposes only state + interface until the nftables backend
// (phase 2, privileged helper) exists. There is deliberately no UI-only
// toggle anywhere in the tree.
package firewall

import "errors"

// ErrNotImplemented is returned by every reserved interface.
var ErrNotImplemented = errors.New("firewall: nftables backend lives in the helper + generated sing-box config (needs privileged helper)")

// Manager enforces the kill switch.
type Manager interface {
	// Enable blocks all non-tunnel egress (except LAN/loopback).
	Enable() error
	// Disable restores normal egress.
	Disable() error
	// Enabled reports enforcement state.
	Enabled() (bool, error)
}

// Stub is a placeholder Manager that is always disabled.
type Stub struct{}

// NewStub returns a Stub.
func NewStub() *Stub { return &Stub{} }

// Enable implements Manager.
func (s *Stub) Enable() error { return ErrNotImplemented }

// Disable implements Manager.
func (s *Stub) Disable() error { return ErrNotImplemented }

// Enabled implements Manager.
func (s *Stub) Enabled() (bool, error) { return false, nil }
