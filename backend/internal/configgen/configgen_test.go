package configgen

import (
	"encoding/json"
	"testing"

	"connective/backend/internal/servers"
)

func mustParse(t *testing.T, link string) *servers.Server {
	t.Helper()
	s, err := servers.ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse %q: %v", link, err)
	}
	return s
}

func genOutbound(t *testing.T, s *servers.Server, opts Options) map[string]any {
	t.Helper()
	raw, err := Generate(s, opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	for _, key := range []string{"log", "dns", "inbounds", "outbounds", "route"} {
		if _, ok := cfg[key]; !ok {
			t.Fatalf("config missing %q", key)
		}
	}
	outs := cfg["outbounds"].([]any)
	return outs[0].(map[string]any)
}

func TestVLESSWsTLS(t *testing.T) {
	s := mustParse(t, "vless://11111111-2222-4333-8444-555555555555@example.com:443?encryption=none&security=tls&sni=example.com&fp=chrome&type=ws&path=%2Fws&host=example.com#X")
	ob := genOutbound(t, s, DefaultOptions())
	if ob["type"] != "vless" || ob["uuid"] != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("bad outbound: %v", ob)
	}
	tls := ob["tls"].(map[string]any)
	if tls["enabled"] != true || tls["server_name"] != "example.com" {
		t.Fatalf("bad tls: %v", tls)
	}
	tr := ob["transport"].(map[string]any)
	if tr["type"] != "ws" || tr["path"] != "/ws" {
		t.Fatalf("bad transport: %v", tr)
	}
}

func TestVLESSReality(t *testing.T) {
	s := mustParse(t, "vless://11111111-2222-4333-8444-555555555555@198.51.100.7:443?encryption=none&security=reality&sni=cdn.example.net&fp=firefox&pbk=PUBKEY&sid=abcd&flow=xtls-rprx-vision#R")
	ob := genOutbound(t, s, DefaultOptions())
	tls := ob["tls"].(map[string]any)
	reality, ok := tls["reality"].(map[string]any)
	if !ok || reality["public_key"] != "PUBKEY" || reality["short_id"] != "abcd" {
		t.Fatalf("bad reality: %v", tls)
	}
	if ob["flow"] != "xtls-rprx-vision" {
		t.Fatalf("bad flow: %v", ob)
	}
}

func TestVMessGRPC(t *testing.T) {
	s := mustParse(t, "vless://11111111-2222-4333-8444-555555555555@example.com:443?encryption=none&type=grpc&serviceName=svc#G")
	ob := genOutbound(t, s, DefaultOptions())
	if _, hasTLS := ob["tls"]; hasTLS {
		t.Fatalf("plain grpc should have no tls block: %v", ob)
	}
	tr := ob["transport"].(map[string]any)
	if tr["type"] != "grpc" || tr["service_name"] != "svc" {
		t.Fatalf("bad transport: %v", tr)
	}
}

func TestTrojanAndShadowsocks(t *testing.T) {
	tr := genOutbound(t, mustParse(t, "trojan://pw@trojan.example.org:443?sni=trojan.example.org#T"), DefaultOptions())
	if tr["type"] != "trojan" || tr["password"] != "pw" {
		t.Fatalf("bad trojan: %v", tr)
	}
	ss := genOutbound(t, mustParse(t, "ss://YWVzLTEyOC1nY206dGVzdHB3@192.0.2.1:8388#S"), DefaultOptions())
	if ss["type"] != "shadowsocks" || ss["method"] != "aes-128-gcm" {
		t.Fatalf("bad ss: %v", ss)
	}
}

