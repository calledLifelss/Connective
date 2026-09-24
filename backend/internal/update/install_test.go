package update

import (
	"archive/zip"
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

// writeZipWithDirEntry mimics `zip -r full.zip <version>/`, which
// always emits an explicit top-folder entry ("0.9.9/"). That entry
// once defeated top-folder stripping and nested the tree a level.
func writeZipWithDirEntry(t *testing.T, dir, top string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, "full.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if _, err := zw.Create(top + "/"); err != nil {
		t.Fatal(err)
	}
	for n, c := range files {
		w, err := zw.Create(top + "/" + n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFullTopFolderDirEntryStripped(t *testing.T) {
	root := t.TempDir()
	in := &Installer{Root: root}
	nested := writeZipWithDirEntry(t, t.TempDir(), "0.9.9", map[string]string{
		"connective": "bin",
		"data/x":     "1",
	})
	dir, err := in.Assemble(context.Background(), "0.9.9", nested, ArtifactFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "connective")); string(got) != "bin" {
		t.Fatalf("top folder not stripped: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "0.9.9")); !os.IsNotExist(err) {
		t.Fatal("nested top folder must not survive stripping")
	}
}

func TestPromoteStaged(t *testing.T) {
	stage := t.TempDir()
	stagedVer := filepath.Join(stage, "versions", "0.9.9")
	writeTree(t, stagedVer, map[string]string{"connective": "bin", "data/x": "1"})
	root := t.TempDir()
	if err := PromoteStaged(stagedVer, root, "0.9.9"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "versions", "0.9.9", "connective")); string(got) != "bin" {
		t.Fatalf("promoted tree wrong: %q", got)
	}
	// Idempotent: second promotion keeps the tree.
	if err := PromoteStaged(stagedVer, root, "0.9.9"); err != nil {
		t.Fatal(err)
	}
	// Missing staged tree and bad versions fail honestly.
	if err := PromoteStaged(filepath.Join(stage, "nope"), root, "0.9.9"); err == nil {
		t.Error("missing staged tree must fail")
	}
	if err := PromoteStaged(stagedVer, root, "../evil"); err == nil {
		t.Error("unsafe version must fail")
	}
}
