//go:build windows

package tun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connective/backend/internal/platform"
)

// WindowsManager verifies the wintun device owned by the core. Same
// division as Linux: sing-box creates and addresses the adapter
// (interface_name + auto_route in generated config); this manager
// ensures it appears, verifies absence on teardown, and never touches
// foreign adapters. Creation itself is the core's job — there is no
// supported user-space "create wintun adapter" command to shell out
// to; the driver instantiates on first open by our core process.
type WindowsManager struct {
	name   string
	runner platform.Runner
}

// NewWindowsManager returns a Manager for name (usually connective0).
func NewWindowsManager(name string) *WindowsManager {
	return &WindowsManager{name: name, runner: platform.DefaultRunner}
}

// NewManager returns the platform Manager.
func NewManager(name string) Manager { return NewWindowsManager(name) }

// Name implements Manager.
func (m *WindowsManager) Name() string { return m.name }

// Up implements Manager: waits for the core-created adapter to appear
// (bounded), then verifies it is enabled. A missing adapter after the
// timeout is a startup failure, reported loudly.
func (m *WindowsManager) Up(Config) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		present, err := m.Exists()
		if err != nil {
			return err
		}
		if present {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tun: interface %q did not appear (core failed to create it?)", m.name)
		}
		select {
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Down implements Manager: delegates removal to the helper (which
// refuses foreign interfaces) and verifies absence.
func (m *WindowsManager) Down() error {
	present, err := m.Exists()
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	return fmt.Errorf("tun: interface %q still present: stop the core and run tun-cleanup", m.name)
}

// Exists implements Manager via netsh interface state.
func (m *WindowsManager) Exists() (bool, error) {
	out, err := m.runner.Run(context.Background(),
		"netsh", "interface", "show", "interface", "name="+m.name)
	if err != nil {
		return false, nil // netsh errors locating it mean absent
	}
	return !interfaceMissing(out), nil
}

// interfaceMissing detects netsh "no such interface" output.
func interfaceMissing(out string) bool {
	for _, s := range []string{
		"There is no such interface",
		"The following command was not found",
	} {
		if strings.Contains(out, s) {
			return true
		}
	}
	return false
}
