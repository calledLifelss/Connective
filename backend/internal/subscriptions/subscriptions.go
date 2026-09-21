// Package subscriptions implements first-class subscription objects:
// download, validation, parsing, metadata extraction and refresh merges
// that preserve local user state.
//
// Behavior is modeled on v2rayN's SubscriptionHandler (validate URL,
// download with proxy-then-direct fallback, optional subconverter step,
// AddBatchServers dedup) and Hiddify's remote-profile update flow
// (profile info such as remaining traffic/expiry, periodic auto update).
// All code here is independently implemented; see
// REFERENCE_IMPLEMENTATION_NOTES.md.
package subscriptions

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"connective/backend/internal/servers"
)

// MaxSubscriptionBytes caps a downloaded subscription body. Subscription
// data is untrusted input; never buffer unbounded responses.
const MaxSubscriptionBytes = 10 << 20 // 10 MiB

// Traffic metadata carried by the `subscription-userinfo` response header:
// upload=...; download=...; total=...; expire=...
type Traffic struct {
	Upload   int64     `json:"upload"`
	Download int64     `json:"download"`
	Total    int64     `json:"total"` // 0 means unlimited/unknown
	Expire   time.Time `json:"expire"`
	HasLimit bool      `json:"hasLimit"`
	HasExp   bool      `json:"hasExp"`
}

// Subscription is a first-class remote server source.
type Subscription struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	URL            string        `json:"url"`
	MoreURLs       []string      `json:"moreUrls,omitempty"`
	UserAgent      string        `json:"userAgent,omitempty"`
	Enabled        bool          `json:"enabled"`
	UpdateInterval time.Duration `json:"updateInterval,omitempty"`
	LastUpdate     time.Time     `json:"lastUpdate,omitempty"`
	Traffic        Traffic       `json:"traffic"`
	ServerCount    int           `json:"serverCount"`
	LastError      string        `json:"lastError,omitempty"`
}

// UpdateResult is the outcome of one refresh attempt.
type UpdateResult struct {
	Servers []*servers.Server
	Traffic Traffic
}

// Fetcher downloads subscription bodies. It is an interface so the update
// pipeline (parse/merge) can be unit-tested without HTTP.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (body string, userinfo string, err error)
}

// Validate rejects subscriptions that must never be fetched: empty id,
// empty URL, or non-http(s) schemes (mirrors v2rayN's IsValidSubscription
// gate; subscription bodies are untrusted input).
func Validate(sub *Subscription) error {
	if strings.TrimSpace(sub.ID) == "" {
		return fmt.Errorf("subscription has no id")
	}
	u := strings.TrimSpace(sub.URL)
	if u == "" {
		return fmt.Errorf("subscription %q has no url", sub.Name)
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return fmt.Errorf("subscription %q: only http(s) urls allowed", sub.Name)
	}
	return nil
}

// HTTPFetcher is the production Fetcher.
type HTTPFetcher struct {
	Client *http.Client
	// ProxyURL, when set with TryProxy, is attempted first; a proxy
	// failure falls back to a direct fetch (v2rayN parity). The daemon
	// points this at the live mixed inbound while connected.
	ProxyURL string
	TryProxy bool
}

// NewHTTPFetcher builds a fetcher with a sane timeout.
func NewHTTPFetcher(timeout time.Duration) *HTTPFetcher {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPFetcher{Client: &http.Client{Timeout: timeout}}
}

// Fetch downloads the raw subscription body and returns the value of the
// subscription-userinfo header (possibly empty).
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (string, string, error) {
	if f.TryProxy && f.ProxyURL != "" {
		if body, info, err := f.fetchVia(ctx, url, f.ProxyURL); err == nil {
			return body, info, nil
		}
		// Proxy failed: fall back to direct (may itself fail).
	}
	return f.fetchVia(ctx, url, "")
}

