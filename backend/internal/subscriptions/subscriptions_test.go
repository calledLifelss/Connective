package subscriptions

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connective/backend/internal/servers"
)

const (
	testUUID = "11111111-2222-4333-8444-555555555555"
	vlessA   = "vless://" + testUUID + "@10.0.0.1:443?encryption=none&security=tls&sni=a.example#A"
	vlessB   = "vless://" + testUUID + "@10.0.0.2:443?encryption=none&B"
)

func TestSplitLinksPlain(t *testing.T) {
	body := "# comment\n" + vlessA + "\n\n" + vlessB + "\r\n"
	links := SplitLinks(body)
	if len(links) != 2 {
		t.Fatalf("got %d links: %v", len(links), links)
	}
}

func TestSplitLinksBase64(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte(vlessA + "\n" + vlessB + "\n"))
	links := SplitLinks(blob)
	if len(links) != 2 {
		t.Fatalf("got %d links", len(links))
	}
}

func TestParseUserInfo(t *testing.T) {
	tr := ParseUserInfo("upload=100; download=200; total=500; expire=1780000000")
	if tr.Upload != 100 || tr.Download != 200 || tr.Total != 500 || !tr.HasLimit {
		t.Fatalf("bad traffic: %+v", tr)
	}
	if !tr.HasExp || tr.Expire.Unix() != 1780000000 {
		t.Fatalf("bad expiry: %+v", tr)
	}
	if empty := ParseUserInfo(""); empty.HasLimit || empty.HasExp {
		t.Fatalf("empty header should parse clean: %+v", empty)
	}
}

func serveSub(t *testing.T, body, userinfo string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userinfo != "" {
			w.Header().Set("subscription-userinfo", userinfo)
		}
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

func TestUpdateFlow(t *testing.T) {
	srv := serveSub(t, vlessA+"\n"+vlessB+"\n", "upload=1; download=2; total=10", 200)
	defer srv.Close()
	res, err := Update(context.Background(), NewHTTPFetcher(5*time.Second), srv.URL, "sub1")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(res.Servers) != 2 {
		t.Fatalf("got %d servers", len(res.Servers))
	}
	for _, s := range res.Servers {
		if s.SubscriptionID != "sub1" {
			t.Errorf("missing subscription tag: %+v", s)
		}
	}
	if res.Traffic.Total != 10 {
		t.Errorf("bad traffic: %+v", res.Traffic)
	}
}

func TestUpdateSkipsBadEntries(t *testing.T) {
	srv := serveSub(t, "not-a-link\n"+vlessA+"\n", "", 200)
	defer srv.Close()
	res, err := Update(context.Background(), NewHTTPFetcher(5*time.Second), srv.URL, "sub1")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(res.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(res.Servers))
	}
}

func TestUpdateDedupesNodes(t *testing.T) {
	srv := serveSub(t, vlessA+"\n"+vlessA+"\n"+vlessB+"\n", "", 200)
	defer srv.Close()
	res, err := Update(context.Background(), NewHTTPFetcher(5*time.Second), srv.URL, "sub1")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(res.Servers) != 2 {
		t.Fatalf("got %d servers, want 2 (dupe collapsed)", len(res.Servers))
	}
}

func TestUpdateLargeSubscription(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "vless://11111111-2222-4333-8444-%012x@10.99.0.1:443?encryption=none#P%d\n", i, i)
	}
	srv := serveSub(t, sb.String(), "", 200)
	defer srv.Close()
	start := time.Now()
	res, err := Update(context.Background(), NewHTTPFetcher(10*time.Second), srv.URL, "sub1")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(res.Servers) != 500 {
		t.Fatalf("got %d servers, want 500", len(res.Servers))
	}
	if dt := time.Since(start); dt > 5*time.Second {
		t.Fatalf("500-node update took %v", dt)
	}
	merged := Merge(nil, res.Servers)
	if len(merged) != 500 {
		t.Fatalf("merge lost nodes: %d", len(merged))
	}
}

