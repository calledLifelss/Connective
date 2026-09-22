// Command update-tool establishes the release-automation contracts:
// gen-key, manifest generation, manifest signing/verification and delta
// packaging. It publishes nothing; GitHub wiring comes later. Private
// keys are operator files (0600) — never committed, never embedded.
//
// Usage:
//
//	update-tool gen-key --private key.pem --public pubkey.hex [--key-id ID]
//	update-tool manifest --version V --channel C --platform P --arch A \
//	  --notes "a;b" --min-version M --artifact type:file[:from] ... --out manifest.json
//	update-tool sign --manifest manifest.json --private key.pem --key-id ID --out signed.json
//	update-tool verify --manifest signed.json --keys pubkeys.json
//	update-tool delta --from TREE-VERSION-DIR --to TREE-DIR --out delta.zip
package main

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connective/backend/internal/update"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "update-tool:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: update-tool <gen-key|manifest|sign|verify|delta>")
	}
	switch args[0] {
	case "gen-key":
		return genKey(args[1:])
	case "manifest":
		return genManifest(args[1:])
	case "sign":
		return signManifest(args[1:])
	case "verify":
		return verifyManifest(args[1:])
	case "delta":
		return genDelta(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func genKey(args []string) error {
	fs := flag.NewFlagSet("gen-key", flag.ContinueOnError)
	priv := fs.String("private", "", "output private key file (0600)")
	pub := fs.String("public", "", "output public key hex file")
	keyID := fs.String("key-id", "test-key-1", "key id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *priv == "" || *pub == "" {
		return fmt.Errorf("private and public outputs required")
	}
	pubK, privK, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*priv, []byte(hex.EncodeToString(privK.Seed())), 0o600); err != nil {
		return err
	}
	out := map[string]string{"key_id": *keyID, "public": hex.EncodeToString(pubK)}
	raw, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(*pub, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("key %s written (private 0600 — keep it out of the repo)\n", *keyID)
	return nil
}

func loadPriv(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("bad private key file: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("bad private key length")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func genManifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	version := fs.String("version", "", "release version")
	channel := fs.String("channel", "stable", "stable|beta|dev")
	platform := fs.String("platform", "linux", "platform")
	arch := fs.String("arch", "x86_64", "architecture")
	notes := fs.String("notes", "", "release notes separated by ;")
	minV := fs.String("min-version", "", "minimum supported version")
	date := fs.String("date", "", "release date (YYYY-MM-DD)")
	assetBase := fs.String("asset-base", "", "absolute URL prefix for artifacts (release download base)")
	out := fs.String("out", "manifest.json", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	m := update.Manifest{
		Schema:       update.ManifestSchema,
		Version:      *version,
		Channel:      *channel,
		ReleaseDate:  *date,
		MinVersion:   *minV,
		Platform:     *platform,
		Arch:         *arch,
		ReleaseNotes: splitNotes(*notes),
	}
	for _, spec := range fs.Args() {
		// type:file[:from]; file may carry a url override via =url suffix:
		//   full:bundle.zip  delta:patch.zip:0.2.0
		// Windows drive letters contain a colon (C:\...), so the split
		// is drive-aware (see splitSpec).
		typ, file, from, err := splitSpec(spec)
		if err != nil {
			return err
		}
		parts := []string{typ, file}
		if from != "" {
			parts = append(parts, from)
		}
		sum, size, err := hashFile(parts[1])
		if err != nil {
			return err
		}
		a := update.Artifact{
			Type:     parts[0],
			Filename: filepath.Base(parts[1]),
			Size:     size,
			SHA256:   sum,
			URL:      filepath.Base(parts[1]),
		}
		if base := strings.TrimSuffix(*assetBase, "/"); base != "" {
			a.URL = base + "/" + a.Filename
		}
		if len(parts) == 3 {
			a.FromVersion = parts[2]
		}
		m.Artifacts = append(m.Artifacts, a)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("manifest for %s (%d artifacts) -> %s\n", m.Version, len(m.Artifacts), *out)
	return nil
}

// splitSpec parses "type:file[:from]", tolerating Windows drive
// letters (C:\...) in the file part. A trailing :from_version is only
// recognized when it parses as a version; anything else stays part of
// the filename (loud hash failure later if truly wrong).
func splitSpec(spec string) (typ, file, from string, err error) {
	i := strings.Index(spec, ":")
	if i <= 0 {
		return "", "", "", fmt.Errorf("bad artifact spec %q (want type:file[:from])", spec)
	}
	typ, rest := spec[:i], spec[i+1:]
	start := 0
	if len(rest) >= 3 && rest[1] == ':' && isDriveLetter(rest[0]) &&
		(rest[2] == '\\' || rest[2] == '/') {
		start = 3
	}
	if j := strings.LastIndex(rest[start:], ":"); j >= 0 {
		if _, verr := update.ParseVersion(rest[start+j+1:]); verr == nil {
			return typ, rest[:start+j], rest[start+j+1:], nil
		}
	}
	return typ, rest, "", nil
}

func isDriveLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func splitNotes(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ";") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func signManifest(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	in := fs.String("manifest", "", "unsigned manifest.json")
	priv := fs.String("private", "", "operator private key file")
	keyID := fs.String("key-id", "", "signing key id")
	out := fs.String("out", "manifest.signed.json", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	var m update.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("bad manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	pk, err := loadPriv(*priv)
	if err != nil {
		return err
	}
	sm, err := update.SignManifest(m, *keyID, pk)
	if err != nil {
		return err
	}
	outRaw, _ := json.MarshalIndent(sm, "", "  ")
	if err := os.WriteFile(*out, append(outRaw, '\n'), 0o644); err != nil {
		return err
	}
	// Detached signature sidecar for external verifiers and the
	// update-manifest.json.sig release asset.
	if err := os.WriteFile(*out+".sig", []byte(sm.Signature+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("signed %s with %s -> %s (+ .sig)\n", m.Version, *keyID, *out)
	return nil
}

func verifyManifest(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	in := fs.String("manifest", "", "signed manifest file")
	keys := fs.String("keys", "", "trusted keys json {key_id: hexpub, ...}")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sm, err := update.ParseSignedManifest(raw)
	if err != nil {
		return err
	}
	kraw, err := os.ReadFile(*keys)
	if err != nil {
		return err
	}
	var hexKeys map[string]string
	if err := json.Unmarshal(kraw, &hexKeys); err != nil {
		return fmt.Errorf("bad keys file: %w", err)
	}
	pubs := map[string]ed25519.PublicKey{}
	for id, hx := range hexKeys {
		b, err := hex.DecodeString(strings.TrimSpace(hx))
		if err != nil {
			return fmt.Errorf("bad key %q: %w", id, err)
		}
		if len(b) != ed25519.PublicKeySize {
			return fmt.Errorf("bad key %q length", id)
		}
		pubs[id] = ed25519.PublicKey(b)
	}
	tk := update.TrustedKeys{Keys: pubs}
	if err := tk.VerifyManifest(sm); err != nil {
		return err
	}
	fmt.Printf("manifest %s signature OK (key %s)\n", sm.Manifest.Version, sm.KeyID)
	return nil
}

func genDelta(args []string) error {
	fs := flag.NewFlagSet("delta", flag.ContinueOnError)
	from := fs.String("from", "", "installed tree of the source version")
	to := fs.String("to", "", "target tree dir")
	out := fs.String("out", "delta.zip", "output delta zip")
	if err := fs.Parse(args); err != nil {
		return err
	}
	changed, err := diffTrees(*from, *to)
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		return fmt.Errorf("no differences between trees")
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, rel := range changed {
		data, err := os.ReadFile(filepath.Join(*to, rel))
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	sum, size, err := hashFile(*out)
	if err != nil {
		return err
	}
	fmt.Printf("delta %s (%d files, %d bytes, sha256 %s)\n", *out, len(changed), size, sum[:16])
	return nil
}

// diffTrees lists files under to/ that are new or differ from from/.
func diffTrees(from, to string) ([]string, error) {
	var out []string
	err := filepath.Walk(to, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(to, path)
		if err != nil {
			return err
		}
		old, rerr := os.ReadFile(filepath.Join(from, rel))
		if rerr != nil {
			out = append(out, rel) // new file
			return nil
		}
		cur, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h1 := sha256.Sum256(old)
		h2 := sha256.Sum256(cur)
		if h1 != h2 {
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}