func (f *HTTPFetcher) fetchVia(ctx context.Context, url, proxyURL string) (string, string, error) {
	client := f.Client
	if proxyURL != "" {
		proxy, err := parseProxyURL(proxyURL)
		if err != nil {
			return "", "", err
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxy)
		client = &http.Client{Transport: transport, Timeout: f.Client.Timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("bad subscription url: %w", err)
	}
	req.Header.Set("User-Agent", "connective/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("subscription download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("subscription server returned %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxSubscriptionBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("subscription download failed: %w", err)
	}
	if len(raw) > MaxSubscriptionBytes {
		return "", "", fmt.Errorf("subscription body exceeds %d bytes", MaxSubscriptionBytes)
	}
	return string(raw), resp.Header.Get("subscription-userinfo"), nil
}

func parseProxyURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("bad proxy url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return nil, fmt.Errorf("unsupported proxy scheme %q", u.Scheme)
	}
	return u, nil
}

// UpdateOne refreshes a single subscription: validates, downloads the
// main URL plus MoreURLs (concatenated, base64-aware like v2rayN), parses
// and tags every server. Disabled subscriptions are skipped by the
// caller (UpdateAll); calling UpdateOne directly forces a refresh.
func UpdateOne(ctx context.Context, f Fetcher, sub *Subscription) (*UpdateResult, error) {
	if err := Validate(sub); err != nil {
		return nil, err
	}
	body, userinfo, err := f.Fetch(ctx, sub.URL)
	if err != nil {
		return nil, err
	}
	for _, extra := range sub.MoreURLs {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		more, _, merr := f.Fetch(ctx, extra)
		if merr != nil {
			continue // one bad mirror must not fail the update
		}
		if decoded, derr := base64.StdEncoding.DecodeString(pad(strings.TrimSpace(more))); derr == nil {
			more = string(decoded)
		}
		body += "\n" + more
	}
	return parseBody(body, userinfo, sub.ID)
}

// UpdateAll refreshes every enabled subscription, collecting per-sub
// errors instead of failing fast. It returns the merged server list and
// the subscriptions with updated metadata (LastUpdate/Traffic/LastError).
func UpdateAll(ctx context.Context, f Fetcher, subs []*Subscription) ([]*servers.Server, []*Subscription) {
	var all []*servers.Server
	out := make([]*Subscription, 0, len(subs))
	for _, sub := range subs {
		cp := *sub
		if !cp.Enabled {
			out = append(out, &cp)
			continue
		}
		res, err := UpdateOne(ctx, f, &cp)
		if err != nil {
			cp.LastError = err.Error()
			out = append(out, &cp)
			continue
		}
		cp.LastError = ""
		cp.LastUpdate = time.Now()
		cp.Traffic = res.Traffic
		cp.ServerCount = len(res.Servers)
		all = append(all, res.Servers...)
		out = append(out, &cp)
	}
	return all, out
}

// Update is a convenience wrapper retaining the phase-1 signature.
func Update(ctx context.Context, f Fetcher, subURL, subID string) (*UpdateResult, error) {
	body, userinfo, err := f.Fetch(ctx, subURL)
	if err != nil {
		return nil, err
	}
	return parseBody(body, userinfo, subID)
}

func parseBody(body, userinfo, subID string) (*UpdateResult, error) {
	links := SplitLinks(body)
	if len(links) == 0 {
		return nil, fmt.Errorf("subscription contains no server links")
	}
	res := &UpdateResult{Traffic: ParseUserInfo(userinfo)}
	seen := map[string]bool{}
	for _, link := range links {
		s, err := servers.ParseShareLink(link)
		if err != nil {
			// One bad entry must not poison the whole subscription.
			continue
		}
		if seen[s.Key()] {
			continue // duplicate node in the same fetch
		}
		seen[s.Key()] = true
		s.SubscriptionID = subID
		res.Servers = append(res.Servers, s)
	}
	if len(res.Servers) == 0 {
		return nil, fmt.Errorf("subscription contained %d links but none parsed", len(links))
	}
	return res, nil
}

// SplitLinks extracts candidate share links from a subscription body,
// which may be plain text (one link per line) or a base64 blob encoding
// such a list. Non-link lines are ignored.
func SplitLinks(body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if lines := linkLines(body); len(lines) > 0 {
		return lines
	}
	if decoded, err := base64.StdEncoding.DecodeString(pad(body)); err == nil {
		if lines := linkLines(string(decoded)); len(lines) > 0 {
			return lines
		}
	}
	if decoded, err := base64.URLEncoding.DecodeString(pad(body)); err == nil {
		if lines := linkLines(string(decoded)); len(lines) > 0 {
			return lines
		}
	}
	return nil
}

func linkLines(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "://") {
			out = append(out, line)
		}
	}
	return out
}

func pad(s string) string {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return s
}

// ParseUserInfo parses `upload=N; download=N; total=N; expire=N`.
func ParseUserInfo(header string) Traffic {
	var t Traffic
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "upload":
			t.Upload = n
		case "download":
			t.Download = n
		case "total":
			t.Total, t.HasLimit = n, n > 0
		case "expire":
			if n > 0 {
				t.Expire, t.HasExp = time.Unix(n, 0), true
			}
		}
	}
	return t
}

// Merge combines a fresh subscription fetch with previously stored servers.
// Remote data wins for connection fields; local user metadata (ID,
// Favorite, CustomName, cached test results) is preserved. Servers that
// vanished remotely are dropped; purely local servers (other subscription
// IDs, or none) are left untouched by the caller.
func Merge(existing, fresh []*servers.Server) []*servers.Server {
	byKey := make(map[string]*servers.Server, len(existing))
	for _, s := range existing {
		byKey[s.Key()] = s
	}
	merged := make([]*servers.Server, 0, len(fresh))
	for _, f := range fresh {
		if old, ok := byKey[f.Key()]; ok {
			f.ID = old.ID
			f.Favorite = old.Favorite
			f.CustomName = old.CustomName
			f.LatencyMs = old.LatencyMs
			f.LastTest = old.LastTest
			f.Health = old.Health
		}
		merged = append(merged, f)
	}
	return merged
}
