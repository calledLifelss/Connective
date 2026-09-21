package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func validManifest() Manifest {
	return Manifest{
		Schema:       ManifestSchema,
		Version:      "0.2.1",
		Channel:      ChannelStable,
		ReleaseDate:  "2026-09-21",
		ReleaseNotes: []string{"Faster Auto", "TUN recovery"},
		MinVersion:   "0.2.0",
		Platform:     PlatformLinux,
		Arch:         ArchX8664,
		Artifacts: []Artifact{
			{Type: ArtifactFull, Filename: "full.zip", Size: 100, SHA256: "ab" + repeat("cd", 31), URL: "full.zip"},
		},
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestManifestValid(t *testing.T) {
	if err := validManifest().Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestManifestInvalid(t *testing.T) {
	base := validManifest()
	cases := []struct {
		name string
		mut  func(*Manifest)
	}{
		{"schema", func(m *Manifest) { m.Schema = 99 }},
		{"version", func(m *Manifest) { m.Version = "bogus" }},
		{"channel", func(m *Manifest) { m.Channel = "nightly" }},
		{"platform", func(m *Manifest) { m.Platform = "" }},
		{"arch", func(m *Manifest) { m.Arch = "" }},
		{"no artifacts", func(m *Manifest) { m.Artifacts = nil }},
		{"delta w/o from", func(m *Manifest) {
			m.Artifacts = append(m.Artifacts, Artifact{Type: ArtifactDelta, Filename: "d.zip", Size: 10, SHA256: repeat("ab", 32), URL: "d.zip"})
		}},
		{"traversal", func(m *Manifest) { m.Artifacts[0].Filename = "../evil" }},
		{"bad hash", func(m *Manifest) { m.Artifacts[0].SHA256 = "xyz" }},
		{"bad size", func(m *Manifest) { m.Artifacts[0].Size = 0 }},
	}
	for _, c := range cases {
		m := base
		m.Artifacts = append([]Artifact(nil), base.Artifacts...)
		c.mut(&m)
		if err := m.Validate(); err == nil {
			t.Errorf("%s: invalid manifest accepted", c.name)
		}
	}
}

func TestMatchQuery(t *testing.T) {
	m := validManifest()
	q := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
	if err := MatchQuery(m, q); err != nil {
		t.Fatalf("applicable release rejected: %v", err)
	}
	// Same version → quiet no-update, not an error surface.
	q.CurrentVersion = "0.2.1"
	if err := MatchQuery(m, q); !IsNoUpdate(err) {
		t.Errorf("same version should be no-update, got %v", err)
	}
	for _, tc := range []struct {
		name string
		mut  func(*Query)
	}{
		{"channel", func(q *Query) { q.Channel = ChannelBeta }},
		{"platform", func(q *Query) { q.Platform = PlatformWindows }},
		{"arch", func(q *Query) { q.Arch = ArchAARCH64 }},
	} {
		qq := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
		tc.mut(&qq)
		if err := MatchQuery(m, qq); !IsNoUpdate(err) {
			t.Errorf("%s: expected no-update, got %v", tc.name, err)
		}
	}
}

func TestDirProviderNoUpdate(t *testing.T) {
	pub, priv, id := testKeys(t)
	dir := t.TempDir()
	m := validManifest()
	m.Version = "0.2.0" // same as caller
	fdir := fixtureDir(t, m, priv, id)
	_ = dir
	p := DirProvider{Dir: fdir}
	q := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
	if _, err := p.Check(context.Background(), q); !IsNoUpdate(err) {
		t.Fatalf("expected no-update, got %v", err)
	}
	_ = pub
}

func TestDirProviderMissingManifest(t *testing.T) {
	p := DirProvider{Dir: t.TempDir()}
	q := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
	if _, err := p.Check(context.Background(), q); err == nil {
		t.Fatal("missing manifest should fail")
	}
}

func TestDirProviderMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := DirProvider{Dir: dir}
	q := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
	if _, err := p.Check(context.Background(), q); err == nil {
		t.Fatal("malformed manifest should fail")
	}
}

func TestGitHubStub(t *testing.T) {
	p := GitHubProvider{}
	q := Query{Channel: ChannelStable, Platform: PlatformLinux, Arch: ArchX8664, CurrentVersion: "0.2.0"}
	if _, err := p.Check(context.Background(), q); err != ErrNotConfigured {
		t.Fatalf("unconfigured github must report not-configured, got %v", err)
	}
	if _, err := p.OpenArtifact(context.Background(), Artifact{}); err != ErrNotConfigured {
		t.Fatalf("unconfigured github open must report not-configured, got %v", err)
	}
}
