// Package tester performs asynchronous server health tests.
//
// Two stages, mirroring the mature clients' approach (v2rayN's
// SpeedtestService distinguishes Tcping from Realping; Hiddify's UX shows
// Testing/waiting states per row):
//
//  1. TCP connect timing ("tcping"): fast, always available.
//  2. Real ping through the running core (phase 2, needs a live core):
//     HTTP fetch of a probe URL via the mixed inbound.
//
// Testing never blocks the caller: RunBulk fans out with bounded
// concurrency, honors context cancellation, and reports per-server
// results through a channel. Results feed the selector and persistence.
package tester

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"connective/backend/internal/servers"
)

// Result is one finished test.
type Result struct {
	ServerID  string
	LatencyMs int64 // >=0 on success
	Err       error
	At        time.Time
}

// Options tunes testing.
type Options struct {
	Timeout     time.Duration
	Concurrency int
	Retries     int
}

// Defaults returns sane options.
func Defaults() Options {
	return Options{Timeout: 5 * time.Second, Concurrency: 10, Retries: 1}
}

// TCPing measures raw TCP connect latency to the server endpoint.
func TCPing(ctx context.Context, address string, port int, timeout time.Duration) (int64, error) {
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		return -1, err
	}
	conn.Close()
	return time.Since(start).Milliseconds(), nil
}

// Outcome applies a Result to its server record.
func Outcome(s *servers.Server, r Result) {
	s.LastTest = r.At
	if r.Err != nil {
		s.LatencyMs = -1
		s.Health = servers.HealthUnhealthy
		return
	}
	s.LatencyMs = r.LatencyMs
	if r.LatencyMs > 2500 {
		s.Health = servers.HealthDegraded
	} else {
		s.Health = servers.HealthHealthy
	}
}

// RunBulk tests every server concurrently (bounded), streaming Results.
// It returns when all tests finish or ctx is canceled.
func RunBulk(ctx context.Context, list []*servers.Server, opts Options, test func(context.Context, *servers.Server) Result) <-chan Result {
	if opts.Concurrency <= 0 {
		opts.Concurrency = Defaults().Concurrency
	}
	out := make(chan Result)
	go func() {
		defer close(out)
		sem := make(chan struct{}, opts.Concurrency)
		var wg sync.WaitGroup
		for _, s := range list {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(srv *servers.Server) {
				defer wg.Done()
				defer func() { <-sem }()
				out <- test(ctx, srv)
			}(s)
		}
		wg.Wait()
	}()
	return out
}

// TCPTestFunc adapts TCPing to RunBulk's test signature with retries.
func TCPTestFunc(opts Options) func(context.Context, *servers.Server) Result {
	return func(ctx context.Context, s *servers.Server) Result {
		var err error
		var latency int64 = -1
		for i := 0; i <= opts.Retries; i++ {
			latency, err = TCPing(ctx, s.Address, s.Port, opts.Timeout)
			if err == nil || ctx.Err() != nil {
				break
			}
		}
		return Result{ServerID: s.ID, LatencyMs: latency, Err: err, At: time.Now()}
	}
}
