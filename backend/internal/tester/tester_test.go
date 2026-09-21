package tester

import (
	"context"
	"net"
	"testing"
	"time"

	"connective/backend/internal/servers"
)

func TestTCPingLocalhost(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	addr := l.Addr().(*net.TCPAddr)
	lat, err := TCPing(context.Background(), "127.0.0.1", addr.Port, 2*time.Second)
	if err != nil {
		t.Fatalf("tcping: %v", err)
	}
	if lat < 0 || lat > 2000 {
		t.Fatalf("implausible latency %d", lat)
	}
}

func TestTCPingRefused(t *testing.T) {
	// Port 1 on localhost is (almost) certainly closed.
	_, err := TCPing(context.Background(), "127.0.0.1", 1, 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected connection error")
	}
}

func TestRunBulk(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port
	mk := func(id, host string, p int) *servers.Server {
		return &servers.Server{ID: id, Name: id, Address: host, Port: p, Protocol: servers.ProtocolVLESS}
	}
	list := []*servers.Server{
		mk("ok1", "127.0.0.1", port),
		mk("ok2", "127.0.0.1", port),
		mk("bad", "127.0.0.1", 1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := map[string]Result{}
	for r := range RunBulk(ctx, list, Options{Timeout: 2 * time.Second, Concurrency: 2, Retries: 0}, TCPTestFunc(Defaults())) {
		results[r.ServerID] = r
	}
	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
	if results["ok1"].Err != nil || results["ok2"].Err != nil {
		t.Errorf("local servers should succeed: %+v", results)
	}
	if results["bad"].Err == nil {
		t.Errorf("closed port should fail")
	}
	// Outcomes apply cleanly to records.
	s := mk("ok1", "127.0.0.1", port)
	Outcome(s, results["ok1"])
	if !s.Tested() || s.Health != servers.HealthHealthy {
		t.Errorf("bad outcome: %+v", s)
	}
	bad := mk("bad", "127.0.0.1", 1)
	Outcome(bad, results["bad"])
	if bad.Tested() || bad.Health != servers.HealthUnhealthy {
		t.Errorf("bad outcome: %+v", bad)
	}
}

func TestRunBulkCancel(t *testing.T) {
	mk := func(id string) *servers.Server {
		return &servers.Server{ID: id, Address: "192.0.2.1", Port: 443, Protocol: servers.ProtocolVLESS}
	}
	var list []*servers.Server
	for i := 0; i < 20; i++ {
		list = append(list, mk(string(rune('a'+i))))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	count := 0
	for range RunBulk(ctx, list, Defaults(), TCPTestFunc(Options{Timeout: time.Second, Retries: 0})) {
		count++
	}
	_ = count // cancellation may still deliver a few; key check is termination
}
