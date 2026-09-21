// Package selector implements Auto server selection.
//
// The design follows Hiddify's observed behavior (delay-based selection
// with stability preference and no flapping on millisecond differences),
// reimplemented from scratch: scores combine latency, health and
// stability, and both initial selection and failover apply a hysteresis
// margin so the client does not hop between nearly-equal servers.
// See REFERENCE_IMPLEMENTATION_NOTES.md.
package selector

import (
	"math"
	"sort"

	"connective/backend/internal/servers"
)

// Params tunes the scorer. Zero values are replaced by Defaults().
type Params struct {
	// MaxLatencyMs is the latency at which the latency component hits 0.
	MaxLatencyMs int64
	// Weights for the three components; normalized automatically.
	LatencyWeight   float64
	HealthWeight    float64
	StabilityWeight float64
	// Hysteresis is the fractional bonus granted to the incumbent server
	// (and the required improvement margin for a switch), e.g. 0.15.
	Hysteresis float64
	// UntestedScore is the score of a never-tested server: below any good
	// tested server, above known-bad ones.
	UntestedScore float64
	// UnhealthyCap is the maximum score of a tested server with
	// HealthUnhealthy: known-bad always ranks below unknown.
	UnhealthyCap float64
}

// Defaults returns sane parameters.
func Defaults() Params {
	return Params{
		MaxLatencyMs:    3000,
		LatencyWeight:   0.55,
		HealthWeight:    0.30,
		StabilityWeight: 0.15,
		Hysteresis:      0.15,
		UntestedScore:   35,
		UnhealthyCap:    15,
	}
}

// Score rates a server 0..100 (higher is better).
// samples holds recent latency samples for the stability component; it
// may be empty. incumbent grants the hysteresis bonus.
func Score(s *servers.Server, samples []int64, incumbent bool, p Params) float64 {
	if p.MaxLatencyMs <= 0 {
		p = Defaults()
	}
	var score float64
	switch {
	case s.Health == servers.HealthUnhealthy:
		// Known-bad always ranks below unknown, whether the failure
		// cleared the latency (-1) or not. Never conflate "last test
		// failed" with "never tested".
		score = p.UnhealthyCap
	case !s.Tested():
		score = p.UntestedScore
	default:
		lat := latencyComponent(s.LatencyMs, p.MaxLatencyMs)
		health := healthComponent(s.Health)
		stab := stabilityComponent(samples)
		total := p.LatencyWeight + p.HealthWeight + p.StabilityWeight
		score = (p.LatencyWeight*lat + p.HealthWeight*health + p.StabilityWeight*stab) / total
	}
	if incumbent {
		score *= 1 + p.Hysteresis
	}
	return math.Min(score, 100)
}

func latencyComponent(latencyMs, maxMs int64) float64 {
	if latencyMs < 0 {
		return 0
	}
	if latencyMs >= maxMs {
		return 0
	}
	return 100 * (1 - float64(latencyMs)/float64(maxMs))
}

func healthComponent(h servers.Health) float64 {
	switch h {
	case servers.HealthHealthy:
		return 100
	case servers.HealthDegraded:
		return 50
	case servers.HealthUnhealthy:
		return 0
	default:
		return 60
	}
}

// stabilityComponent rewards consistent samples: 100 for zero spread,
// decaying as the coefficient of variation grows.
func stabilityComponent(samples []int64) float64 {
	if len(samples) < 2 {
		return 60 // neutral when there is no history
	}
	var sum float64
	for _, v := range samples {
		sum += float64(v)
	}
	mean := sum / float64(len(samples))
	if mean <= 0 {
		return 60
	}
	var variance float64
	for _, v := range samples {
		d := float64(v) - mean
		variance += d * d
	}
	cv := math.Sqrt(variance/float64(len(samples))) / mean
	return 100 / (1 + 3*cv)
}

// Ranked is a server with its computed score.
type Ranked struct {
	Server *servers.Server
	Score  float64
}

// Rank orders usable servers best-first. Known-unhealthy servers are
// excluded when any usable alternative exists; if every server is
// unhealthy the list is returned anyway (a sick server may still work,
// and the caller decides whether to attempt it).
func Rank(list []*servers.Server, histories map[string][]int64, incumbentID string, p Params) []Ranked {
	out := make([]Ranked, 0, len(list))
	for _, s := range list {
		out = append(out, Ranked{
			Server: s,
			Score:  Score(s, histories[s.ID], s.ID == incumbentID, p),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	var usable []Ranked
	for _, r := range out {
		if r.Server.Health != servers.HealthUnhealthy {
			usable = append(usable, r)
		}
	}
	if len(usable) > 0 {
		return usable
	}
	return out
}

// Select picks the Auto server. incumbentID may be empty.
func Select(list []*servers.Server, histories map[string][]int64, incumbentID string, p Params) *servers.Server {
	ranked := Rank(list, histories, incumbentID, p)
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0].Server
}

// ShouldSwitch reports whether failover should move from current to best.
// A switch requires best to beat current by the hysteresis margin, so a
// few milliseconds of noise never causes a hop.
func ShouldSwitch(current, best float64, p Params) bool {
	if p.Hysteresis <= 0 {
		p = Defaults()
	}
	return best > current*(1+p.Hysteresis)
}
