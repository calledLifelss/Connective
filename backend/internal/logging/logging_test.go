package logging

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	msg := Redact(`connecting uuid=11111111-2222-4333-8444-555555555555 as user`)
	if strings.Contains(msg, "11111111") {
		t.Errorf("uuid leaked: %q", msg)
	}
	if !strings.Contains(msg, "uuid=***") {
		t.Errorf("marker lost: %q", msg)
	}
	clean := Redact("core started, tun initialized")
	if clean != "core started, tun initialized" {
		t.Errorf("clean message altered: %q", clean)
	}
}

func TestLevelsAndBuffer(t *testing.T) {
	l := New(INFO, 3)
	l.Debug("hidden")
	l.Info("one")
	l.Warn("two")
	l.Error("three")
	l.Error("four")
	got := l.Recent(0, DEBUG)
	if len(got) != 3 {
		t.Fatalf("ring should hold 3, got %d", len(got))
	}
	if got[0].Message != "two" || got[2].Message != "four" {
		t.Fatalf("wrong eviction order: %+v", got)
	}
	filtered := l.Recent(0, WARN)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 >= WARN, got %d", len(filtered))
	}
	l.Clear()
	if len(l.Recent(0, DEBUG)) != 0 {
		t.Errorf("clear failed")
	}
}
