#!/bin/bash
# Skeleton: hash artifacts into manifest.json. Usage:
#   scripts/generate-manifest.sh <dir> <version> <channel> <platform> <arch> [notes...]
# Example: scripts/generate-manifest.sh dist/0.3.0 0.3.0 stable linux x86_64 "Note one" "Note two"
set -u
DIR="${1:?}"; VER="$2"; CH="$3"; PLAT="$4"; ARCH="$5"; shift 5
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=/dev/null
. "$ROOT/scripts/go-env.sh"
ABS="$(cd "$DIR" && pwd)"
ARTS=()
for f in "$ABS"/*.zip; do ARTS+=( "full:$f"); done
# shellcheck disable=SC2068
go_run "$ROOT/backend/cmd/update-tool" manifest --version "$VER" --channel "$CH" --platform "$PLAT" --arch "$ARCH" --notes "$(IFS=';'; echo "$*")" --out "$ABS/manifest.json" ${ARTS[@]}
