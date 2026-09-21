// Package persistence stores Connective's durable state: subscriptions,
// servers, settings, selection and UI state. Each domain has its own file
// so one corrupt or busy document can never take down the rest, and every
// write is atomic (temp file + rename) so crashes cannot leave half
// documents behind.
package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store is a directory of JSON documents with per-document locking.
type Store struct {
	dir string
	mu  sync.Mutex
}

// New creates (or opens) a store rooted at dir.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("persistence: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Save marshals v to doc atomically.
func (s *Store) Save(doc string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("persistence: marshal %s: %w", doc, err)
	}
	tmp, err := os.CreateTemp(s.dir, doc+".*.tmp")
	if err != nil {
		return fmt.Errorf("persistence: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("persistence: write %s: %w", doc, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("persistence: write %s: %w", doc, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("persistence: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(s.dir, doc+".json")); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("persistence: rename %s: %w", doc, err)
	}
	return nil
}

// Load unmarshals doc into v. A missing document is not an error; v keeps
// its defaults and ok=false is returned.
func (s *Store) Load(doc string, v any) (ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(s.dir, doc+".json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("persistence: read %s: %w", doc, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, fmt.Errorf("persistence: corrupt %s: %w", doc, err)
	}
	return true, nil
}

// Document names owned by each domain.
const (
	DocSubscriptions = "subscriptions"
	DocServers       = "servers"
	DocSettings      = "settings"
	DocSelection     = "selection"
	DocUIState       = "ui_state"
	DocTestResults   = "test_results"
	DocUpdateCache   = "update_cache"
)
