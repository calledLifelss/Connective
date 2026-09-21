package servers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseShareLink parses a single share-link / URI into a Server.
// Supported schemes: vless://, vmess://, trojan://, ss://.
//
// The accepted shapes mirror what v2rayN's Fmt handlers and Hiddify's
// importers accept (behavior studied from source, reimplemented here):
//
//	vless://uuid@host:port?params#name
//	vmess://base64-json
//	trojan://password@host:port?params#name
//	ss://base64(method:password)@host:port#name  (also ...@... with inline info)
func ParseShareLink(link string) (*Server, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return nil, fmt.Errorf("empty share link")
	}
	scheme := schemeOf(link)
	switch strings.ToLower(scheme) {
	case "vless":
		return parseVLESS(link)
	case "vmess":
		return parseVMess(link)
	case "trojan":
		return parseTrojan(link)
	case "ss", "shadowsocks":
		return parseShadowsocks(link)
	default:
		return nil, fmt.Errorf("unsupported share-link scheme %q", scheme)
	}
}

func schemeOf(link string) string {
	if i := strings.Index(link, "://"); i > 0 {
		return link[:i]
	}
	return ""
}

func parseVLESS(link string) (*Server, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid vless link: %w", err)
	}
	s := &Server{Protocol: ProtocolVLESS}
	s.UUID = u.User.Username()
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid vless link: %w", err)
	}
	s.Address, s.Port = host, port
	q := u.Query()
	s.Transport = Transport(q.Get("type"))
	s.Security = Security(q.Get("security"))
	s.SNI = firstNonEmpty(q.Get("sni"), q.Get("serverName"))
	s.Fingerprint = q.Get("fp")
	s.Flow = q.Get("flow")
	s.PublicKey = q.Get("pbk")
	s.ShortID = q.Get("sid")
	s.Path = q.Get("path")
	s.Host = q.Get("host")
	s.ServiceName = q.Get("serviceName")
	s.ALPN = q.Get("alpn")
	s.Name = displayName(u)
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if !looksLikeUUID(s.UUID) {
		return nil, fmt.Errorf("invalid vless link: id is not a uuid")
	}
	return s, nil
}

func parseTrojan(link string) (*Server, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid trojan link: %w", err)
	}
	s := &Server{Protocol: ProtocolTrojan}
	if pw, has := passwordOf(u); has {
		s.Password = pw
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid trojan link: %w", err)
	}
	s.Address, s.Port = host, port
	q := u.Query()
	s.Transport = Transport(q.Get("type"))
	if s.Transport == "" {
		s.Transport = TransportTCP
	}
	s.Security = Security(q.Get("security"))
	if s.Security == "" {
		// Trojan is TLS-by-design; an absent parameter means TLS.
		s.Security = SecurityTLS
	}
	s.SNI = firstNonEmpty(q.Get("sni"), q.Get("peer"))
	s.Fingerprint = q.Get("fp")
	s.Path = q.Get("path")
	s.Host = q.Get("host")
	s.ServiceName = q.Get("serviceName")
	s.ALPN = q.Get("alpn")
	s.Name = displayName(u)
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if s.Password == "" {
		return nil, fmt.Errorf("invalid trojan link: empty password")
	}
	return s, nil
}

