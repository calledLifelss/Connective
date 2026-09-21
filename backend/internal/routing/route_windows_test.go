//go:build windows

package routing

import (
	"errors"
	"testing"

	"connective/backend/internal/platform"
)

func withRunner(t *testing.T) *platform.FakeRunner {
	t.Helper()
	old := runner
	t.Cleanup(func() { runner = old })
	fake := &platform.FakeRunner{}
	runner = fake
	return fake
}

// Regression: proxy-only mode (no TUN adapter present) must verify
// cleanly. The first Windows implementation resolved the TUN name
// unconditionally and failed every proxy-mode connect.
func TestVerifyProxyWithoutTunDevice(t *testing.T) {
	fake := withRunner(t)
	fake.On("netsh interface ip show addresses name=connective0",
		"", errors.New("no such interface"))
	if err := VerifyTUNDefault("connective0", false); err != nil {
		t.Fatalf("proxy mode without TUN device must pass: %v", err)
	}
	if err := VerifyTUNDefault("connective0", true); err == nil {
		t.Fatal("TUN mode without TUN device must fail")
	}
}
