package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// TunExists reports whether the named TUN interface is present. Unix
// checks sysfs; Windows asks netsh. Used by daemon connect
// verification on both platforms.
func TunExists(name string) (bool, error) {
	if runtime.GOOS == "windows" {
		out, err := DefaultRunner.Run(context.Background(),
			"netsh", "interface", "show", "interface", "name="+name)
		if err != nil {
			return false, nil
		}
		if strings.Contains(out, "There is no such interface") {
			return false, nil
		}
		return true, nil
	}
	if _, err := os.Stat("/sys/class/net/" + name); err == nil {
		return true, nil
	}
	return false, nil
}

// RequireWintun fails fast with an actionable error when the Windows
// TUN driver is not bundled next to the core binary. sing-box loads
// wintun.dll from its own directory; without it, TUN connects die
// deep inside the core with an obscure log line. No-op off Windows.
func RequireWintun(corePath string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	dll := filepath.Join(filepath.Dir(corePath), "wintun.dll")
	if _, err := os.Stat(dll); err != nil {
		return fmt.Errorf("TUN needs %q next to sing-box.exe (reinstall Connective or place wintun.dll alongside the core)", dll)
	}
	return nil
}
