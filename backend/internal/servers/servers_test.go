package servers

import (
	"strings"
	"testing"
)

func TestParseVLESS(t *testing.T) {
	link := "vless://11111111-2222-4333-8444-555555555555@example.com:443" +
		"?encryption=none&security=tls&sni=example.com&fp=chrome&type=ws&path=%2Fws&host=example.com#Germany%205"
	s, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Protocol != ProtocolVLESS || s.Address != "example.com" || s.Port != 443 {
		t.Fatalf("wrong endpoint: %+v", s)
	}
	if s.Transport != TransportWS || s.Security != SecurityTLS {
		t.Fatalf("wrong transport/security: %+v", s)
	}
	if s.SNI != "example.com" || s.Fingerprint != "chrome" || s.Path != "/ws" {
		t.Fatalf("wrong tls fields: %+v", s)
	}
	if s.Name != "Germany 5" {
		t.Fatalf("wrong name: %q", s.Name)
	}
	if s.LatencyMs != -1 || s.Health != HealthUnknown {
		t.Fatalf("fresh server should be untested: %+v", s)
	}
}

func TestParseVLESSReality(t *testing.T) {
	link := "vless://11111111-2222-4333-8444-555555555555@198.51.100.7:443" +
		"?encryption=none&security=reality&sni=cdn.example.net&fp=firefox&pbk=PUBKEY&sid=abcd&flow=xtls-rprx-vision#R1"
	s, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Security != SecurityReality || s.PublicKey != "PUBKEY" || s.ShortID != "abcd" {
		t.Fatalf("wrong reality fields: %+v", s)
	}
	if s.Flow != "xtls-rprx-vision" {
		t.Fatalf("wrong flow: %+v", s)
	}
}

func TestParseVMess(t *testing.T) {
	// vmess://base64({"v":"2","ps":"VM","add":"203.0.113.9","port":"8388",
	//   "id":"11111111-2222-4333-8444-555555555555","net":"ws","path":"/p","tls":"tls"})
	link := "vmess://eyJ2IjoiMiIsInBzIjoiVk0iLCJhZGQiOiIyMDMuMC4xMTMuOSIsInBvcnQiOiI4Mzg4IiwiaWQiOiIxMTExMTExMS0yMjIyLTQzMzMtODQ0NC01NTU1NTU1NTU1NTUiLCJuZXQiOiJ3cyIsInBhdGgiOiIvcCIsInRscyI6InRscyJ9"
	s, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Protocol != ProtocolVMess || s.Transport != TransportWS || s.Security != SecurityTLS {
		t.Fatalf("wrong fields: %+v", s)
	}
	if s.Port != 8388 || s.Path != "/p" || s.Name != "VM" {
		t.Fatalf("wrong fields: %+v", s)
	}
}

func TestParseTrojan(t *testing.T) {
	link := "trojan://secret-pw@trojan.example.org:443?sni=trojan.example.org#Trojan%201"
	s, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Protocol != ProtocolTrojan || s.Password != "secret-pw" {
		t.Fatalf("wrong fields: %+v", s)
	}
	if s.Security != SecurityTLS {
		t.Fatalf("trojan must default to tls, got %q", s.Security)
	}
}

func TestParseShadowsocks(t *testing.T) {
	// ss://base64(aes-128-gcm:testpw)@192.0.2.1:8388#SS1
	link := "ss://YWVzLTEyOC1nY206dGVzdHB3@192.0.2.1:8388#SS1"
	s, err := ParseShareLink(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Protocol != ProtocolShadowsocks || s.Method != "aes-128-gcm" || s.Password != "testpw" {
		t.Fatalf("wrong fields: %+v", s)
	}
	if s.Address != "192.0.2.1" || s.Port != 8388 || s.Name != "SS1" {
		t.Fatalf("wrong fields: %+v", s)
	}
}

func TestParseRejects(t *testing.T) {
	for _, link := range []string{
		"",
		"http://example.com/sub",
		"vless://not-a-uuid@example.com:443",
		"vless://11111111-2222-4333-8444-555555555555@example.com:99999",
		"trojan://@example.com:443",
		"ss://@@@",
	} {
		if _, err := ParseShareLink(link); err == nil {
			t.Errorf("expected error for %q", link)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	links := []string{
		"vless://11111111-2222-4333-8444-555555555555@example.com:443?encryption=none&security=tls&sni=example.com&fp=chrome&type=ws&path=%2Fws#A",
		"trojan://pw@trojan.example.org:443?sni=trojan.example.org#B",
		"ss://YWVzLTEyOC1nY206dGVzdHB3@192.0.2.1:8388#C",
	}
	for _, link := range links {
		s, err := ParseShareLink(link)
		if err != nil {
			t.Fatalf("parse %q: %v", link, err)
		}
		out, err := ExportShareLink(s)
		if err != nil {
			t.Fatalf("export %q: %v", link, err)
		}
		s2, err := ParseShareLink(out)
		if err != nil {
			t.Fatalf("re-parse %q (from %q): %v", out, link, err)
		}
		if s2.Address != s.Address || s2.Port != s.Port || s2.Protocol != s.Protocol ||
			s2.Name != s.Name || !strings.EqualFold(string(s2.Security), string(s.Security)) {
			t.Errorf("round trip mismatch:\n in: %q\nout: %q\n%+v vs %+v", link, out, s, s2)
		}
	}
}

func TestValidate(t *testing.T) {
	s := &Server{Name: "x", Address: "h", Port: 1, Protocol: ProtocolVLESS}
	s.Normalize()
	if err := s.Validate(); err != nil {
		t.Fatalf("valid server rejected: %v", err)
	}
	bad := *s
	bad.Protocol = "wireguerd"
	if err := bad.Validate(); err == nil {
		t.Errorf("expected protocol rejection")
	}
}
