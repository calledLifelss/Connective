#!/bin/bash
# Skeleton: re-verify signature (+ caller checks hashes via manifest).
# Usage: scripts/verify-update.sh <signed-manifest> <keys.json>
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=/dev/null
. "$ROOT/scripts/go-env.sh"
go_run "$ROOT/backend/cmd/update-tool" verify --manifest "$1" --keys "$2"
