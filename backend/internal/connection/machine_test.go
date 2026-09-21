package connection

import (
	"testing"
)

func connectPath(t *testing.T, m *Machine) {
	t.Helper()
	for _, step := range []State{
		StSelecting, StConnecting, StStartingCore,
		StInitializingTUN, StApplyingRouting, StConnected,
	} {
		if err := m.Transition(step, "test"); err != nil {
			t.Fatalf("step %q: %v", step, err)
		}
	}
}

func TestHappyPath(t *testing.T) {
	m := NewMachine()
	if m.State() != StDisconnected {
		t.Fatalf("initial state = %q", m.State())
	}
	connectPath(t, m)
	if !m.Connected() {
		t.Errorf("should be connected")
	}
	if err := m.Transition(StDisconnecting, "user"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if err := m.Transition(StDisconnected, "done"); err != nil {
		t.Fatalf("disconnected: %v", err)
	}
	if m.Connected() {
		t.Errorf("should not be connected")
	}
}

func TestIllegalTransition(t *testing.T) {
	m := NewMachine()
	if err := m.Transition(StConnected, "skip"); err == nil {
		t.Errorf("expected illegal-transition error")
	}
	if m.State() != StDisconnected {
		t.Errorf("state must not change on illegal transition")
	}
}

func TestFailoverPath(t *testing.T) {
	m := NewMachine()
	connectPath(t, m)
	for _, step := range []State{StDegraded, StSwitchingServer, StConnecting} {
		if err := m.Transition(step, "failover"); err != nil {
			t.Fatalf("step %q: %v", step, err)
		}
	}
}

func TestSwitchingToDegraded(t *testing.T) {
	m := NewMachine()
	connectPath(t, m)
	if err := m.Transition(StDegraded, "probe"); err != nil {
		t.Fatal(err)
	}
	if err := m.Transition(StSwitchingServer, "switch"); err != nil {
		t.Fatal(err)
	}
	// A failed post-switch verification must land back on degraded,
	// never wedge the machine in switching-server.
	if err := m.Transition(StDegraded, "new path bad"); err != nil {
		t.Fatalf("switching->degraded: %v", err)
	}
}

func TestReset(t *testing.T) {
	m := NewMachine()
	m.Reset("idle")
	if m.State() != StDisconnected {
		t.Fatalf("reset from disconnected should stay put")
	}
	connectPath(t, m)
	var got []Event
	m.Subscribe(func(e Event) { got = append(got, e) })
	m.Reset("operator")
	if m.State() != StDisconnected {
		t.Fatalf("reset should force disconnected, got %q", m.State())
	}
	if len(got) != 1 || got[0].From != StConnected || got[0].Reason != "operator" {
		t.Fatalf("bad reset event: %+v", got)
	}
}

func TestEvents(t *testing.T) {
	m := NewMachine()
	var got []Event
	m.Subscribe(func(e Event) { got = append(got, e) })
	m.SetServer("srv1")
	if err := m.Transition(StSelecting, "auto"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].From != StDisconnected || got[0].To != StSelecting {
		t.Fatalf("bad events: %+v", got)
	}
	if got[0].Server != "srv1" {
		t.Errorf("event should carry server id")
	}
	if len(m.History()) != 1 {
		t.Errorf("history should have 1 entry")
	}
}
