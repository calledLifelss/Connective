// Package servers defines Connective's server data model and share-link
// parsing/generation.
//
// The model is informed by studying v2rayN's ProfileItem
// (Models/Entities/ProfileItem.cs plus ProtocolExtraItem and
// TransportExtraItem) and Hiddify's sing-box profile handling, but every
// line here is an independent implementation. No reference source is
// copied: both Hiddify (GPLv3 + additional conditions) and v2rayN (GPLv3)
// are inspected for behavior only. See REFERENCE_IMPLEMENTATION_NOTES.md.
package servers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Protocol is a proxy protocol supported by the underlying core.
type Protocol string

const (
	ProtocolVLESS       Protocol = "vless"
	ProtocolVMess       Protocol = "vmess"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolSocks       Protocol = "socks"
	ProtocolWireGuard   Protocol = "wireguard"
	ProtocolHysteria2   Protocol = "hysteria2"
	ProtocolTUIC        Protocol = "tuic"
)

// Transport is a stream transport.
type Transport string

const (
	TransportTCP  Transport = "tcp"
	TransportWS   Transport = "ws"
	TransportGRPC Transport = "grpc"
	TransportH2   Transport = "h2"
	TransportHTTP Transport = "http"
	TransportQUIC Transport = "quic"
)

// Security is the transport security layer.
type Security string

const (
	SecurityNone    Security = "none"
	SecurityTLS     Security = "tls"
	SecurityReality Security = "reality"
)

// Health describes the last known health of a server.
type Health string

const (
	HealthUnknown   Health = "unknown"
	HealthHealthy   Health = "healthy"
	HealthDegraded  Health = "degraded"
	HealthUnhealthy Health = "unhealthy"
)

// Server is the canonical Connective server record. Remote subscription
// data and local user metadata live in the same struct but are merged
// carefully on subscription refresh: local fields (Favorite, custom Name
// edits) are preserved. See subscriptions.Merge.
type Server struct {
	// Identity.
	ID             string `json:"id"`
	Name           string `json:"name"`
	SubscriptionID string `json:"subscriptionId,omitempty"`

	// Endpoint.
	Address string `json:"address"`
	Port    int    `json:"port"`

	// Protocol core.
	Protocol  Protocol  `json:"protocol"`
	Transport Transport `json:"transport"`
	Security  Security  `json:"security"`

	// Auth / crypto.
	UUID      string `json:"uuid,omitempty"`      // vless/vmess id
	Password  string `json:"password,omitempty"`  // trojan/ss password
	Method    string `json:"method,omitempty"`    // shadowsocks method
	Flow      string `json:"flow,omitempty"`      // vless flow (xtls-rprx-vision, ...)
	PublicKey string `json:"publicKey,omitempty"` // reality pbk
	ShortID   string `json:"shortId,omitempty"`   // reality sid

	// TLS / transport details.
	SNI         string `json:"sni,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	ALPN        string `json:"alpn,omitempty"`
	Path        string `json:"path,omitempty"`        // ws/h2/http path
	Host        string `json:"host,omitempty"`        // ws host header
	ServiceName string `json:"serviceName,omitempty"` // grpc service name

	// Classification.
	Country string   `json:"country,omitempty"`
	Tags    []string `json:"tags,omitempty"`

	// Local user metadata (never overwritten by subscription refresh).
	Favorite   bool   `json:"favorite"`
	CustomName string `json:"customName,omitempty"`

	// Test results (updated by the tester, cached in persistence).
	LatencyMs int64     `json:"latencyMs"` // -1 means never tested
	LastTest  time.Time `json:"lastTest,omitempty"`
	Health    Health    `json:"health"`
}

// DisplayName returns the user-visible name, preferring a custom name.
func (s *Server) DisplayName() string {
	if s.CustomName != "" {
		return s.CustomName
	}
	return s.Name
}

// Tested reports whether the server has ever produced a test result.
func (s *Server) Tested() bool { return s.LatencyMs >= 0 }

// Validate checks structural invariants. It treats all imported data as
// untrusted input: addresses, ports and enum values are verified before
// the server is stored or rendered into a core configuration.
func (s *Server) Validate() error {
	if s.Address == "" {
		return fmt.Errorf("server %q: empty address", s.Name)
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("server %q: port %d out of range", s.Name, s.Port)
	}
	switch s.Protocol {
	case ProtocolVLESS, ProtocolVMess, ProtocolTrojan,
		ProtocolShadowsocks, ProtocolSocks, ProtocolWireGuard,
		ProtocolHysteria2, ProtocolTUIC:
	default:
		return fmt.Errorf("server %q: unsupported protocol %q", s.Name, s.Protocol)
	}
	switch s.Transport {
	case "", TransportTCP, TransportWS, TransportGRPC,
		TransportH2, TransportHTTP, TransportQUIC:
	default:
		return fmt.Errorf("server %q: unsupported transport %q", s.Name, s.Transport)
	}
	switch s.Security {
	case "", SecurityNone, SecurityTLS, SecurityReality:
		return nil
	default:
		return fmt.Errorf("server %q: unsupported security %q", s.Name, s.Security)
	}
}

// Key returns a stable dedup identity for a server: protocol, endpoint and
// authentication identity. Two records with the same key describe the same
// remote server, even if display names differ.
func (s *Server) Key() string {
	auth := s.UUID
	if auth == "" {
		auth = s.Password
	}
	return string(s.Protocol) + "|" + s.Address + "|" +
		fmt.Sprint(s.Port) + "|" + auth + "|" + s.Method
}

// NewID generates a random server ID.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// Normalize fills defaults for a freshly parsed or created server.
func (s *Server) Normalize() {
	if s.ID == "" {
		s.ID = NewID()
	}
	if s.Transport == "" {
		s.Transport = TransportTCP
	}
	if s.Security == "" {
		s.Security = SecurityNone
	}
	if s.Health == "" {
		s.Health = HealthUnknown
	}
	if s.LatencyMs == 0 {
		s.LatencyMs = -1
	}
}
