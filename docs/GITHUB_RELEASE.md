# GitHub release record

Repository: `git@github.com:calledLifelss/Connective.git`
(`https://github.com/calledLifelss/Connective`)

## Versions

| Train | Tag | Version | Channel | Status |
|---|---|---|---|---|
| stable | `v0.2.0` | 0.2.0 (RPM `0.2.0-2`) | stable | assets built + signed locally, awaiting publish |
| test | `v0.2.1-test.1` | 0.2.1-test.1 (RPM `0.2.1-0.test.1`) | beta | assets built + signed locally, awaiting publish |

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

Required secrets (name + purpose; values never recorded):

- `UPDATE_SIGNING_KEY` — hex seed of the Ed25519 release private key,
  written 0600 into the runner and shredded after signing.
- `UPDATE_KEY_ID` — signing key id (`connective-release-1`).

Provision (maintainer, from the machine holding the key):

```bash
gh secret set UPDATE_SIGNING_KEY --repo calledLifelss/Connective \
  --body "$(cat /path/to/signing.key)"
gh secret set UPDATE_KEY_ID --repo calledLifelss/Connective \
  --body "connective-release-1"
```

## Updater verification (no GitHub publish rights in this session)

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

## Remaining limitations

1. **Publishing needs rights this session's credential lacks.**
   Required follow-ups by the maintainer:
   - repository description/topics (web UI or a token with
     Administration scope);
   - `UPDATE_SIGNING_KEY` / `UPDATE_KEY_ID` secrets (commands above);
   - creating the releases (web UI, or with a token that has
     Contents scope):
     ```bash
     gh release create v0.2.0 --title v0.2.0 \
       --notes-file docs/RELEASE_NOTES_v0.2.0.md \
       connective-0.2.0-2.fc44.x86_64.rpm full-0.2.0.zip \
       update-manifest.json update-manifest.json.sig SHA256SUMS
     ```
   - re-running the tag workflows once secrets exist.
2. **No live self-replacement of `/opt` yet.** The updater engine,
   versioned layout, and rollback are implemented and proven on real
   trees, but activating into the root-owned RPM layout needs the
   planned polkit-gated step (see `docs/UPDATE_ARCHITECTURE.md`).
   Until then, `update.install` on production stages + verifies and
   hands activation to `connective-updater`.
3. **CI desktop integration/e2e** run locally (display + loopback
   harness), not in GitHub Actions; see test reports in this doc's
   history and `docs/E2E_REPORT.md`.
