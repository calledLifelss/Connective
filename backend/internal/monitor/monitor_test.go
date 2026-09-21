package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestProbeDirect(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ok.Close()
	if err := Probe(context.Background(), "", ok.URL, 3*time.Second); err != nil {
		t.Fatalf("204 should pass: %v", err)
	}
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Success")
	}))
	defer good.Close()
	if err := Probe(context.Background(), "", good.URL, 3*time.Second); err != nil {
		t.Fatalf("200 should pass: %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bad.Close()
	if err := Probe(context.Background(), "", bad.URL, 3*time.Second); err == nil {
		t.Fatalf("403 should fail")
	}
	if err := Probe(context.Background(), "", "http://127.0.0.1:1/", 300*time.Millisecond); err == nil {
		t.Fatalf("refused should fail")
	}
}

func TestProbeURLsFallback(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer good.Close()
	// Primary dead, fallback alive => healthy.
	if err := ProbeURLs(context.Background(), "", []string{bad.URL, good.URL}, 3*time.Second); err != nil {
		t.Fatalf("fallback should succeed: %v", err)
	}
	// All dead => error naming culprits.
	if err := ProbeURLs(context.Background(), "", []string{bad.URL}, 3*time.Second); err == nil {
		t.Fatalf("expected all-failed error")
	}
	// Empty list => configuration error, not success.
	if err := ProbeURLs(context.Background(), "", nil, 3*time.Second); err == nil {
		t.Fatalf("expected no-url error")
	}
}

func TestProbeViaProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "proxied")
	}))
	defer target.Close()
	// Minimal forward proxy: whatever the path, fetch it and relay.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(target.URL)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	if err := Probe(context.Background(), proxy.URL, target.URL, 3*time.Second); err != nil {
		t.Fatalf("via proxy: %v", err)
	}
}

func TestSnapshotStable(t *testing.T) {
	a, err := Snapshot()
	if err != nil || a == "" {
		t.Fatalf("snapshot: %v %q", a, err)
	}
	b, err := Snapshot()
	if err != nil || a != b {
		t.Fatalf("snapshot unstable: %q vs %q", a, b)
	}
}

func TestWatcherFires(t *testing.T) {
	states := []string{"a", "a", "b", "b", "c"}
	var mu sync.Mutex
	i := 0
	w := &Watcher{
		Interval: 20 * time.Millisecond,
		Take: func() (string, error) {
			mu.Lock()
			defer mu.Unlock()
			s := states[i]
			if i < len(states)-1 {
				i++
			}
			return s, nil
		},
	}
	var changes []string
	var cmu sync.Mutex
	w.OnChange = func(before, after string) {
		cmu.Lock()
		changes = append(changes, before+"->"+after)
		cmu.Unlock()
	}
	w.Start()
	time.Sleep(300 * time.Millisecond)
	w.Stop()
	w.Stop() // idempotent
	cmu.Lock()
	defer cmu.Unlock()
	if len(changes) != 2 || changes[0] != "a->b" || changes[1] != "b->c" {
		t.Fatalf("bad changes: %v", changes)
	}
}
