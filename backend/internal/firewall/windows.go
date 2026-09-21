//go:build windows

package firewall

import (
	"context"
	"fmt"
	"strings"

	"connective/backend/internal/platform"
)

// NetshManager enforces the kill switch through Windows Firewall. It runs
// wherever the process is already elevated (helper context, tests);
// the unprivileged daemon reaches it exclusively via the helper.
//
// It satisfies the shared Manager interface exactly (Enable/Disable/
// Enabled with no arguments); configuration travels in Allows and
// RestorePolicy so CLI parsing stays in the helper.
type NetshManager struct {
	Runner platform.Runner
	Allows []string
	// RestorePolicy overrides the snapshot on Disable (the helper
	// passes the persisted pre-lockdown value).
	RestorePolicy string

	prevPolicy string
}

// Compile-time proof it satisfies the shared Manager interface.
var _ Manager = (*NetshManager)(nil)

func (m *NetshManager) runner() platform.Runner {
	if m.Runner != nil {
		return m.Runner
	}
	return platform.DefaultRunner
}

// Enable blocks all non-tunnel egress except owned allows, after
// snapshotting the previous policy for Disable.
func (m *NetshManager) Enable() error {
	ctx := context.Background()
	prev, err := ReadPolicy(m.runner())
	if err != nil {
		return err
	}
	m.prevPolicy = prev
	rules, err := BuildAllowRules(m.Allows)
	if err != nil {
		return err
	}
	if err := DeleteOwned(m.runner()); err != nil {
		return err
	}
	for _, r := range rules {
		if err := addRule(m.runner(), r); err != nil {
			return err
		}
	}
	if out, err := m.runner().Run(ctx, "netsh", "advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,blockoutbound"); err != nil {
		return fmt.Errorf("firewall: set policy: %s: %w", oneLine(out), err)
	}
	return nil
}

// Disable restores the snapshot (or RestorePolicy) and deletes owned rules.
func (m *NetshManager) Disable() error {
	policy := m.prevPolicy
	if m.RestorePolicy != "" {
		policy = m.RestorePolicy
	}
	if policy == "" {
		policy = DefaultOutbound
	}
	if err := DeleteOwned(m.runner()); err != nil {
		return err
	}
	ctx := context.Background()
	if out, err := m.runner().Run(ctx, "netsh", "advfirewall", "set", "allprofiles", "firewallpolicy", policy); err != nil {
		return fmt.Errorf("firewall: restore policy: %s: %w", oneLine(out), err)
	}
	return nil
}

// Enabled reports whether our lockdown is in force: block-outbound
// policy plus at least one owned rule present.
func (m NetshManager) Enabled() (bool, error) {
	ctx := context.Background()
	out, err := m.runner().Run(ctx, "netsh", "advfirewall", "show", "allprofiles")
	if err != nil {
		return false, err
	}
	blocked := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Firewall Policy") && strings.Contains(strings.ToLower(line), "blockoutbound") {
			blocked = true
		}
	}
	if !blocked {
		return false, nil
	}
	rules, err := m.runner().Run(ctx, "netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	if err != nil {
		return false, err
	}
	return strings.Contains(rules, OwnedPrefix), nil
}

// ReadPolicy returns the current allprofiles firewall policy. Real
// netsh output is whitespace-columnar ("Firewall Policy" +
// spaces + value), but colon-separated variants are tolerated.
func ReadPolicy(r platform.Runner) (string, error) {
	ctx := context.Background()
	out, err := r.Run(ctx, "netsh", "advfirewall", "show", "allprofiles")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "Firewall Policy") {
			continue
		}
		rest := line
		if i := strings.Index(rest, ":"); i >= 0 {
			rest = rest[i+1:]
		} else {
			rest = strings.TrimPrefix(rest, "Firewall Policy")
		}
		if p := strings.ReplaceAll(strings.TrimSpace(rest), " ", ""); p != "" {
			// Sanity: a policy is comma-joined inbound/outbound halves.
			if strings.Contains(p, ",") {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("firewall: could not parse policy")
}

// DeleteOwned removes only Connective-owned rules.
func DeleteOwned(r platform.Runner) error {
	ctx := context.Background()
	out, err := r.Run(ctx, "netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	if err != nil {
		return err
	}
	var firstErr error
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "Rule Name:"))
		if line == name || !strings.HasPrefix(name, OwnedPrefix) {
			continue
		}
		if _, err := r.Run(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name=\""+name+"\""); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func addRule(r platform.Runner, rule AllowRule) error {
	ctx := context.Background()
	args := []string{"advfirewall", "firewall", "add", "rule",
		"name=\"" + OwnedPrefix + rule.Name + "\"",
		"dir=out", "action=allow", "enable=yes",
		"remoteip=" + rule.Remote}
	if rule.Proto != "" {
		args = append(args, "protocol="+rule.Proto)
	}
	if rule.Port != "" {
		args = append(args, "remoteport="+rule.Port)
	}
	if out, err := r.Run(ctx, "netsh", args...); err != nil {
		return fmt.Errorf("firewall: allow %s: %s: %w", rule.Name, oneLine(out), err)
	}
	return nil
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
