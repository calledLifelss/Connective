# GitHub release record

Repository: `git@github.com:calledLifelss/Connective.git`
(`https://github.com/calledLifelss/Connective`)

## Versions

| Train | Tag | Version | Channel | Status |
|---|---|---|---|---|
| stable | `v0.2.0` | 0.2.0 (RPM `0.2.0-2`) | stable | published |
| test | `v0.2.1-test.1` | 0.2.1-test.1 (RPM `0.2.1-0.test.1`) | beta | published |
| prerelease | `v0.3.0-beta.1` | 0.3.0-beta.1 (RPM `0.3.0-0.beta.1`) | beta | published (Windows x64 train, installer + updater) |
| stable | `v0.3.0` | 0.3.0 (RPM `0.3.0-1`) | stable | published: RPM, Windows setup, full zips, signed per-platform manifests |
| stable | `v0.3.1` | 0.3.1 (RPM `0.3.1-1`) | stable | published: finishing updater (restart-to-activate), versioned /opt layout |
| stable | `v0.4.0` | 0.4.0 (RPM `0.4.0-1`) | stable | published: consolidated dashboard UI |
| stable | `v0.5.0` | 0.5.0 (RPM `0.5.0-1`) | stable | published: per-app split tunneling + updater hardening (RPM, full zip, signed manifest; no Windows exe — windows job failed on Window-only-untestable app tests) |
| stable | `v0.5.1` | 0.5.1 (RPM `0.5.1-1`) | stable | published: updater repair (pkexec spawn, stage promotion, zip strip, cancel-from-restarting) + Windows CI fix (RPM, Windows setup.exe, per-platform full zips + signed manifests) |
| stable | `v6.0` | 6.0 (RPM `6.0-1`) | stable | published: full-width dashboard + live traffic graph, routing reconnect prompts, Windows updater repair (RPM, Windows setup.exe, per-platform full zips + signed manifests) |

Version is coherent across Flutter pubspec, Go `CurrentVersion`,
RPM spec, manifest, and tag (CI `version guard` enforces this).

## Trust

- Release key id: `connective-release-1` (Ed25519).
- Public key embedded in the app
  (`backend/internal/update/trusted_keys.go`) and published in
  `packaging/update-pubkeys.json`.
- Private key: operator-held, never in the repo, never in assets.

## CI status

- `ci.yml` (push/PR): backend gofmt/vet/test + Flutter
  analyze/test. Caught real issues on first run (gofmt drift,
  fatal-infos); green after fixes.
- `release.yml` (tags `v*`): test → build (Go + Flutter) → RPM →
  full/delta artifacts → manifest → sign → verify → publish.
  Runs live on the pushed tags; stops at the signing step until the
  secrets below exist. Delta step degrades to full-only when no stable
  release exists yet.

Required secrets (provisioned; values never recorded):

- `UPDATE_SIGNING_KEY` — hex seed of the Ed25519 release private key,
  written 0600 into the runner and shredded after signing.
- `UPDATE_KEY_ID` — signing key id (`connective-release-1`).

To rotate, generate a new pair with `update-tool gen-key`, embed the
new public key (`trusted_keys.go` + `packaging/update-pubkeys.json`),
and overwrite the secrets.

## Updater verification

Live anonymous discovery against `api.github.com` succeeds and an
empty repo reports quiet no-update (no auth needed by design).

End-to-end with the real signed bytes (50 MB full, 11 MB delta),
production trust roots, temp install root:

- beta discovery of `0.2.1-test.1`, delta selected (11 223 562 B vs
  50 010 510 B shown to the user);
- download → SHA-256 verified → assembled over the real 0.2.0 tree →
  activated → health-checked → updated;
- delta-assembled tree byte-identical to the full tree;
- dead target → automatic rollback to the previous version;
- tampered manifest / bad hash / corrupt / missing asset → rejected,
  installation untouched.

UI flow (banner → dialog → download → install → updated) is covered
against signed local fixtures in `frontend/test/update_test.dart`.

## CI validation history

The pipeline earned its keep during this task, catching in live runs:
gofmt drift, info-level analyze failures, untracked update fixtures
(`.gitignore` swallowing `testdata/`), missing lab binaries, workflow
step-ordering bugs, RPM hyphen-version tarball naming, and a missing
updater copy step. `ci` is green; `release` progressed stage by stage
to manifest/sign/publish.

## Final state (2026-09-21)

- `v0.2.0` stable published with RPM, full artifact, signed manifest
  (+ detached `.sig`) and SHA256SUMS; round-trip downloaded and
  re-verified.
- `v0.2.1-test.1` prerelease (beta) **built, signed, and published by
  the CI pipeline itself** after `UPDATE_SIGNING_KEY` / `UPDATE_KEY_ID`
  secrets were provisioned: RPM, full (49.6 MB), delta (13.8 MB),
  manifest, sig. Signature re-verified against the embedded trust root.
- Live updater proof with the real releases: beta discovery of
  `0.2.1-test.1`, delta selected (13 799 565 B shown), download →
  SHA-256 verified → assembled over a real 0.2.0 tree → activated →
  health-checked → updated; `previous` anchor retained.
- First-generation path proven too: with no versioned base, the delta
  is skipped and the full artifact is fetched automatically.
- Failure paths (bad signature, tampered manifest, hash mismatch,
  corrupt artifact, invalid delta, missing asset) reject with the
  installation untouched — via daemon IPC tests and the Go suite.
- Rollback proven on real trees (dead target → previous version
  active, data preserved).
- Full regression green after all changes: `go test ./...`, `flutter
  analyze`, `flutter test` (37), `integration_test -d linux`,
  `e2e-30s`, `e2e-failover`.
- Production private key shredded after provisioning; CI holds the
  only usable copy as `UPDATE_SIGNING_KEY`.

## Remaining limitations

1. **Social preview image**: GitHub offers no API for the repository
   social image — upload `packaging/icons/ConnectiveBanner.png` at
   Settings → Social preview in the web UI. (The same banner already
   heads the README.)
2. **No live self-replacement of `/opt` yet.** The updater engine,
   versioned layout, and rollback are implemented and proven on real
   trees, but activating into the root-owned RPM layout needs the
   planned polkit-gated step (see `docs/UPDATE_ARCHITECTURE.md`).
   Until then, `update.install` on production stages + verifies and
   hands activation to `connective-updater`.
3. **CI desktop integration/e2e** run locally (display + loopback
   harness), not in GitHub Actions; see test reports in this doc's
   history and `docs/E2E_REPORT.md`.
