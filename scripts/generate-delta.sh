#!/bin/bash
# Skeleton: zip delta of changed files between two installed trees.
# Usage: scripts/generate-delta.sh <from-tree> <to-tree> <out-delta.zip>
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=/dev/null
. "$ROOT/scripts/go-env.sh"
go_run "$ROOT/backend/cmd/update-tool" delta --from "$1" --to "$2" --out "$3"
