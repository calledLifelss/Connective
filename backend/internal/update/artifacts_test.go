package update

import "testing"

func art(typ string, size int64, from string) Artifact {
	return Artifact{Type: typ, Filename: typ + ".zip", Size: size, SHA256: repeat("ab", 32), URL: typ + ".zip", FromVersion: from}
}

func TestDeltaPreferred(t *testing.T) {
	m := validManifest()
	m.Artifacts = []Artifact{art(ArtifactFull, 1000, ""), art(ArtifactDelta, 100, "0.2.0")}
	got, err := SelectArtifact("0.2.0", m)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != ArtifactDelta {
		t.Fatalf("smaller valid delta should win, got %s", got.Type)
	}
}

func TestFullFallbacks(t *testing.T) {
	cases := []struct {
		name string
		arts []Artifact
	}{
		{"delta bigger", []Artifact{art(ArtifactFull, 100, ""), art(ArtifactDelta, 500, "0.2.0")}},
		{"delta from other version", []Artifact{art(ArtifactFull, 100, ""), art(ArtifactDelta, 10, "0.1.0")}},
		{"delta invalid from", []Artifact{art(ArtifactFull, 100, ""), {Type: ArtifactDelta, Filename: "d", Size: 10, SHA256: repeat("ab", 32), URL: "d", FromVersion: "bogus"}}},
		{"full only", []Artifact{art(ArtifactFull, 100, "")}},
	}
	for _, c := range cases {
		m := validManifest()
		m.Artifacts = c.arts
		got, err := SelectArtifact("0.2.0", m)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.Type != ArtifactFull {
			t.Errorf("%s: expected full fallback, got %s", c.name, got.Type)
		}
	}
}

func TestNoFullFails(t *testing.T) {
	m := validManifest()
	m.Artifacts = []Artifact{art(ArtifactDelta, 10, "0.2.0")}
	if _, err := SelectArtifact("0.2.0", m); err == nil {
		t.Error("manifest without full artifact must fail")
	}
}

func TestEventsDeliveredInOrder(t *testing.T) {
	// Regression: events must arrive in emission order. A stale
	// snapshot delivered late once overwrote a newer state in the UI.
	m := &Manager{status: Status{State: StateIdle}}
	var got []string
	m.OnEvent = func(s Status) { got = append(got, s.State) }
	for _, to := range []string{StateChecking, StateUpdateAvailable, StateDownloading, StateVerifying} {
		m.setState(to, "")
	}
	want := []string{StateChecking, StateUpdateAvailable, StateDownloading, StateVerifying}
	if len(got) != len(want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v, want %v", got, want)
		}
	}
}

func TestStateTransitions(t *testing.T) {
	legal := [][2]string{
		{StateIdle, StateChecking},
		{StateChecking, StateUpdateAvailable},
		{StateUpdateAvailable, StateDownloading},
		{StateUpdateAvailable, StateStaging},
		{StateDownloading, StateVerifying},
		{StateVerifying, StateUpdateAvailable},
		{StateVerifying, StateStaging},
		{StateStaging, StateInstalling},
		{StateInstalling, StateRestarting},
		{StateInstalling, StateRolledBack},
		{StateRestarting, StateUpdated},
		{StateFailed, StateChecking},
	}
	for _, l := range legal {
		if err := Transition(l[0], l[1]); err != nil {
			t.Errorf("legal %s→%s rejected: %v", l[0], l[1], err)
		}
	}
	illegal := [][2]string{
		{StateIdle, StateDownloading},
		{StateIdle, StateInstalling},
		{StateNoUpdate, StateDownloading},
		{StateDownloading, StateInstalling},
		{StateUpdated, StateDownloading},
		{StateUpdateAvailable, StateInstalling},
	}
	for _, l := range illegal {
		if err := Transition(l[0], l[1]); err == nil {
			t.Errorf("impossible %s→%s accepted", l[0], l[1])
		}
	}
}
