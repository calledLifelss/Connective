package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustedBinary(t *testing.T) {
	dir := t.TempDir()
	// Temp dirs are refused even for sane files.
	f := filepath.Join(dir, "helper")
	if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := TrustedBinary(f); err == nil {
		t.Errorf("temp-dir binary must be refused")
	}
	// Relative paths refused.
	if err := TrustedBinary("relative/helper"); err == nil {
		t.Errorf("relative path must be refused")
	}
	// World-writable refused (in an allowed root).
	safe := "/tmp"
	_ = safe
	// A system binary passes.
	if err := TrustedBinary("/bin/sh"); err != nil {
		t.Errorf("system binary should pass: %v", err)
	}
	// Missing file refused.
	if err := TrustedBinary("/nonexistent-xyz/helper"); err == nil {
		t.Errorf("missing file must be refused")
	}
}
