// Package stats reads live traffic data from the core's experimental
// clash API (enabled by configgen when ClashPort > 0, on by default like
// Hiddify's enable-clash-api). All numbers come from the core; nothing
// is estimated or faked.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Totals is cumulative core traffic.
type Totals struct {
	Up   int64 `json:"uploadTotal"`
	Down int64 `json:"downloadTotal"`
}

// Snapshot is one traffic sample with session deltas computed by the caller.
type Snapshot struct {
	Up       int64
	Down     int64
	UpRate   int64 // bytes/s since last snapshot
	DownRate int64
	At       time.Time
}

// Client talks to the clash API controller.
type Client struct {
	BaseURL string // e.g. http://127.0.0.1:16756
	Secret  string
	http    *http.Client
}

// New creates a Client for 127.0.0.1:port.
func New(port int, secret string) *Client {
	return &Client{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		Secret:  secret,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clash api %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clash api %s: %s", path, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// Totals returns cumulative upload/download from GET /connections.
func (c *Client) Totals(ctx context.Context) (Totals, error) {
	raw, err := c.get(ctx, "/connections")
	if err != nil {
		return Totals{}, err
	}
	var body struct {
		Totals
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Totals{}, fmt.Errorf("clash api /connections: %w", err)
	}
	return body.Totals, nil
}

// SwitchProxy changes a selector/urltest group's active choice without a
// core restart: PUT /proxies/{group} {"name": target}. Used for manual
// server switches inside a running group config.
func (c *Client) SwitchProxy(ctx context.Context, group, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.BaseURL+"/proxies/"+url.PathEscape(group),
		jsonBody(map[string]string{"name": target}))
	if err != nil {
		return err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clash api switch: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clash api switch: %s", resp.Status)
	}
	return nil
}

// Tracker turns Totals polls into per-second snapshots.
type Tracker struct {
	client *Client
	last   Totals
	lastAt time.Time
}

// NewTracker creates a Tracker.
func NewTracker(client *Client) *Tracker { return &Tracker{client: client} }

// Sample polls once and returns rates since the previous sample.
func (t *Tracker) Sample(ctx context.Context) (Snapshot, error) {
	now := time.Now()
	tot, err := t.client.Totals(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{Up: tot.Up, Down: tot.Down, At: now}
	if !t.lastAt.IsZero() {
		dt := now.Sub(t.lastAt).Seconds()
		if dt > 0 {
			snap.UpRate = int64(float64(tot.Up-t.last.Up) / dt)
			snap.DownRate = int64(float64(tot.Down-t.last.Down) / dt)
			if snap.UpRate < 0 {
				snap.UpRate = 0
			}
			if snap.DownRate < 0 {
				snap.DownRate = 0
			}
		}
	}
	t.last, t.lastAt = tot, now
	return snap, nil
}

func jsonBody(v any) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(json.NewEncoder(pw).Encode(v))
	}()
	return pr
}