// vmessJSON mirrors the JSON payload inside a vmess:// link.
type vmessJSON struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	Aid  any    `json:"aid"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	FP   string `json:"fp"`
}

func parseVMess(link string) (*Server, error) {
	raw := strings.TrimPrefix(link, "vmess://")
	raw = strings.TrimSpace(raw)
	decoded, err := decodeBase64URL(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid vmess link: %w", err)
	}
	var v vmessJSON
	if err := json.Unmarshal(decoded, &v); err != nil {
		return nil, fmt.Errorf("invalid vmess link: %w", err)
	}
	s := &Server{Protocol: ProtocolVMess}
	s.Name = v.PS
	s.Address = v.Add
	port, err := anyPort(v.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid vmess link: %w", err)
	}
	s.Port = port
	s.UUID = v.ID
	s.Transport = Transport(v.Net)
	s.Path = v.Path
	s.Host = v.Host
	s.SNI = v.SNI
	s.Fingerprint = v.FP
	switch strings.ToLower(v.TLS) {
	case "tls":
		s.Security = SecurityTLS
	default:
		s.Security = SecurityNone
	}
	if s.Name == "" {
		s.Name = s.Address
	}
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if !looksLikeUUID(s.UUID) {
		return nil, fmt.Errorf("invalid vmess link: id is not a uuid")
	}
	return s, nil
}

func parseShadowsocks(link string) (*Server, error) {
	rest := strings.TrimPrefix(strings.TrimPrefix(link, "ss://"), "shadowsocks://")
	name := ""
	if i := strings.Index(rest, "#"); i >= 0 {
		name, rest = rest[i+1:], rest[:i]
	}
	// Two shapes: ss://BASE64@host:port  or  ss://BASE64(method:pass@host:port)
	var method, password, hostport string
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		userPart, hostPart := rest[:i], rest[i+1:]
		// userPart may itself be a URL with query (plugin opts); strip query.
		if qi := strings.Index(userPart, "?"); qi >= 0 {
			userPart = userPart[:qi]
		}
		if decoded, err := decodeBase64URL(userPart); err == nil && strings.Contains(string(decoded), ":") {
			rest = string(decoded) + "@" + hostPart
			return parseShadowsocks("ss://" + rest + "#" + name)
		}
		// Non-encoded method:password form.
		if decoded, err := decodeBase64URL(userPart); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				method, password, hostport = parts[0], parts[1], hostPart
			}
		} else {
			parts := strings.SplitN(userPart, ":", 2)
			if len(parts) == 2 {
				method, password, hostport = parts[0], parts[1], hostPart
			}
		}
	} else {
		// Whole thing is base64 of method:password@host:port.
		decoded, err := decodeBase64URL(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid ss link: %w", err)
		}
		return parseShadowsocks("ss://" + string(decoded) + "#" + name)
	}
	if method == "" || hostport == "" {
		return nil, fmt.Errorf("invalid ss link: cannot determine method/host")
	}
	// hostport may carry a /?plugin= suffix.
	if i := strings.Index(hostport, "/"); i >= 0 {
		hostport = hostport[:i]
	}
	if i := strings.Index(hostport, "?"); i >= 0 {
		hostport = hostport[:i]
	}
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, fmt.Errorf("invalid ss link: %w", err)
	}
	s := &Server{
		Protocol: ProtocolShadowsocks,
		Method:   method,
		Password: password,
		Address:  host,
		Port:     port,
		Name:     name,
	}
	if s.Name == "" {
		s.Name = host
	} else if decoded, err := url.PathUnescape(name); err == nil {
		s.Name = decoded
	}
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// --- helpers ---

func splitHostPort(hostport string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		// Tolerate links without an explicit port separator edge cases by
		// reporting the error plainly; ports are mandatory in share links.
		return "", 0, fmt.Errorf("bad host:port %q", hostport)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("bad port %q", portStr)
	}
	return host, port, nil
}

func displayName(u *url.URL) string {
	if u.Fragment != "" {
		if name, err := url.PathUnescape(u.Fragment); err == nil {
			return name
		}
		return u.Fragment
	}
	return u.Hostname()
}

func passwordOf(u *url.URL) (string, bool) {
	if u.User == nil {
		return "", false
	}
	if pw, ok := u.User.Password(); ok {
		if unesc, err := url.PathUnescape(pw); err == nil {
			return unesc, true
		}
		return pw, true
	}
	// trojan://password@host form: username carries the password.
	username := u.User.Username()
	if unesc, err := url.PathUnescape(username); err == nil {
		return unesc, unesc != ""
	}
	return username, username != ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func decodeBase64URL(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.URLEncoding.DecodeString(padBase64(s)); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(padBase64(s))
}

func padBase64(s string) string {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return s
}

func anyPort(v any) (int, error) {
	switch p := v.(type) {
	case float64:
		return int(p), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(p))
	case json.Number:
		n, err := p.Int64()
		return int(n), err
	default:
		return 0, fmt.Errorf("bad port value %v", v)
	}
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
