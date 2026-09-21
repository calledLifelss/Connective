# Shared Go resolver for release scripts: PATH first, then known
# user-space/system installs. Provides go_run() for update-tool calls.
# shellcheck disable=SC2148
_GO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_GO_BIN="$(command -v go || true)"
if [ -z "$_GO_BIN" ]; then
  for _c in "$HOME/.local/golang-root/usr/lib/golang/bin/go" \
           "$HOME/.local/golang-root/bin/go" \
           /usr/local/go/bin/go /usr/lib/golang/bin/go; do
    if [ -x "$_c" ]; then _GO_BIN="$_c"; break; fi
  done
fi
unset _c
go_run() {
  if [ -z "$_GO_BIN" ]; then
    echo "go toolchain not found" >&2
    return 1
  fi
  # update-tool lives in the backend module: run from the module root
  # so package resolution works regardless of caller CWD.
  (cd "$_GO_ROOT/backend" && \
    GOFLAGS="${GOFLAGS:-}" GOPROXY="${GOPROXY:-off}" GOTOOLCHAIN=local \
    "$_GO_BIN" run "$@")
}
