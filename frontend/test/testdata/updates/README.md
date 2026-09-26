# Update fixtures

Signed local manifests for `update_test.dart`. The daemon runs with
`CONNECTIVE_UPDATE_PROVIDER=dir:<fixture>` and `CONNECTIVE_UPDATE_TEST_APPLY=1`,
so nothing here ever touches the network or the live install.

| fixture | manifest version | purpose |
|---|---|---|
| `has-update{,-win}` | `99.0.0` | banner → dialog → download → install |
| `hash-mismatch{,-win}` | `99.0.0` | artifact hash mismatch fails the download |
| `bad-sig{,-win}` | `99.0.0` | signature invalid (never re-sign these) |
| `no-update{,-win}` | `0.2.0` | candidate older than the app → quiet |

`keys.json` holds **public** halves only. `99.0.0` is deliberately far
ahead of the app version: a fixture older than `CurrentVersion` is
simply reported as `no-update`, which used to silently turn these tests
green-to-red the day the app version crossed the fixture (that is
exactly what happened at 6.0).

## Re-signing after a fixture edit

```bash
cd backend
go run ./cmd/update-tool gen-key --private /tmp/fk --public /tmp/fp --key-id test-key-1
# write {"test-key-1": <public hex>} into testdata/updates/keys.json
go run ./cmd/update-tool sign --manifest manifest.json \
  --private /tmp/fk --key-id test-key-1 --out ../frontend/test/testdata/updates/<fixture>/manifest.json
rm ../frontend/test/testdata/updates/<fixture>/manifest.json.sig
go run ./cmd/update-tool verify --manifest ... --keys .../keys.json
```

The private fixture key stays out of the repo (never commit `*.key`).
