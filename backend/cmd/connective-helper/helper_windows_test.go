//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"connective/backend/internal/platform"
)

// withFakeRunner swaps the tool runner for deterministic fixtures.
func withFakeRunner(t *testing.T) *platform.FakeRunner {
	t.Helper()
	old := winRunner
	t.Cleanup(func() { winRunner = old })
	fake := &platform.FakeRunner{}
	winRunner = fake
	return fake
}

func TestStopCoreGonePidSucceedsWindows(t *testing.T) {
	fake := withFakeRunner(t)
	fake.On("tasklist /FI PID eq 999991 /FO CSV /NH",
		"INFO: No tasks are running which match the specified criteria.\n", nil)
	if err := stopCore([]string{"--pid", "999991"}); err != nil {
		t.Fatalf("gone pid must succeed, got %v", err)
	}
}

func TestStopCoreRefusesParentProcess(t *testing.T) {
	fake := withFakeRunner(t)
	ppid := os.Getppid()
	// Parent (test binary) is alive and is not sing-box.
	fake.On("tasklist /FI PID eq "+strconv.Itoa(ppid)+" /FO CSV /NH",
		`"go.exe","`+strconv.Itoa(ppid)+`","Console","1","10,000 K"`+"\n", nil)
	fake.On("wmic process where ProcessId="+strconv.Itoa(ppid)+" get ExecutablePath,CommandLine /format:csv",
		"Node,CommandLine,ExecutablePath\nX,\"C:\\go\\bin\\go.exe test\",C:\\go\\bin\\go.exe\n", nil)
	err := stopCore([]string{"--pid", strconv.Itoa(ppid)})
	if err == nil {
		t.Fatal("stop-core must refuse a live non-core pid")
	}
	if !strings.Contains(err.Error(), "does not look like") {
		t.Fatalf("refusal must name the guard, got %q", err)
	}
}

func TestKillswitchValidationWindows(t *testing.T) {
	_ = withFakeRunner(t)
	if err := killswitch(true, []string{"--tun", "connective0", "--allow", "bogus"}); err == nil {
		t.Fatal("bad --allow must fail without touching the firewall")
	}
	if err := killswitch(true, []string{"--tun", "connective0"}); err == nil {
		t.Fatal("missing --state must fail")
	}
	if err := killswitch(true, []string{"--bogus"}); err == nil {
		t.Fatal("unknown flag must fail")
	}
}

const winPolicyFixture = "Domain Profile Settings:\n    Firewall Policy                                   BlockInbound,AllowOutbound\n"

func TestKillswitchOnOffWindows(t *testing.T) {
	fake := withFakeRunner(t)
	fake.On("netsh advfirewall show allprofiles", winPolicyFixture, nil)
	fake.On("netsh advfirewall firewall show rule name=all",
		"No rules match the specified criteria.\n", nil)
	add := func(key string) {
		fake.On("netsh advfirewall firewall add rule "+key, "Ok.\n", nil)
	}
	add("name=\"Connective Allow 127.0.0.0/8\" dir=out action=allow enable=yes remoteip=127.0.0.0/8")
	add("name=\"Connective Allow ::1\" dir=out action=allow enable=yes remoteip=::1")
	add("name=\"Connective Allow 10.0.0.0/8\" dir=out action=allow enable=yes remoteip=10.0.0.0/8")
	add("name=\"Connective Allow 172.16.0.0/12\" dir=out action=allow enable=yes remoteip=172.16.0.0/12")
	add("name=\"Connective Allow 192.168.0.0/16\" dir=out action=allow enable=yes remoteip=192.168.0.0/16")
	add("name=\"Connective Allow fe80::/10\" dir=out action=allow enable=yes remoteip=fe80::/10")
	add("name=\"Connective Allow DHCP\" dir=out action=allow enable=yes remoteip=255.255.255.255 protocol=UDP remoteport=67")
	fake.On("netsh advfirewall firewall delete rule name=\"Connective Allow 127.0.0.0/8\"", "Ok.\n", nil)
	fake.On("netsh advfirewall set allprofiles firewallpolicy blockinbound,blockoutbound", "Ok.\n", nil)

	state := filepath.Join(t.TempDir(), "ks.state")
	if err := killswitch(true, []string{"--tun", "connective0", "--state", state}); err != nil {
		t.Fatalf("killswitch-on: %v", err)
	}
	if raw, err := os.ReadFile(state); err != nil || !strings.Contains(string(raw), "BlockInbound,AllowOutbound") {
		t.Fatalf("policy snapshot wrong: %q %v", raw, err)
	}

	// OFF restores the snapshot and deletes only owned rules.
	fake.On("netsh advfirewall firewall show rule name=all",
		"Rule Name:                            Connective Allow 127.0.0.0/8\n\nRule Name:                            Windows Defender\n", nil)
	fake.On("netsh advfirewall set allprofiles firewallpolicy BlockInbound,AllowOutbound", "Ok.\n", nil)
	if err := killswitch(false, []string{"--state", state}); err != nil {
		t.Fatalf("killswitch-off: %v", err)
	}
	for _, c := range fake.Calls {
		if strings.Contains(c, "Windows Defender") && strings.Contains(c, "delete") {
			t.Fatalf("foreign rule must never be deleted: %q", c)
		}
	}
}

func TestTunCleanupWindows(t *testing.T) {
	fake := withFakeRunner(t)
	if err := tunCleanup([]string{"--if", "Ethernet0"}); err == nil {
		t.Fatal("foreign interface must be refused")
	}
	fake.On("netsh interface show interface name=connective0",
		"There is no such interface.\n", nil)
	if err := tunCleanup([]string{"--if", "connective0"}); err != nil {
		t.Fatalf("absent own interface must succeed: %v", err)
	}
}
