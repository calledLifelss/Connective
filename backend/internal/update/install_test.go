package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFullAssembleActivate(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	ctx := context.Background()

	// Full artifact: complete target tree zipped.
	stage := t.TempDir()
	fullZip, _ := writeZip(t, stage, map[string]string{
		"connective": "new-binary", "data/x": "1",
	})
	dir, err := in.Assemble(ctx, "0.2.1", fullZip, ArtifactFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "connective")); string(got) != "new-binary" {
		t.Fatal("assembled tree incomplete")
	}
	if in.Current() != "" {
		t.Fatal("current must be unset before activation")
	}
	prev, err := in.Activate("0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "" || in.Current() != "0.2.1" {
		t.Fatalf("activation wrong (prev=%q cur=%q)", prev, in.Current())
	}
}

func TestFullNestedTopFolderStripped(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	ctx := context.Background()

	// Full artifacts built as `zip -r full.zip <version>/` carry one
	// top folder; assembly must land files at the tree root.
	// writeZip names its output payload.zip — reuse it directly.
	stage := t.TempDir()
	nested, _ := writeZip(t, stage, map[string]string{
		"0.9.9/connective": "bin",
		"0.9.9/data/x":     "1",
	})
	dir, err := in.Assemble(ctx, "0.9.9", nested, ArtifactFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "connective")); string(got) != "bin" {
		t.Fatalf("top folder not stripped: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "data", "x")); string(got) != "1" {
		t.Fatalf("nested file wrong: %q", got)
	}
}

func TestDeltaAssembleOverCurrent(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	ctx := context.Background()

	// Seed current 0.2.0 tree and activate.
	cur, _ := in.VersionDir("0.2.0")
	writeTree(t, cur, map[string]string{"connective": "v1", "keep": "k"})
	if _, err := in.Activate("0.2.0"); err != nil {
		t.Fatal(err)
	}
	// Delta changes one file.
	stage := t.TempDir()
	deltaZip, _ := writeZip(t, stage, map[string]string{"connective": "v2"})
	if _, err := in.Assemble(ctx, "0.2.1", deltaZip, ArtifactDelta, ZipOverlayApplier{}); err != nil {
		t.Fatal(err)
	}
	prev, err := in.Activate("0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "0.2.0" {
		t.Fatalf("rollback anchor wrong: %q", prev)
	}
	got, _ := os.ReadFile(filepath.Join(root, "current", "connective"))
	if string(got) != "v2" {
		t.Fatal("delta not applied through current link")
	}
	keep, _ := os.ReadFile(filepath.Join(root, "current", "keep"))
	if string(keep) != "k" {
		t.Fatal("delta must preserve untouched files")
	}
	// Rollback restores the previous tree.
	back, err := in.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if back != "0.2.0" || in.Current() != "0.2.0" {
		t.Fatalf("rollback wrong: %q/%q", back, in.Current())
	}
}

func TestDeltaZipSlipRejected(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	cur, _ := in.VersionDir("0.2.0")
	writeTree(t, cur, map[string]string{"ok": "1"})
	if _, err := in.Activate("0.2.0"); err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	evil, _ := writeZip(t, stage, map[string]string{"../escape": "x"})
	if _, err := in.Assemble(context.Background(), "0.2.1", evil, ArtifactDelta, ZipOverlayApplier{}); err == nil {
		t.Fatal("zip-slip delta must be rejected")
	}
	if in.Current() != "0.2.0" {
		t.Fatal("failed assemble must leave current untouched")
	}
}

func TestHealthTimeout(t *testing.T) {
	in := &Installer{Root: t.TempDir(), HealthTimeout: 300 * time.Millisecond}
	if err := in.AwaitHealthy(context.Background(), func() bool { return false }); err == nil {
		t.Fatal("never-healthy must time out")
	}
	if err := in.AwaitHealthy(context.Background(), func() bool { return true }); err != nil {
		t.Fatalf("healthy must pass: %v", err)
	}
}

func TestUnsafeVersionRejected(t *testing.T) {
	in := &Installer{Root: t.TempDir()}
	for _, v := range []string{"../x", "a/b", "", "bogus"} {
		if _, err := in.VersionDir(v); err == nil {
			t.Errorf("version %q must be rejected", v)
		}
	}
}

func TestRollbackEmpty(t *testing.T) {
	in := &Installer{Root: t.TempDir()}
	if _, err := in.Rollback(); err == nil {
		t.Error("rollback with no previous must fail")
	}
}
