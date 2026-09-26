// Package settings owns Connective's organized configuration with
// progressive disclosure: a small set of safe basics plus namespaced
// advanced sections. Unknown JSON keys are ignored on load so older
// versions forward-tolerate newer documents.
package settings

import (
	"fmt"

	"connective/backend/internal/apps"
)

// Settings is the full application configuration.
type Settings struct {
	AutoConnect   bool   `json:"autoConnect"`
	AutoMode      bool   `json:"autoMode"` // true = AUTO, false = manual server
	RoutingMode   string `json:"routingMode"`
	DNSMode       string `json:"dnsMode"`
	TunEnabled    bool   `json:"tunEnabled"`
	KillSwitch    bool   `json:"killSwitch"`
	MixedPort     int    `json:"mixedPort"`
	TestTimeoutMs int    `json:"testTimeoutMs"`
	UpdateOnStart bool   `json:"updateOnStart"`
	Theme         string `json:"theme"`

	// Phase 2: values adopted from reference verification
	// (docs/PHASE2_VERIFICATION.md).
	URLTestIntervalMin int    `json:"urlTestIntervalMin"` // periodic core url-test cadence; 0 disables
	ConnectionTestURL  string `json:"connectionTestUrl"`  // health probe fetched through the proxy
	ClashAPIPort       int    `json:"clashApiPort"`       // sing-box experimental clash-api controller
	CorePath           string `json:"corePath"`           // sing-box binary; empty = PATH lookup
	HelperPath         string `json:"helperPath"`         // connective-helper; empty = next to daemon
	MTU                int    `json:"mtu"`
	TestConcurrency    int    `json:"testConcurrency"`
	// HealthIntervalSec is the active-path probe cadence (default 30).
	HealthIntervalSec int `json:"healthIntervalSec"`
	// UpdateChannel selects the update stream (stable/beta/dev).
	UpdateChannel string `json:"updateChannel"`
	// UpdateAutoCheck enables the periodic background update check.
	UpdateAutoCheck bool `json:"updateAutoCheck"`

	// Split-tunnel (per-app routing). SplitMode is one of "off"
	// (everything via VPN), "bypass" (listed apps go direct), or "only"
	// (only listed apps use the VPN). SplitApps holds normalized
	// executable basenames (see internal/apps); applied on next connect.
	SplitMode string   `json:"splitMode"`
	SplitApps []string `json:"splitApps"`
}

// Split tunnel modes.
const (
	SplitOff    = "off"
	SplitBypass = "bypass"
	SplitOnly   = "only"
)

// MaxSplitApps caps the per-app list so a hostile document cannot bloat
// the generated core config.
const MaxSplitApps = 200

// Defaults returns first-run settings: AUTO mode, global routing,
// proxy-aware DNS, TUN on (Linux primary target).
func Defaults() Settings {
	return Settings{
		AutoMode:      true,
		RoutingMode:   "global",
		DNSMode:       "proxy-aware",
		TunEnabled:    true,
		MixedPort:     10808,
		TestTimeoutMs: 5000,
		UpdateOnStart: true,
		Theme:         "dark",

		URLTestIntervalMin: 10,
		ConnectionTestURL:  "http://connectivitycheck.gstatic.com/generate_204",
		ClashAPIPort:       16756,
		MTU:                9000,
		TestConcurrency:    5,
		HealthIntervalSec:  30,
		UpdateChannel:      "stable",
		UpdateAutoCheck:    true,
		SplitMode:          SplitOff,
		SplitApps:          []string{},
	}
}

// Conflict reports settings combinations the daemon must reject instead
// of silently normalizing: a kill switch blocks all non-tunnel egress,
// so split-tunneled (direct) apps could never reach the network. The
// settings.update handler checks this BEFORE Validate (which resolves
// legacy combinations on load).
func (s Settings) Conflict() error {
	if s.KillSwitch && s.SplitMode != "" && s.SplitMode != SplitOff {
		return fmt.Errorf("kill switch blocks non-tunnel egress and conflicts with split tunneling; turn off split tunneling or the kill switch first")
	}
	return nil
}

// Validate fixes out-of-range values in place.
func (s *Settings) Validate() {
	if s.MixedPort < 0 || s.MixedPort > 65535 {
		s.MixedPort = 10808
	}
	if s.TestTimeoutMs < 500 || s.TestTimeoutMs > 60000 {
		s.TestTimeoutMs = 5000
	}
	switch s.RoutingMode {
	case "global", "rules":
	default:
		s.RoutingMode = "global"
	}
	switch s.DNSMode {
	case "system", "custom", "proxy-aware":
	default:
		s.DNSMode = "proxy-aware"
	}
	if s.URLTestIntervalMin < 0 || s.URLTestIntervalMin > 24*60 {
		s.URLTestIntervalMin = 10
	}
	if s.ConnectionTestURL == "" {
		// Dual-stack 204 endpoint: reachable from IPv4-only and
		// IPv6-capable networks alike (captive.apple.com is
		// IPv6-only in some regions and misdiagnoses v4 hosts).
		s.ConnectionTestURL = "http://connectivitycheck.gstatic.com/generate_204"
	}
	if s.ClashAPIPort < 0 || s.ClashAPIPort > 65535 {
		s.ClashAPIPort = 16756
	}
	if s.MTU < 1280 || s.MTU > 9000 {
		s.MTU = 9000
	}
	if s.TestConcurrency < 1 || s.TestConcurrency > 50 {
		s.TestConcurrency = 5
	}
	if s.HealthIntervalSec < 5 || s.HealthIntervalSec > 600 {
		s.HealthIntervalSec = 30
	}
	switch s.UpdateChannel {
	case "stable", "beta", "dev":
	default:
		s.UpdateChannel = "stable"
	}
	switch s.SplitMode {
	case SplitOff, SplitBypass, SplitOnly:
	default:
		s.SplitMode = SplitOff
	}
	// Auto-resolve legacy/foreign combinations (e.g. an older document
	// written before the conflict existed): split tunneling loses, the
	// kill switch is the stronger guarantee.
	if s.KillSwitch && s.SplitMode != SplitOff {
		s.SplitMode = SplitOff
	}
	s.SplitApps = apps.NormalizeList(s.SplitApps, MaxSplitApps)
}