func TestTunExcludes(t *testing.T) {
	opts := DefaultOptions()
	opts.TunEnabled = true
	opts.ExcludeAddrs = []string{"10.200.1.1/32"}
	raw, err := Generate(mustParse(t, "trojan://pw@trojan.example.org:443#T"), opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Inbounds []struct {
			Type     string   `json:"type"`
			Excludes []string `json:"route_exclude_address"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, in := range cfg.Inbounds {
		if in.Type == "tun" {
			if len(in.Excludes) != 1 || in.Excludes[0] != "10.200.1.1/32" {
				t.Fatalf("bad excludes: %v", in.Excludes)
			}
			return
		}
	}
	t.Fatalf("tun inbound missing")
}

func TestTunInbound(t *testing.T) {
	s := mustParse(t, "trojan://pw@trojan.example.org:443#T")
	opts := DefaultOptions()
	opts.TunEnabled = true
	raw, err := Generate(s, opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Inbounds []struct {
			Type string `json:"type"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range cfg.Inbounds {
		if in.Type == "tun" {
			found = true
		}
	}
	if !found {
		t.Errorf("tun inbound missing")
	}
}

func TestInvalidServerRejected(t *testing.T) {
	bad := &servers.Server{Name: "bad"}
	if _, err := Generate(bad, DefaultOptions()); err == nil {
		t.Errorf("expected error for invalid server")
	}
	wireguard := &servers.Server{Name: "w", Address: "h", Port: 1, Protocol: servers.ProtocolWireGuard}
	wireguard.Normalize()
	if _, err := Generate(wireguard, DefaultOptions()); err == nil {
		t.Errorf("expected not-implemented error for wireguard")
	}
}

func groupServers(t *testing.T) []*servers.Server {
	t.Helper()
	return []*servers.Server{
		mustParse(t, "vless://11111111-2222-4333-8444-555555555555@10.0.0.1:443?encryption=none#A"),
		mustParse(t, "trojan://pw@10.0.0.2:443#B"),
	}
}

func TestGroupURLTestSelector(t *testing.T) {
	list := groupServers(t)
	list[0].ID, list[1].ID = "id-a", "id-b"
	raw, err := GenerateGroup(list, "", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []struct {
			Type      string   `json:"type"`
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
			Default   string   `json:"default"`
			Tolerance int      `json:"tolerance"`
			URL       string   `json:"url"`
		} `json:"outbounds"`
		Route struct {
			Final string `json:"final"`
		} `json:"route"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	byTag := map[string]int{}
	for i, o := range cfg.Outbounds {
		byTag[o.Tag] = i
	}
	ut, ok := byTag[ProxyAutoTag]
	if !ok || cfg.Outbounds[ut].Type != "urltest" {
		t.Fatalf("urltest group missing: %+v", cfg.Outbounds)
	}
	if len(cfg.Outbounds[ut].Outbounds) != 2 || cfg.Outbounds[ut].URL == "" {
		t.Errorf("bad urltest: %+v", cfg.Outbounds[ut])
	}
	if cfg.Outbounds[ut].Tolerance != 1000 {
		t.Errorf("urltest tolerance should be 1000ms, got %d", cfg.Outbounds[ut].Tolerance)
	}
	sel, ok := byTag[ProxyTag]
	if !ok || cfg.Outbounds[sel].Type != "selector" {
		t.Fatalf("selector missing")
	}
	got := cfg.Outbounds[sel].Outbounds
	if len(got) != 3 || got[0] != ProxyAutoTag {
		t.Errorf("selector must lead with urltest group: %v", got)
	}
	if cfg.Outbounds[sel].Default != ProxyAutoTag {
		t.Errorf("AUTO default should be urltest, got %q", cfg.Outbounds[sel].Default)
	}
	if cfg.Route.Final != ProxyTag {
		t.Errorf("route final should be selector, got %q", cfg.Route.Final)
	}
}

func TestGroupManualDefault(t *testing.T) {
	list := groupServers(t)
	list[0].ID, list[1].ID = "id-a", "id-b"
	raw, err := GenerateGroup(list, "id-b", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []struct {
			Type    string `json:"type"`
			Tag     string `json:"tag"`
			Default string `json:"default"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, o := range cfg.Outbounds {
		if o.Type == "selector" && o.Default != "proxy-1" {
			t.Errorf("manual default should be proxy-1, got %q", o.Default)
		}
	}
}

func TestGroupClashAPI(t *testing.T) {
	opts := DefaultOptions()
	opts.ClashPort = 16756
	opts.ClashSecret = "s3cret"
	raw, err := GenerateGroup(groupServers(t), "", opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Experimental struct {
			ClashAPI struct {
				Controller string `json:"external_controller"`
				Secret     string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Experimental.ClashAPI.Controller != "127.0.0.1:16756" ||
		cfg.Experimental.ClashAPI.Secret != "s3cret" {
		t.Errorf("bad clash api: %+v", cfg.Experimental)
	}
}

func TestGroupEmpty(t *testing.T) {
	if _, err := GenerateGroup(nil, "", DefaultOptions()); err == nil {
		t.Errorf("expected error for empty group")
	}
}

func routeOf(t *testing.T, opts Options) (string, []map[string]any) {
	t.Helper()
	raw, err := Generate(mustParse(t, "trojan://pw@trojan.example.org:443#T"), opts)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Route struct {
			Final string           `json:"final"`
			Rules []map[string]any `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg.Route.Final, cfg.Route.Rules
}

func TestSplitOffNoProcessRules(t *testing.T) {
	final, rules := routeOf(t, DefaultOptions())
	if final != "proxy" {
		t.Fatalf("final = %q", final)
	}
	for _, r := range rules {
		if _, ok := r["process_name"]; ok {
			t.Fatalf("off mode must not emit process rules: %v", r)
		}
	}
}

func TestSplitBypassRoutesAppsDirect(t *testing.T) {
	opts := DefaultOptions()
	opts.SplitMode = "bypass"
	opts.SplitApps = []string{"firefox", "discord"}
	final, rules := routeOf(t, opts)
	if final != "proxy" {
		t.Fatalf("bypass keeps proxy final, got %q", final)
	}
	if len(rules) == 0 {
		t.Fatal("no rules")
	}
	first := rules[0]
	names, _ := first["process_name"].([]any)
	if len(names) != 2 || first["outbound"] != "direct" {
		t.Fatalf("bad bypass rule: %v", first)
	}
}

func TestSplitOnlyRoutesAppsProxyRestDirect(t *testing.T) {
	opts := DefaultOptions()
	opts.SplitMode = "only"
	opts.SplitApps = []string{"firefox"}
	final, rules := routeOf(t, opts)
	if final != "direct" {
		t.Fatalf("only mode flips final to direct, got %q", final)
	}
	first := rules[0]
	names, _ := first["process_name"].([]any)
	if len(names) != 1 || first["outbound"] != "proxy" {
		t.Fatalf("bad only rule: %v", first)
	}
}

func TestSplitEmptyAppsNoChange(t *testing.T) {
	for _, mode := range []string{"bypass", "only"} {
		opts := DefaultOptions()
		opts.SplitMode = mode
		final, rules := routeOf(t, opts)
		if final != "proxy" {
			t.Fatalf("mode %s with no apps must keep proxy final, got %q", mode, final)
		}
		for _, r := range rules {
			if _, ok := r["process_name"]; ok {
				t.Fatalf("mode %s with no apps must not emit process rules", mode)
			}
		}
	}
}
