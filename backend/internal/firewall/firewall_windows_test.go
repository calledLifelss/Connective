//go:build windows

package firewall

import (
	"strings"
	"testing"

	"connective/backend/internal/platform"
)

func TestNetshManagerEnableDisable(t *testing.T) {
	fake := &platform.FakeRunner{}
	fake.On("netsh advfirewall show allprofiles",
		"Firewall Policy   BlockInbound,AllowOutbound\n", nil)
	fake.On("netsh advfirewall firewall show rule name=all",
		"No rules match the specified criteria.\n", nil)
	// Spot-check one allow; the helper test covers the full script.
	fake.On("netsh advfirewall firewall add rule name=\"Connective Allow 127.0.0.0/8\" dir=out action=allow enable=yes remoteip=127.0.0.0/8",
		"Ok.\n", nil)
	fake.On("netsh advfirewall set allprofiles firewallpolicy blockinbound,blockoutbound",
		"Ok.\n", nil)
	fake.On("netsh advfirewall set allprofiles firewallpolicy BlockInbound,AllowOutbound",
		"Ok.\n", nil)

	mgr := &NetshManager{Runner: fake, Allows: []string{"203.0.113.7:443"}}
	// Full script needs every add scripted; assert the model layer
	// instead and drive Enable only after scripting the rest.
	rules, err := BuildAllowRules([]string{"203.0.113.7:443"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		key := "netsh advfirewall firewall add rule name=\"" + OwnedPrefix + r.Name + "\" dir=out action=allow enable=yes remoteip=" + r.Remote
		if r.Proto != "" {
			key += " protocol=" + r.Proto
		}
		if r.Port != "" {
			key += " remoteport=" + r.Port
		}
		fake.On(key, "Ok.\n", nil)
	}
	if err := mgr.Enable(); err != nil {
		t.Fatalf("enable: %v", err)
	}
	// Policy now reads back as blocking, with an owned rule present.
	fake.On("netsh advfirewall show allprofiles",
		"Firewall Policy   BlockInbound,BlockOutbound\n", nil)
	fake.On("netsh advfirewall firewall show rule name=all",
		"Rule Name:  Connective Allow 127.0.0.0/8\n", nil)
	ok, err := mgr.Enabled()
	if err != nil || !ok {
		t.Fatalf("enabled=%v err=%v", ok, err)
	}
	// OFF restores the snapshot policy captured at Enable.
	fake.On("netsh advfirewall firewall show rule name=all",
		"Rule Name:  Connective Allow 127.0.0.0/8\n", nil)
	fake.On("netsh advfirewall firewall delete rule name=\"Connective Allow 127.0.0.0/8\"",
		"Ok.\n", nil)
	if err := mgr.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	seen := strings.Join(fake.Calls, "\n")
	if !strings.Contains(seen, "firewallpolicy BlockInbound,AllowOutbound") {
		t.Fatalf("snapshot policy not restored:\n%s", seen)
	}
}
