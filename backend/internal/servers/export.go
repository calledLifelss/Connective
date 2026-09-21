package servers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ExportShareLink renders a Server back to its canonical share-link form.
// Used for duplicate/copy/export. Only protocols with a stable link
// representation are supported.
func ExportShareLink(s *Server) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	switch s.Protocol {
	case ProtocolVLESS:
		return exportVLESS(s), nil
	case ProtocolVMess:
		return exportVMess(s)
	case ProtocolTrojan:
		return exportTrojan(s), nil
	case ProtocolShadowsocks:
		return exportShadowsocks(s)
	default:
		return "", fmt.Errorf("cannot export protocol %q as share link", s.Protocol)
	}
}

func exportVLESS(s *Server) string {
	u := &url.URL{Scheme: "vless", User: url.User(s.UUID), Host: hostPort(s)}
	q := u.Query()
	if s.Transport != "" {
		q.Set("type", string(s.Transport))
	}
	q.Set("encryption", "none")
	if s.Security != "" && s.Security != SecurityNone {
		q.Set("security", string(s.Security))
	}
	setIf := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	setIf("flow", s.Flow)
	setIf("sni", s.SNI)
	setIf("fp", s.Fingerprint)
	setIf("pbk", s.PublicKey)
	setIf("sid", s.ShortID)
	setIf("path", s.Path)
	setIf("host", s.Host)
	setIf("serviceName", s.ServiceName)
	setIf("alpn", s.ALPN)
	u.RawQuery = q.Encode()
	u.Fragment = s.DisplayName()
	return u.String()
}

func exportTrojan(s *Server) string {
	u := &url.URL{Scheme: "trojan", User: url.User(s.Password), Host: hostPort(s)}
	q := u.Query()
	if s.SNI != "" {
		q.Set("sni", s.SNI)
	}
	if s.Security != "" && s.Security != SecurityNone {
		q.Set("security", string(s.Security))
	}
	if s.Transport != "" && s.Transport != TransportTCP {
		q.Set("type", string(s.Transport))
	}
	if s.Path != "" {
		q.Set("path", s.Path)
	}
	if s.Host != "" {
		q.Set("host", s.Host)
	}
	u.RawQuery = q.Encode()
	u.Fragment = s.DisplayName()
	return u.String()
}

func exportVMess(s *Server) (string, error) {
	payload := map[string]string{
		"v":    "2",
		"ps":   s.DisplayName(),
		"add":  s.Address,
		"port": strconv.Itoa(s.Port),
		"id":   s.UUID,
		"aid":  "0",
		"net":  string(s.Transport),
		"type": "none",
		"host": s.Host,
		"path": s.Path,
		"sni":  s.SNI,
		"fp":   s.Fingerprint,
	}
	if s.Security == SecurityTLS {
		payload["tls"] = "tls"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(raw), nil
}

func exportShadowsocks(s *Server) (string, error) {
	if s.Method == "" || s.Password == "" {
		return "", fmt.Errorf("shadowsocks export needs method and password")
	}
	user := base64.URLEncoding.EncodeToString([]byte(s.Method + ":" + s.Password))
	// Build manually to keep the base64 userinfo unescaped.
	return "ss://" + user + "@" + hostPort(s) + "#" + url.PathEscape(s.DisplayName()), nil
}

func hostPort(s *Server) string {
	return s.Address + ":" + strconv.Itoa(s.Port)
}
