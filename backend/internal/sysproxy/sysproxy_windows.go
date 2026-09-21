//go:build windows

package sysproxy

import (
	"context"
	"fmt"
	"strings"

	"connective/backend/internal/platform"
)

// Windows system proxy lives in HKCU (per-user, no elevation needed):
// HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
//
//	ProxyEnable (DWORD), ProxyServer, ProxyOverride.
//
// reg.exe is used over any registry library to stay stdlib-only.
// Values are snapshotted before Apply so Restore returns exactly.
const (
	regKey      = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	valEnable   = "ProxyEnable"
	valServer   = "ProxyServer"
	valOverride = "ProxyOverride"
)

// WindowsManager applies/restores the system proxy. Same interface as
// the desktop backends; used in proxy routing modes.
type WindowsManager struct {
	runner platform.Runner
	prev   map[string]string
	active bool
}

// NewWindowsManager returns a Manager.
func NewWindowsManager() *WindowsManager {
	return &WindowsManager{runner: platform.DefaultRunner, prev: map[string]string{}}
}

// NewManager returns the platform Manager.
func NewManager() Manager { return NewWindowsManager() }

func (m *WindowsManager) run(args ...string) (string, error) {
	return m.runner.Run(context.Background(), "reg", args...)
}

func (m *WindowsManager) query(name string) (string, bool) {
	out, err := m.run("query", regKey, "/v", name)
	if err != nil {
		return "", false
	}
	// "    ProxyEnable    REG_DWORD    0x1"
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && strings.EqualFold(f[0], name) {
			return strings.Join(f[2:], " "), true
		}
	}
	return "", false
}

// Apply implements Manager: snapshots current values, then enables the
// proxy to host:port with LAN/loopback bypass preserved.
func (m *WindowsManager) Apply(c Config) error {
	if !c.Enabled {
		return m.Restore()
	}
	if m.prev == nil {
		m.prev = map[string]string{}
	}
	for _, v := range []string{valEnable, valServer, valOverride} {
		if val, ok := m.query(v); ok {
			if _, seen := m.prev[v]; !seen {
				m.prev[v] = val
			}
		}
	}
	bypass := "<local>"
	if len(c.Bypass) > 0 {
		bypass = "<local>;" + strings.Join(c.Bypass, ";")
	}
	server := fmt.Sprintf("%s:%d", c.Host, c.Port)
	if _, err := m.run("add", regKey, "/v", valServer, "/t", "REG_SZ", "/d", server, "/f"); err != nil {
		return fmt.Errorf("sysproxy: set server: %w", err)
	}
	if _, err := m.run("add", regKey, "/v", valOverride, "/t", "REG_SZ", "/d", bypass, "/f"); err != nil {
		return fmt.Errorf("sysproxy: set bypass: %w", err)
	}
	if _, err := m.run("add", regKey, "/v", valEnable, "/t", "REG_DWORD", "/d", "1", "/f"); err != nil {
		return fmt.Errorf("sysproxy: enable: %w", err)
	}
	m.active = true
	return nil
}

// Restore implements Manager: writes back the snapshot (deleting keys
// we created when nothing was there before).
func (m *WindowsManager) Restore() error {
	for _, v := range []string{valEnable, valServer, valOverride} {
		prev, had := m.prev[v]
		typ := "REG_SZ"
		if v == valEnable {
			typ = "REG_DWORD"
		}
		if !had {
			_, _ = m.run("delete", regKey, "/v", v, "/f")
			continue
		}
		if _, err := m.run("add", regKey, "/v", v, "/t", typ, "/d", prev, "/f"); err != nil {
			return fmt.Errorf("sysproxy: restore %s: %w", v, err)
		}
	}
	m.active = false
	return nil
}

// Active implements Manager.
func (m *WindowsManager) Active() (bool, error) { return m.active, nil }
