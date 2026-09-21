package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Download policy.
const (
	MaxArtifactBytes = 512 << 20 // 512 MiB safety cap
	maxRetries       = 3
)

// ProgressFunc receives live byte counts; return false to abort.
type ProgressFunc func(done, total int64) bool

// Downloader streams artifacts into a staging directory: progress,
// cancellation, timeout, retry, resume (HTTP Range where offered),
// size caps, and SHA-256 calculated while downloading. It never writes
// into the live installation.
type Downloader struct {
	// Client performs HTTP fetches (httptest-injectable in tests).
	Client *http.Client
	// MaxBytes caps a single artifact (0 = MaxArtifactBytes).
	MaxBytes int64
	// OnProgress is called per chunk (may be nil).
	OnProgress ProgressFunc
}

func (d *Downloader) client() *http.Client {
	if d.Client != nil {
		return d.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (d *Downloader) maxBytes() int64 {
	if d.MaxBytes > 0 {
		return d.MaxBytes
	}
	return MaxArtifactBytes
}

// Fetch downloads rawURL into a temp file under stageDir and returns
// the staged path plus the hex SHA-256 observed on the wire.
func (d *Downloader) Fetch(ctx context.Context, rawURL, stageDir string, expectedSize int64) (string, string, error) {
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return "", "", fmt.Errorf("update: stage: %w", err)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("update: bad url: %w", err)
	}
	tmp, err := os.CreateTemp(stageDir, "artifact-*.part")
	if err != nil {
		return "", "", fmt.Errorf("update: stage: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
	}()
	fail := func(err error) (string, string, error) {
		os.Remove(tmpName)
		return "", "", err
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fail(ctx.Err())
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		// Resume: continue from what a previous attempt staged.
		var have int64
		if st, serr := tmp.Stat(); serr == nil {
			have = st.Size()
		}
		done, total, h, rerr := d.attempt(ctx, u, tmp, have, expectedSize)
		_ = done
		if rerr == nil {
			sum := h
			if err := tmp.Close(); err != nil {
				return fail(err)
			}
			if total > 0 && done != total {
				return fail(fmt.Errorf("update: short download (%d/%d)", done, total))
			}
			return tmpName, sum, nil
		}
		lastErr = rerr
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
	}
	return fail(fmt.Errorf("update: download failed after %d tries: %w", maxRetries, lastErr))
}

// attempt performs one fetch pass, appending from offset `have`.
func (d *Downloader) attempt(ctx context.Context, u *url.URL, tmp *os.File, have, expectedSize int64) (int64, int64, string, error) {
	var src io.ReadCloser
	var total = expectedSize
	switch u.Scheme {
	case "file":
		if u.Host != "" && u.Host != "localhost" {
			return 0, 0, "", fmt.Errorf("update: refusing remote file host")
		}
		// URL paths always start with '/'; Windows needs the drive
		// form (C:/…), not the URL form (/C:/…).
		fpath := u.Path
		if len(fpath) > 2 && fpath[0] == '/' && fpath[2] == ':' {
			fpath = fpath[1:]
		}
		f, err := os.Open(fpath)
		if err != nil {
			return 0, 0, "", fmt.Errorf("update: missing asset: %w", err)
		}
		defer f.Close()
		if have > 0 {
			if _, err := f.Seek(have, io.SeekStart); err != nil {
				return 0, 0, "", err
			}
		}
		if total <= 0 {
			if st, err := f.Stat(); err == nil {
				total = st.Size()
			}
		}
		src = io.NopCloser(f)
		// file assets are local: hash from scratch each attempt.
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return 0, 0, "", err
		}
		if err := tmp.Truncate(0); err != nil {
			return 0, 0, "", err
		}
		have = 0
	default:
		if u.Scheme != "http" && u.Scheme != "https" {
			return 0, 0, "", fmt.Errorf("update: unsupported scheme %q", u.Scheme)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return 0, 0, "", err
		}
		if have > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
		}
		resp, err := d.client().Do(req)
		if err != nil {
			return 0, 0, "", fmt.Errorf("update: fetch: %w", err)
		}
		defer resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			if have > 0 {
				// Server ignored Range: restart from zero.
				if _, err := tmp.Seek(0, io.SeekStart); err != nil {
					return 0, 0, "", err
				}
				if err := tmp.Truncate(0); err != nil {
					return 0, 0, "", err
				}
				have = 0
			}
			if total <= 0 {
				total = resp.ContentLength
			}
		case http.StatusPartialContent:
			if total <= 0 {
				if cr := resp.Header.Get("Content-Range"); cr != "" {
					if i := lastSlash(cr); i >= 0 {
						if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil {
							total = n
						}
					}
				}
			}
		default:
			return 0, 0, "", fmt.Errorf("update: server returned %s", resp.Status)
		}
		src = resp.Body
	}

	// Hash the full staged file: re-hash prefix when resuming.
	h := sha256.New()
	if have > 0 {
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return 0, 0, "", err
		}
		if _, err := io.CopyN(h, tmp, have); err != nil {
			return 0, 0, "", err
		}
	}
	if _, err := tmp.Seek(have, io.SeekStart); err != nil {
		return 0, 0, "", err
	}
	done := have
	start := time.Now()
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return done, total, "", ctx.Err()
		default:
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			if done+int64(n) > d.maxBytes() {
				return done, total, "", fmt.Errorf("update: artifact exceeds %d bytes", d.maxBytes())
			}
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return done, total, "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if d.OnProgress != nil {
				elapsed := time.Since(start).Seconds()
				_ = elapsed
				if !d.OnProgress(done, total) {
					return done, total, "", context.Canceled
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return done, total, "", fmt.Errorf("update: fetch: %w", rerr)
		}
	}
	return done, total, hex.EncodeToString(h.Sum(nil)), nil
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
