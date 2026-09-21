#!/bin/bash
# Skeleton: assemble a versioned release bundle layout under dist/.
# Publishes nothing. Usage: scripts/build-release.sh <version>
set -u
VER="${1:?usage: build-release.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist/connective-$VER"
mkdir -p "$OUT/versions/$VER"
echo "bundle layout: $OUT (populate versions/$VER from the RPM/bundle build, then generate-manifest.sh)"
