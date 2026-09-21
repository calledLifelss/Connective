// Package configgen renders canonical sing-box configurations from
// Connective server records.
//
// sing-box is Connective's primary core (see ARCHITECTURE.md): a single
// mature core that owns proxying, TUN, routing and DNS in one process,
// which is exactly the combination Hiddify ships on desktop and v2rayN
// supports as its modern backend. The JSON schema below follows the
// public sing-box configuration format; field names are facts of the
// external format, not copied code.
package configgen

import (
	"encoding/json"
	"fmt"
	"time"

	"connective/backend/internal/servers"
)

// Mode selects the routing posture.
type Mode string

const (
	// ModeGlobal sends (almost) everything through the proxy.
	ModeGlobal Mode = "global"
	// ModeRules bypasses LAN/private addresses and routes the rest by rule.
	ModeRules Mode = "rules"
)

// Options tunes generation.
type Options struct {
	MixedPort  int  // localhost SOCKS5+HTTP inbound; 0 disables
	TunEnabled bool // include a TUN inbound (requires privilege at runtime)
	Mode       Mode
	LogLevel   string

	// ClashPort enables experimental.clash_api on 127.0.0.1 (0 disables).
	// The stats subsystem polls it for real traffic data.
	ClashPort   int
	ClashSecret string
	// URLTestURL is the probe fetched by urltest groups.
	URLTestURL string
	// URLTestInterval is the urltest re-check cadence (0 = core default).
	URLTestInterval time.Duration
	// MTU for the TUN inbound (0 = 9000).
	MTU int
	// ExcludeAddrs are IPs kept out of the TUN (route_exclude_address).
	// The daemon fills this with the VPN server endpoints: without it
	// the core's own server-bound sockets re-enter its TUN and loop
	// forever (observed as hung handshakes in the netns e2e).
	ExcludeAddrs []string
}

// DefaultOptions returns standard desktop options.
func DefaultOptions() Options {
	return Options{MixedPort: 10808, Mode: ModeGlobal, LogLevel: "info"}
}

// Generate renders a complete sing-box config for one active server.
func Generate(s *servers.Server, opts Options) ([]byte, error) {
	outbound, err := outboundFor(s, "proxy")
	if err != nil {
		return nil, err
	}
	cfg := baseConfig(opts)
	cfg["outbounds"] = append([]any{outbound}, systemOutbounds()...)
	cfg["route"] = routeConfig(opts.Mode, "proxy")
	return json.MarshalIndent(cfg, "", "  ")
}

// Group selection tags. The selector named ProxyTag is the routing
// target; ProxyAutoTag is the core-native urltest group (Hiddify/v2rayN
// parity: auto-pick inside the core, manual override via selector).
const (
	ProxyTag     = "proxy"
	ProxyAutoTag = "proxy-auto"
)

// GenerateGroup renders a multi-server config with core-native auto
// selection: one outbound per server, a urltest group over all of them,
// and a selector whose default is the urltest group (AUTO) unless
// defaultTag names a specific server outbound (manual mode).
func GenerateGroup(list []*servers.Server, defaultTag string, opts Options) ([]byte, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("no servers for group config")
	}
	var outbounds []any
	var tags []string
	tagOf := map[string]string{}
	for i, s := range list {
		tag := fmt.Sprintf("proxy-%d", i)
		ob, err := outboundFor(s, tag)
		if err != nil {
			return nil, fmt.Errorf("server %q: %w", s.DisplayName(), err)
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, tag)
		tagOf[s.ID] = tag
	}
	urltest := map[string]any{
		"type":                        "urltest",
		"tag":                         ProxyAutoTag,
		"outbounds":                   tags,
		"url":                         firstNonEmpty(opts.URLTestURL, "https://www.gstatic.com/generate_204"),
		"interrupt_exist_connections": false,
		// Tolerance is a millisecond number (uint16): the challenger must
		// beat the incumbent by this margin to take over — anti-flap.
		"tolerance": 1000,
	}
	if opts.URLTestInterval > 0 {
		urltest["interval"] = opts.URLTestInterval.String()
	}
	selectorList := append([]any{ProxyAutoTag}, toAny(tags)...)
	selector := map[string]any{
		"type":                        "selector",
		"tag":                         ProxyTag,
		"outbounds":                   selectorList,
		"interrupt_exist_connections": false,
	}
	if tag, ok := tagOf[defaultTag]; ok {
		selector["default"] = tag
	} else {
		selector["default"] = ProxyAutoTag
	}
	outbounds = append(outbounds, urltest, selector)
	outbounds = append(outbounds, systemOutbounds()...)
	cfg := baseConfig(opts)
	cfg["outbounds"] = outbounds
	cfg["route"] = routeConfig(opts.Mode, ProxyTag)
	return json.MarshalIndent(cfg, "", "  ")
}

