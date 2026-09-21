#!/bin/bash
# Resumable Flutter SDK fetch. Safe to re-run; skips when done.
URL="https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_3.47.5-stable.tar.xz"
DEST="$HOME/flutter-sdk.tar.xz"
MARK="$HOME/flutter-sdk.done"
[ -f "$MARK" ] && exit 0
for i in $(seq 1 40); do
  curl -L --retry 2 -C - -o "$DEST" "$URL" && [ $(stat -c%s "$DEST") -gt 500000000 ] && { touch "$MARK"; exit 0; }
  sleep 5
done
exit 1
