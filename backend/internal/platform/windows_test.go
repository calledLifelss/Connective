//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run on Windows CI (and compile-check everywhere via
// GOOS=windows go vet). Fixtures are deterministic; no machine state
// is touched except t.TempDir().
func TestWindowsVirtualNoise(t *testing.T) {
	for _, n := range []string{"Loopback", "Bluetooth", "vEthernet (WSL)", "Hyper-V Virtual", "Npcap Loopback"} {
		if !isWindowsVirtualNoise(strings.ToLower(n)) {
			t.Fatalf("%q should be noise", n)
		}
	}
	for _, n := range []string{"Ethernet", "Wi-Fi", "connective0", "WireGuard"} {
		if isWindowsVirtualNoise(strings.ToLower(n)) {
			t.Fatalf("%q must not be noise", n)
		}
	}
}

func TestOtherTunInterfacesWindows(t *testing.T) {
	old := DefaultRunner
	defer func() { DefaultRunner = old }()
	fake := &FakeRunner{}
	// own device excluded; foreign up tunnel reported; down ignored.
	fake.On("netsh interface show interface", "Admin State  State          Type             Interface Name\n"+
		"Enabled        Connected      Dedicated        connective0\n"+
		"Enabled        Connected      Dedicated        ProtonVPN Tunnel\n"+
		"Enabled        Disconnected   Dedicated        NordLynx\n"+
		"Enabled        Connected      Dedicated        Ethernet\n", nil)
	fake.On("wintun list", "", errors.New("no wintun tool"))
	DefaultRunner = fake
	found := OtherTunInterfaces()
	if len(found) != 1 || found[0] != "ProtonVPN Tunnel" {
		t.Fatalf("got %v", found)
	}
}

func TestTrustedBinaryWindows(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "helper.exe")
	if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := TrustedBinary(f); err == nil {
		t.Errorf("temp-dir binary must be refused")
	}
	if err := TrustedBinary("relative\\helper.exe"); err == nil {
		t.Errorf("relative path must be refused")
	}
	if err := TrustedBinary(filepath.Join(dir, "helper")); err == nil {
		t.Errorf("non-.exe must be refused")
	}
	if err := TrustedBinary(`C:\nonexistent-xyz\helper.exe`); err == nil {
		t.Errorf("missing file must be refused")
	}
}

func TestElevateCommandQuoting(t *testing.T) {
	bin, args := ElevateCommand(`C:\Program Files\Connective\connective-helper.exe`,
		[]string{"killswitch-on", "--tun", "connective0", "--allow", "1.2.3.4:443"})
	if bin != "powershell" {
		t.Fatalf("bin %q", bin)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-Verb RunAs -Wait") {
		t.Fatalf("missing UAC verb: %q", joined)
	}
	if strings.Contains(joined, "connective-helper.exe', 'killswitch-on") == false {
		t.Fatalf("args not passed through: %q", joined)
	}
	bin2, args2 := ElevateCommand(`C:\x\h.exe`, []string{"a'b"})
	if bin2 != "powershell" || !strings.Contains(strings.Join(args2, " "), "a''b") {
		t.Fatalf("single quotes must be doubled: %q", args2)
	}
}
