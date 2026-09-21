package stats

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeAPI(t *testing.T, secret string, switchSeen *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret != "" && r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/connections":
			fmt.Fprint(w, `{"connections":[],"downloadTotal":123456,"uploadTotal":7890}`)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/proxies/"):
			raw, _ := io.ReadAll(r.Body)
			*switchSeen = string(raw)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestTotals(t *testing.T) {
	var seen string
	srv := fakeAPI(t, "s3cret", &seen)
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Secret: "s3cret", http: &http.Client{Timeout: 5 * time.Second}}
	tot, err := c.Totals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tot.Down != 123456 || tot.Up != 7890 {
		t.Fatalf("bad totals: %+v", tot)
	}
}

func TestAuthEnforced(t *testing.T) {
	var seen string
	srv := fakeAPI(t, "s3cret", &seen)
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Secret: "wrong", http: &http.Client{Timeout: 5 * time.Second}}
	if _, err := c.Totals(context.Background()); err == nil {
		t.Fatalf("expected auth error")
	}
}

func TestSwitchProxy(t *testing.T) {
	var seen string
	srv := fakeAPI(t, "", &seen)
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, http: &http.Client{Timeout: 5 * time.Second}}
	if err := c.SwitchProxy(context.Background(), "proxy", "proxy-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "proxy-1") {
		t.Fatalf("bad switch body: %q", seen)
	}
}

func TestTrackerRates(t *testing.T) {
	up, down := int64(0), int64(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"connections":[],"downloadTotal":%d,"uploadTotal":%d}`, down, up)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, http: &http.Client{Timeout: 5 * time.Second}}
	tr := NewTracker(c)
	if _, err := tr.Sample(context.Background()); err != nil {
		t.Fatal(err)
	}
	up, down = 2000, 4000
	time.Sleep(1100 * time.Millisecond)
	snap, err := tr.Sample(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Up != 2000 || snap.Down != 4000 {
		t.Fatalf("bad cumulative: %+v", snap)
	}
	if snap.UpRate < 1000 || snap.UpRate > 3000 || snap.DownRate < 2000 || snap.DownRate > 6000 {
		t.Fatalf("implausible rates: %+v", snap)
	}
}
