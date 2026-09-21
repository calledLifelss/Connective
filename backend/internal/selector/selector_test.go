package selector

import (
	"testing"

	"connective/backend/internal/servers"
)

func mkServer(id string, latency int64, h servers.Health) *servers.Server {
	return &servers.Server{ID: id, Name: id, Address: "10.0.0.1", Port: 443,
		Protocol: servers.ProtocolVLESS, LatencyMs: latency, Health: h}
}

func TestLatencyOrdering(t *testing.T) {
	p := Defaults()
	fast := mkServer("fast", 80, servers.HealthHealthy)
	slow := mkServer("slow", 1200, servers.HealthHealthy)
	if Score(fast, nil, false, p) <= Score(slow, nil, false, p) {
		t.Errorf("fast should outscore slow: %v vs %v",
			Score(fast, nil, false, p), Score(slow, nil, false, p))
	}
}

func TestUntestedBetweenGoodAndBad(t *testing.T) {
	p := Defaults()
	good := mkServer("good", 80, servers.HealthHealthy)
	untested := mkServer("new", -1, servers.HealthUnknown)
	bad := mkServer("bad", 500, servers.HealthUnhealthy)
	sg, su, sb := Score(good, nil, false, p), Score(untested, nil, false, p), Score(bad, nil, false, p)
	if !(sg > su && su > sb) {
		t.Errorf("expected good > untested > bad, got %v %v %v", sg, su, sb)
	}
}

func TestFailedNeverOutranksUnknown(t *testing.T) {
	p := Defaults()
	// A failed test clears latency to -1; it must still score as
	// known-bad, never as "untested".
	failed := mkServer("failed", -1, servers.HealthUnhealthy)
	never := mkServer("never", -1, servers.HealthUnknown)
	if Score(failed, nil, false, p) >= Score(never, nil, false, p) {
		t.Errorf("failed (%v) must score below never-tested (%v)",
			Score(failed, nil, false, p), Score(never, nil, false, p))
	}
	// ... even with the incumbent bonus.
	if Score(failed, nil, true, p) >= Score(never, nil, false, p) {
		t.Errorf("incumbent bonus must not rescue a known-bad server")
	}
}

func TestHealthBeatsSlightlyBetterPing(t *testing.T) {
	p := Defaults()
	healthy := mkServer("h", 300, servers.HealthHealthy)
	sick := mkServer("s", 100, servers.HealthUnhealthy)
	if Score(healthy, nil, false, p) <= Score(sick, nil, false, p) {
		t.Errorf("healthy should beat unhealthy despite ping")
	}
}

func TestStabilityMatters(t *testing.T) {
	p := Defaults()
	a := mkServer("a", 200, servers.HealthHealthy)
	b := mkServer("b", 200, servers.HealthHealthy)
	steady := []int64{195, 200, 205}
	jittery := []int64{50, 400, 900}
	if Score(a, steady, false, p) <= Score(b, jittery, false, p) {
		t.Errorf("steady should beat jittery")
	}
}

func TestHysteresisPreventsFlap(t *testing.T) {
	p := Defaults()
	current, challenger := 70.0, 72.0 // ~3% better: noise, not a reason
	if ShouldSwitch(current, challenger, p) {
		t.Errorf("3%% improvement should not trigger a switch")
	}
	if !ShouldSwitch(current, 90.0, p) {
		t.Errorf("large improvement should trigger a switch")
	}
}

func TestRankExcludesUnhealthy(t *testing.T) {
	p := Defaults()
	list := []*servers.Server{
		mkServer("sick", 50, servers.HealthUnhealthy),
		mkServer("ok", 400, servers.HealthHealthy),
	}
	ranked := Rank(list, nil, "", p)
	if len(ranked) != 1 || ranked[0].Server.ID != "ok" {
		t.Fatalf("expected only ok, got %+v", ranked)
	}
}

func TestSelectEmpty(t *testing.T) {
	if Select(nil, nil, "", Defaults()) != nil {
		t.Errorf("expected nil for empty list")
	}
}

func TestIncumbentBonus(t *testing.T) {
	p := Defaults()
	a := mkServer("a", 200, servers.HealthHealthy)
	b := mkServer("b", 200, servers.HealthHealthy)
	sel := Select([]*servers.Server{a, b}, nil, "b", p)
	if sel.ID != "b" {
		t.Errorf("tied incumbent should win, got %q", sel.ID)
	}
}
