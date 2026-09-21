package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestTunCleanupRefusesForeign(t *testing.T) {
	for _, foreign := range []string{"tun0", "wg0", "tailscale0", "eth0"} {
		err := tunCleanup([]string{"--if", foreign})
		if err == nil {
			t.Fatalf("tun-cleanup --if %s must be refused", foreign)
		}
		if !strings.Contains(err.Error(), "foreign") {
			t.Fatalf("error must say foreign, got %q", err)
		}
	}
}

func TestTunCleanupRequiresFlag(t *testing.T) {
	if err := tunCleanup(nil); err == nil {
		t.Fatalf("missing --if must fail")
	}
}

func TestStopCoreRequiresPid(t *testing.T) {
	if err := stopCore(nil); err == nil {
		t.Fatalf("missing --pid must fail")
	}
}

func TestStopCoreRefusesBadPids(t *testing.T) {
	for _, bad := range []string{"abc", "0", "1", "-5", "12a", "1.5", ""} {
		if err := stopCore([]string{"--pid", bad}); err == nil {
			t.Fatalf("stop-core --pid %q must be refused", bad)
		}
	}
}

func TestStopCoreRefusesSelf(t *testing.T) {
	if err := stopCore([]string{"--pid", strconv.Itoa(os.Getpid())}); err == nil {
		t.Fatalf("stop-core must refuse the helper's own pid")
	}
}
