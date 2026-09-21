//go:build windows

package tun

import (
	"errors"
	"testing"

	"connective/backend/internal/platform"
)

func TestWindowsExistsParsing(t *testing.T) {
	mgr := NewWindowsManager("connective0")
	fake := &platform.FakeRunner{}
	mgr.runner = fake

	fake.On("netsh interface show interface name=connective0",
		"Admin State: Enabled\nState: Connected\n", nil)
	present, err := mgr.Exists()
	if err != nil || !present {
		t.Fatalf("present=%v err=%v", present, err)
	}

	fake.On("netsh interface show interface name=connective0",
		"There is no such interface.\n", errors.New("exit status 1"))
	present, err = mgr.Exists()
	if err != nil || present {
		t.Fatalf("absent must report cleanly: %v %v", present, err)
	}

	if mgr.Name() != "connective0" {
		t.Fatalf("name %q", mgr.Name())
	}
	if NewManager("connective0") == nil {
		t.Fatal("NewManager must return the platform manager")
	}
}
