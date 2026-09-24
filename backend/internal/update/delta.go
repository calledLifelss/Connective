package update

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DeltaApplier turns (installed tree at sourceVersion + delta artifact)
// into a complete target tree in outDir. Implementations must produce a
// FULL verified installation — never a partial overlay.
type DeltaApplier interface {
	Apply(ctx context.Context, currentDir, deltaPath, outDir string) error
}

// Extraction safety caps: a signed manifest guarantees the bytes came
// from the release operator, but a corrupt/malicious zip must still not
// fill the disk. Totals are enforced across all members (headers can lie,
// so the copy itself is bounded, not just UncompressedSize64).
const (
	maxUnzipTotalBytes = 1 << 30 // 1 GiB expanded per artifact
	maxUnzipFiles      = 10000
)

// unzipBudget tracks extraction across one artifact.
type unzipBudget struct {
	remaining int64
	files     int
}

func newUnzipBudget() *unzipBudget { return &unzipBudget{remaining: maxUnzipTotalBytes} }

// ZipOverlayApplier is the honest reference implementation used by
// tests and tooling: the delta is a zip of changed files that overlays
// a copy of the current tree. Production may swap in a binary-diff
// applier behind this same interface (e.g. bsdiff-style) without
// touching the manager, installer, or UI. A test delta is never
// presented as a production mechanism.
type ZipOverlayApplier struct{}

// Apply copies currentDir to outDir, then overlays the zip contents.
// Zip-slip paths are rejected; context cancellation aborts.
func (ZipOverlayApplier) Apply(ctx context.Context, currentDir, deltaPath, outDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := copyDir(currentDir, outDir); err != nil {
		return fmt.Errorf("update: delta base copy: %w", err)
	}
	zr, err := zip.OpenReader(deltaPath)
	if err != nil {
		return fmt.Errorf("update: bad delta: %w", err)
	}
	defer zr.Close()
	budget := newUnzipBudget()
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := unzipOneBudgeted(f, outDir, budget); err != nil {
			return err
		}
	}
	return nil
}

func unzipOne(f *zip.File, outDir string) error {
	return unzipOneBudgeted(f, outDir, newUnzipBudget())
}

func unzipOneBudgeted(f *zip.File, outDir string, budget *unzipBudget) error {
	if filepath.IsAbs(f.Name) {
		return fmt.Errorf("update: unsafe delta path %q", f.Name)
	}
	rel := filepath.Clean(f.Name)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("update: unsafe delta path %q", f.Name)
	}
	target := filepath.Join(outDir, rel)
	if f.FileInfo().IsDir() {
		budget.files++
		if budget.files > maxUnzipFiles {
			return fmt.Errorf("update: too many files in artifact")
		}
		return os.MkdirAll(target, 0o755)
	}
	budget.files++
	if budget.files > maxUnzipFiles {
		return fmt.Errorf("update: too many files in artifact")
	}
	// Header size is advisory only; enforce against the live copy.
	if f.UncompressedSize64 > uint64(budget.remaining) {
		return fmt.Errorf("update: artifact expands beyond %d bytes", int64(maxUnzipTotalBytes))
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	// Bound the copy itself: headers can lie, so LimitReader+1 detects
	// overflow even when UncompressedSize64 is spoofed.
	n, err := io.Copy(out, io.LimitReader(rc, budget.remaining+1))
	if cerr := out.Close(); cerr != nil && err == nil {
		return cerr
	}
	if err != nil {
		return err
	}
	if n > budget.remaining {
		os.Remove(target)
		return fmt.Errorf("update: artifact expands beyond %d bytes", int64(maxUnzipTotalBytes))
	}
	budget.remaining -= n
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode()&0o777)
		if err != nil {
			return err
		}
		_, cerr := io.Copy(out, in)
		if err := out.Close(); err != nil {
			return err
		}
		return cerr
	})
}
