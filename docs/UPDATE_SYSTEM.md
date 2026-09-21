# Update system (user + operator view)

## User experience

Connective opens → background check (cached; at most every 6h, manual
always allowed) → small Dashboard banner only when an update applies:

> Connective 0.2.1 is available · 5.2 MB · [Update now] [Later]

Update now → details dialog (current → new, notes, size, type) →
Downloading (real bytes/percent/speed/ETA) → Verifying… → Install now →
Installing… → Restarting… → Updated. Failures speak plainly and never
touch the working install:

- offline: "Couldn't check for updates. We'll try again later."
- bad signature/hash: "This update could not be verified and was not
  installed."
- install failure: "The update could not be installed. Your current
  version is still safe."

Dismissing (Later) hides the banner until a NEWER version appears;
Settings → UPDATES holds version, channel (stable/beta/dev),
auto-check, manual check, and last-checked time.

## Operator: cutting a release (skeleton, no GitHub yet)

```bash
scripts/build-release <version>        # bundle layout under dist/
scripts/generate-manifest ...          # manifest.json with sha256+sizes
scripts/sign-manifest ...              # signed manifest (operator key)
scripts/generate-delta --from A --to B # delta.zip of changed files
scripts/verify-update ...              # signature + hash re-check
```

Implementation: thin wrappers over `go run ./cmd/update-tool`
(`gen-key|manifest|sign|verify|delta`). They publish nothing today;
after the repository exists they gain upload steps without changing
contracts.

## Delta policy

The selector prefers the smallest safe artifact: a delta sourced at
exactly the running version wins when smaller than full; otherwise
full. Invalid/foreign-version/non-beneficial deltas fall back to full,
which always exists. Test deltas are zip overlays (see `delta.go`) —
a documented reference, not a production binary-diff claim.

## GitHub transition (post-task checklist)

1. Create the repository (user step — not done here).
2. Set `GitHubProvider{Owner, Repo}` + implement release/asset fetch.
3. Embed the production public key; store the private key as a CI
   secret; sign manifests in Actions.
4. Publish a test release; run the customer flow against it.
