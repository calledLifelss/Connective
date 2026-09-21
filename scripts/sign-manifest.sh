#!/bin/bash
# Skeleton: sign manifest.json with the OPERATOR key (0600, never committed).
# Usage: scripts/sign-manifest.sh <dir/manifest.json> <private-key-file> <key-id>
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=/dev/null
. "$ROOT/scripts/go-env.sh"
go_run "$ROOT/backend/cmd/update-tool" sign --manifest "$1" --private "$2" --key-id "$3" --out "$1"
