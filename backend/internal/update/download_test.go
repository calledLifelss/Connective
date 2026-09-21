package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadSuccessProgress(t *testing.T) {
	body := strings.Repeat("x", 200000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.Write([]byte(body))
	}))
	defer srv.Close()

	var calls int
	var lastDone int64
	dl := &Downloader{
		Client: srv.Client(),
		OnProgress: func(done, total int64) bool {
			calls++
			lastDone = done
			if total != int64(len(body)) {
				t.Errorf("total %d, want %d", total, len(body))
			}
			return true
		},
	}
	path, sum, err := dl.Fetch(context.Background(), srv.URL+"/f.zip", t.TempDir(), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if sum != sha256Of([]byte(body)) {
		t.Fatal("wire hash mismatch")
	}
	if calls == 0 || lastDone != int64(len(body)) {
		t.Fatalf("progress not reported (calls=%d last=%d)", calls, lastDone)
	}
}

func TestDownloadCancel(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // hang until the test cancels
	}))
	// Registered last → runs first: unblocks the handler before
	// srv.Close() waits for outstanding requests (LIFO).
	defer srv.Close()
	defer close(release)

	dl := &Downloader{Client: srv.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := dl.Fetch(ctx, srv.URL+"/slow.zip", t.TempDir(), 0)
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled download should fail")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop the download")
	}
}

func TestDownloadTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()
	c := srv.Client()
	c.Timeout = 300 * time.Millisecond
	dl := &Downloader{Client: c, MaxBytes: MaxArtifactBytes}
	if _, _, err := dl.Fetch(context.Background(), srv.URL+"/t.zip", t.TempDir(), 0); err == nil {
		t.Fatal("timeout should fail the download")
	}
}

func TestDownloadSizeCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("y", 10000)))
	}))
	defer srv.Close()
	dl := &Downloader{Client: srv.Client(), MaxBytes: 100}
	if _, _, err := dl.Fetch(context.Background(), srv.URL+"/big.zip", t.TempDir(), 0); err == nil {
		t.Fatal("oversize artifact must be rejected")
	}
	if files, _ := filepath.Glob(filepath.Join(os.TempDir(), "artifact-*.part")); len(files) > 0 {
		// Staging dir is per-test temp; nothing global should leak.
		t.Logf("note: %d stray part files (test temp, cleaned by runner)", len(files))
	}
}

func TestDownloadResume(t *testing.T) {
	body := strings.Repeat("z", 100000)
	var first = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if first && r.Header.Get("Range") == "" {
			first = false
			// Simulate a dropped connection halfway.
			w.Write([]byte(body[:50000]))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				return
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		// Second attempt: honor Range.
		const prefix = "bytes="
		rangeHdr := r.Header.Get("Range")
		start := 0
		fmt.Sscanf(rangeHdr, prefix+"%d-", &start)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(body)-1, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte(body[start:]))
	}))
	defer srv.Close()

	dl := &Downloader{Client: srv.Client()}
	path, sum, err := dl.Fetch(context.Background(), srv.URL+"/r.zip", t.TempDir(), int64(len(body)))
	if err != nil {
		t.Fatalf("resume should recover: %v", err)
	}
	defer os.Remove(path)
	if sum != sha256Of([]byte(body)) {
		t.Fatal("resumed bytes corrupt")
	}
}

func TestDownloadFileURL(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.zip")
	content := []byte("local asset bytes")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	dl := &Downloader{}
	// file:// URLs need the empty-host form on every OS
	// (file:///C:/... on Windows, file:///tmp/... on unix).
	fileURL := "file:///" + strings.TrimPrefix(filepath.ToSlash(src), "/")
	path, sum, err := dl.Fetch(context.Background(), fileURL, t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if sum != sha256Of(content) {
		t.Fatal("file hash mismatch")
	}
}