func TestUpdateHTTPError(t *testing.T) {
	srv := serveSub(t, "", "", 500)
	defer srv.Close()
	if _, err := Update(context.Background(), NewHTTPFetcher(5*time.Second), srv.URL, "sub1"); err == nil {
		t.Fatalf("expected error for HTTP 500")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(&Subscription{ID: "a", URL: "https://example.com/sub"}); err != nil {
		t.Errorf("valid rejected: %v", err)
	}
	for _, sub := range []*Subscription{
		{ID: "", URL: "https://example.com/sub"},
		{ID: "a", URL: ""},
		{ID: "a", URL: "ftp://example.com/sub"},
		{ID: "a", URL: "vless://x@y:1"},
	} {
		if err := Validate(sub); err == nil {
			t.Errorf("expected rejection: %+v", sub)
		}
	}
}

func TestUpdateOneWithMoreURLs(t *testing.T) {
	srv1 := serveSub(t, vlessA+"\n", "", 200)
	defer srv1.Close()
	srv2 := serveSub(t, vlessB+"\n", "upload=1; download=2; total=9", 200)
	defer srv2.Close()
	sub := &Subscription{ID: "s", Name: "S", URL: srv1.URL, MoreURLs: []string{srv2.URL}, Enabled: true}
	res, err := UpdateOne(context.Background(), NewHTTPFetcher(5*time.Second), sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Servers) != 2 {
		t.Fatalf("got %d servers", len(res.Servers))
	}
}

func TestUpdateAll(t *testing.T) {
	good := serveSub(t, vlessA+"\n", "", 200)
	defer good.Close()
	bad := serveSub(t, "", "", 500)
	defer bad.Close()
	subs := []*Subscription{
		{ID: "ok", Name: "OK", URL: good.URL, Enabled: true},
		{ID: "bad", Name: "BAD", URL: bad.URL, Enabled: true},
		{ID: "off", Name: "OFF", URL: bad.URL, Enabled: false},
	}
	all, updated := UpdateAll(context.Background(), NewHTTPFetcher(5*time.Second), subs)
	if len(all) != 1 {
		t.Fatalf("got %d servers", len(all))
	}
	byID := map[string]*Subscription{}
	for _, u := range updated {
		byID[u.ID] = u
	}
	if byID["bad"].LastError == "" {
		t.Errorf("bad sub should record error")
	}
	if byID["off"].LastError != "" {
		t.Errorf("disabled sub should be skipped cleanly")
	}
	if byID["ok"].ServerCount != 1 || byID["ok"].LastUpdate.IsZero() {
		t.Errorf("ok sub metadata wrong: %+v", byID["ok"])
	}
}

func TestProxyFallback(t *testing.T) {
	srv := serveSub(t, vlessA+"\n", "", 200)
	defer srv.Close()
	f := NewHTTPFetcher(5 * time.Second)
	f.TryProxy = true
	f.ProxyURL = "http://127.0.0.1:1" // nothing there: must fall back
	body, _, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if len(SplitLinks(body)) != 1 {
		t.Fatalf("bad body: %q", body)
	}
}

func TestMergePreservesLocal(t *testing.T) {
	old := &servers.Server{ID: "keep-me", Name: "Old Name", Address: "10.0.0.1", Port: 443,
		Protocol: servers.ProtocolVLESS, UUID: testUUID, Favorite: true, CustomName: "Mine",
		LatencyMs: 82, Health: servers.HealthHealthy}
	fresh := &servers.Server{Name: "New Name", Address: "10.0.0.1", Port: 443,
		Protocol: servers.ProtocolVLESS, UUID: testUUID}
	merged := Merge([]*servers.Server{old}, []*servers.Server{fresh})
	if len(merged) != 1 {
		t.Fatalf("got %d", len(merged))
	}
	m := merged[0]
	if m.ID != "keep-me" || !m.Favorite || m.CustomName != "Mine" || m.LatencyMs != 82 {
		t.Errorf("local state lost: %+v", m)
	}
	if m.Name != "New Name" {
		t.Errorf("remote rename not applied: %+v", m)
	}
}
