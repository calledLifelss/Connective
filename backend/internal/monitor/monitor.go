// Package monitor watches connection health and the host network.
//
// Health: the active path is proven by fetching the connection-test URL
// through the mixed inbound (Hiddify's connectionTestUrl default) and by
// requiring the core process plus clash API to answer. Any failure means
// the path is down — "core process exists" is never equated with working
// Internet.
//
// Network changes: a poller snapshots interfaces + default route; on
// change the daemon re-verifies health and reconnects when broken
// (covers Wi-Fi↔Ethernet hops, sleep/wake, DHCP changes).
package monitor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProbeURLs fetches each test URL in order through proxyURL and returns
// nil on the first success. Multiple URLs matter: a single probe target
// can be unreachable from a given network (IPv6-only hosts on IPv4-only
// upstreams, CDN blocks) without the tunnel being broken. Only
// all-failing means down.
func ProbeURLs(ctx context.Context, proxyURL string, testURLs []string, timeout time.Duration) error {
	var errs []string
	for _, u := range testURLs {
		if u == "" {
			continue
		}
		if err := Probe(ctx, proxyURL, u, timeout); err == nil {
			return nil
		} else {
			errs = append(errs, u+": "+err.Error())
		}
	}
	if len(errs) == 0 {
		return fmt.Errorf("no test url configured")
	}
	return fmt.Errorf("all probes failed: %s", strings.Join(errs, "; "))
}

// Probe fetches testURL (directly or via proxyURL) and validates the
// answer: HTTP 204, or 2xx with a non-empty body. It is the same class
// of check both reference clients use (generate_204 / captive-portal
// URLs).
func Probe(ctx context.Context, proxyURL, testURL string, timeout time.Duration) error {
	if testURL == "" {
		return fmt.Errorf("no test url configured")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("bad proxy url: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", testURL, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("probe %s: unexpected status %s", testURL, resp.Status)
}

// Snapshot captures the current network identity: up interfaces (name,
// MAC, addresses) plus the default-route table.
func Snapshot() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var parts []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		var as []string
		for _, a := range addrs {
			as = append(as, a.String())
		}
		sort.Strings(as)
		parts = append(parts, iface.Name+"|"+iface.HardwareAddr.String()+"|"+strings.Join(as, ","))
	}
	sort.Strings(parts)
	routes, err := defaultRoutes()
	if err != nil {
		routes = "routes-unavailable"
	}
	return strings.Join(parts, ";") + "||" + routes, nil
}

func defaultRoutes() (string, error) {
	raw, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", err
	}
	var def []string
	for _, line := range strings.Split(string(raw), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 3 && f[1] == "00000000" {
			def = append(def, f[0]+"="+f[2])
		}
	}
	sort.Strings(def)
	return strings.Join(def, ","), nil
}

// Watcher polls Snapshot and calls OnChange whenever it differs.
// Confirm sets how many consecutive differing polls are required before
// firing (default 1); 2+ debounces single-poll noise on busy machines.
type Watcher struct {
	Interval time.Duration
	Confirm  int
	Take     func() (string, error) // defaults to Snapshot
	OnChange func(before, after string)

	mu      sync.Mutex
	stop    chan struct{}
	stopped bool
}

// Start begins polling. The first snapshot is the baseline (no callback).
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.stop != nil {
		w.mu.Unlock()
		return
	}
	w.stop = make(chan struct{})
	w.mu.Unlock()
	take := w.Take
	if take == nil {
		take = Snapshot
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	confirm := w.Confirm
	if confirm <= 0 {
		confirm = 1
	}
	go func() {
		base, err := take()
		if err != nil {
			base = ""
		}
		pending, rounds := "", 0
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-tick.C:
				cur, err := take()
				if err != nil {
					continue
				}
				if cur == base {
					pending, rounds = "", 0
					continue
				}
				if cur != pending {
					pending, rounds = cur, 1
				} else {
					rounds++
				}
				if rounds >= confirm {
					before := base
					base, pending, rounds = cur, "", 0
					if w.OnChange != nil {
						w.OnChange(before, cur)
					}
				}
			}
		}
	}()
}

// Stop halts polling (idempotent).
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.stop == nil {
		return
	}
	w.stopped = true
	close(w.stop)
}