func toAny(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

func systemOutbounds() []any {
	// NOTE: the "dns" outbound type was removed in sing-box 1.13;
	// DNS interception uses the hijack-dns rule action instead.
	return []any{
		map[string]any{"type": "direct", "tag": "direct"},
		map[string]any{"type": "block", "tag": "block"},
	}
}

func baseConfig(opts Options) map[string]any {
	cfg := map[string]any{
		"log":      map[string]any{"level": firstNonEmpty(opts.LogLevel, "info")},
		"dns":      dnsConfig(),
		"inbounds": inbounds(opts),
	}
	if opts.ClashPort > 0 {
		cfg["experimental"] = map[string]any{
			"clash_api": map[string]any{
				"external_controller": fmt.Sprintf("127.0.0.1:%d", opts.ClashPort),
				"secret":              opts.ClashSecret,
			},
		}
	}
	return cfg
}

func outboundFor(s *servers.Server, tag string) (map[string]any, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	base := map[string]any{
		"tag":         tag,
		"server":      s.Address,
		"server_port": s.Port,
	}
	switch s.Protocol {
	case servers.ProtocolVLESS:
		if s.UUID == "" {
			return nil, fmt.Errorf("vless needs a uuid")
		}
		base["type"] = "vless"
		base["uuid"] = s.UUID
		if s.Flow != "" {
			base["flow"] = s.Flow
		}
		base["packet_encoding"] = "xudp"
	case servers.ProtocolVMess:
		if s.UUID == "" {
			return nil, fmt.Errorf("vmess needs a uuid")
		}
		base["type"] = "vmess"
		base["uuid"] = s.UUID
		base["security"] = "auto"
		base["packet_encoding"] = "xudp"
	case servers.ProtocolTrojan:
		if s.Password == "" {
			return nil, fmt.Errorf("trojan needs a password")
		}
		base["type"] = "trojan"
		base["password"] = s.Password
	case servers.ProtocolShadowsocks:
		if s.Method == "" || s.Password == "" {
			return nil, fmt.Errorf("shadowsocks needs method and password")
		}
		base["type"] = "shadowsocks"
		base["method"] = s.Method
		base["password"] = s.Password
	default:
		return nil, fmt.Errorf("config generation for %q not implemented yet", s.Protocol)
	}
	if tls, ok := tlsConfig(s); ok {
		base["tls"] = tls
	}
	if tr, ok := transportConfig(s); ok {
		base["transport"] = tr
	}
	return base, nil
}

// tlsConfig builds the sing-box TLS object. Reality is TLS + reality block.
func tlsConfig(s *servers.Server) (map[string]any, bool) {
	if s.Security != servers.SecurityTLS && s.Security != servers.SecurityReality {
		return nil, false
	}
	t := map[string]any{
		"enabled":     true,
		"server_name": firstNonEmpty(s.SNI, s.Address),
	}
	if s.ALPN != "" {
		t["alpn"] = []string{s.ALPN}
	}
	if s.Fingerprint != "" {
		t["utls"] = map[string]any{"enabled": true, "fingerprint": s.Fingerprint}
	}
	if s.Security == servers.SecurityReality {
		t["reality"] = map[string]any{
			"enabled":    true,
			"public_key": s.PublicKey,
			"short_id":   s.ShortID,
		}
	}
	return t, true
}

// transportConfig builds the sing-box transport object (nil for plain TCP).
func transportConfig(s *servers.Server) (map[string]any, bool) {
	switch s.Transport {
	case servers.TransportWS:
		t := map[string]any{"type": "ws", "path": firstNonEmpty(s.Path, "/")}
		if s.Host != "" {
			t["headers"] = map[string]any{"Host": s.Host}
		}
		return t, true
	case servers.TransportGRPC:
		return map[string]any{
			"type":         "grpc",
			"service_name": firstNonEmpty(s.ServiceName, "grpc"),
		}, true
	case servers.TransportH2:
		return map[string]any{"type": "http", "path": firstNonEmpty(s.Path, "/")}, true
	default:
		return nil, false
	}
}

func inbounds(opts Options) []any {
	var in []any
	if opts.MixedPort > 0 {
		in = append(in, map[string]any{
			"type": "mixed", "tag": "mixed-in",
			"listen": "127.0.0.1", "listen_port": opts.MixedPort,
		})
	}
	if opts.TunEnabled {
		mtu := opts.MTU
		if mtu <= 0 {
			mtu = 9000
		}
		tun := map[string]any{
			"type": "tun", "tag": "tun-in",
			"interface_name": "connective0",
			"mtu":            mtu,
			"auto_route":     true,
			"strict_route":   true,
			"stack":          "system",
			"address":        []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
		}
		if len(opts.ExcludeAddrs) > 0 {
			tun["route_exclude_address"] = opts.ExcludeAddrs
		}
		in = append(in, tun)
	}
	return in
}

func dnsConfig() map[string]any {
	// New-style DNS server objects (sing-box >= 1.12; legacy string
	// addresses were removed in 1.14 — verified by `sing-box check`).
	return map[string]any{
		"servers": []any{
			map[string]any{"tag": "proxy-dns", "type": "tls", "server": "8.8.8.8", "detour": "proxy"},
			map[string]any{"tag": "local-dns", "type": "local", "detour": "direct"},
		},
		"rules": []any{
			map[string]any{"clash_mode": "direct", "server": "local-dns"},
		},
		"final": "proxy-dns",
	}
}

func routeConfig(mode Mode, final string) map[string]any {
	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
		map[string]any{"ip_is_private": true, "action": "route", "outbound": "direct"},
	}
	if final == "" {
		final = "proxy"
	}
	// Rule mode keeps the same safe defaults; richer geosite/rule-set
	// driven policy lives in the routing package.
	return map[string]any{
		"rules":                   rules,
		"final":                   final,
		"auto_detect_interface":   true,
		"default_domain_resolver": "local-dns",
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
