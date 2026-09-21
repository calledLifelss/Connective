//go:build windows

package sysproxy

import (
	"errors"
	"strings"
	"testing"

	"connective/backend/internal/platform"
)

const regQueryProxy = "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings\n" +
	"    ProxyEnable    REG_DWORD    0x0\n" +
	"    ProxyServer    REG_SZ    old-proxy:8080\n"

func TestWindowsProxyApplyRestore(t *testing.T) {
	fake := &platform.FakeRunner{}
	fake.On(`reg query HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyEnable`,
		"    ProxyEnable    REG_DWORD    0x0\n", nil)
	fake.On(`reg query HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyServer`,
		"    ProxyServer    REG_SZ    old-proxy:8080\n", nil)
	fake.On(`reg query HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyOverride`,
		"ERROR: The system was unable to find the specified registry key or value.\n", errors.New("exit status 1"))
	fake.On(`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyServer /t REG_SZ /d 127.0.0.1:10808 /f`,
		"The operation completed successfully.\n", nil)
	fake.On(`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyOverride /t REG_SZ /d <local>;192.168.0.0/16 /f`,
		"The operation completed successfully.\n", nil)
	fake.On(`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyEnable /t REG_DWORD /d 1 /f`,
		"The operation completed successfully.\n", nil)
	// Restore: values that existed come back, created ones are deleted.
	fake.On(`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyEnable /t REG_DWORD /d 0x0 /f`,
		"The operation completed successfully.\n", nil)
	fake.On(`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyServer /t REG_SZ /d old-proxy:8080 /f`,
		"The operation completed successfully.\n", nil)
	fake.On(`reg delete HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings /v ProxyOverride /f`,
		"The operation completed successfully.\n", nil)

	mgr := &WindowsManager{}
	mgr.runner = fake
	if err := mgr.Apply(Config{Enabled: true, Host: "127.0.0.1", Port: 10808, Bypass: []string{"192.168.0.0/16"}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if ok, _ := mgr.Active(); !ok {
		t.Fatal("must report active after apply")
	}
	if err := mgr.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if ok, _ := mgr.Active(); ok {
		t.Fatal("must report inactive after restore")
	}
	seen := strings.Join(fake.Calls, "\n")
	if !strings.Contains(seen, "ProxyEnable") || !strings.Contains(seen, "/d 127.0.0.1:10808") {
		t.Fatalf("proxy not programmed:\n%s", seen)
	}
}
