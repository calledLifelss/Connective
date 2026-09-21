# Update architecture

Self-update foundation: backend engine + UI skeleton. GitHub Releases
is the future source; nothing in the manager, verification, staging,
installation, rollback, or UI knows how GitHub works.

## Layers

```
Flutter UI (banner/dialog/progress/settings)
    ↓  update.* IPC (docs/IPC_CONTRACT.md), event.update
Go UpdateManager (backend/internal/update/manager.go)
    ↓  ReleaseProvider interface (provider.go)
GitHubReleaseProvider (stub) · DirProvider (tests/dev)
    ↓
DownloadManager → staging → verify → InstallationManager
    ↓  activation
connective-updater (separate process) → versioned layout
```

## Components (one concern per file, stdlib only)

| File | Owns |
|---|---|
| `version.go` | numeric version compare, channels, downgrade/minimum policy |
| `manifest.go` | v1 signed-manifest schema + structural validation |
| `provider.go` | `ReleaseProvider` interface, `Query`, `MatchQuery` |
| `local_provider.go` | `DirProvider`: deterministic fixtures (never production) |
| `github_provider.go` | skeleton returning `ErrNotConfigured` until the repo exists |
| `state.go` | centralized lifecycle states + legal transitions |
| `manager.go` | check/download/install/cancel/dismiss, cache, backoff |
| `download.go` | streaming fetch, resume, retry, caps, wire hashing |
| `verify.go` | SHA-256 + Ed25519 manifest trust (`TrustedKeys`) |
| `artifacts.go` | delta-vs-full selection (smallest safe wins) |
| `delta.go` | `DeltaApplier` + honest zip-overlay reference impl |
| `install.go` | versioned trees, atomic symlink swap, rollback, health |

## State machine

Idle → Checking → NoUpdate | UpdateAvailable → Downloading →
Verifying → UpdateAvailable (verified, staged) | Staging → Installing →
Restarting → Updated; Cancelled/Failed/RolledBack re-enter Checking.
Impossible transitions are rejected (`Transition`). Events are emitted
synchronously in emission order — a late stale snapshot must never
overwrite newer state (regression-tested).

## Versioned installation layout (target model)

```
<root>/versions/0.2.0/…   immutable installed trees
<root>/versions/0.2.1/…
<root>/current -> versions/0.2.1   (rename = atomic activation)
<root>/previous -> versions/0.2.0  (rollback anchor)
<root>/staging/…                   scratch, wiped per run
```

Windows uses the same model under `%ProgramFiles%\Connective`, with
`current.txt` (version pointer, atomically replaced) alongside the
trees until the launcher stub lands; full artifacts tolerate a single
top-level folder either way. Per-platform manifests
(`update-manifest-windows.json`, fallback `update-manifest.json`) let
one release serve both OSes. See `docs/WINDOWS.md`.

## RPM coexistence (no breakage today)

The current Fedora package keeps its flat `/opt/connective/` layout.
The updater code is capable of the versioned model NOW, and the RPM
adopts it in a later step by one of:

1. `%post` migration: copy the installed tree to
   `/opt/connective/versions/<ver>/`, create the `current` symlink,
   keep `/opt/connective/connective` as a symlink into it; or
2. a `connective-versions` subpackage owning the new layout while the
   main package stays a compatibility shim for one release.

No migration runs until the GitHub provider + production signing land.
`connectived` itself never self-replaces: activation is always the
updater process after the app exits.

## Privileges

The updater inherits the app's helper philosophy: no root runtime, no
stored passwords, paths confined under the install root (`..` rejected
in versions, filenames, and delta entries), provider-supplied URLs
never become filesystem locations. `/opt` writes remain a polkit-gated
operation (same as TUN today), executed by the updater process only.
