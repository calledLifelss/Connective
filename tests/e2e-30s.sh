#!/bin/bash
# e2e-30s: bounded connect/traffic/disconnect test. Proxy-only (no TUN,
# no nft, no privileges, loopback only). Self-terminating: hard timeout
# plus trap cleanup. Must finish in ~30s.
set -u
TIMEOUT=${TIMEOUT:-60}
REALHOME="$HOME"
LAB_BIN="$REALHOME/.local/share/connective/lab-bin"
LAB="$REALHOME/.local/share/connective/lab"
SB="$(find "$LAB_BIN" -maxdepth 2 -name sing-box -type f | head -1)"
REPO="$REALHOME/Desktop/Connective"
E2E="$REPO/tests/e2e-lab"
RUN="$(mktemp -d /tmp/e2e30.XXXXXX)"
freeport() { python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()"; }
export HOME="$RUN" CONNECTIVE_SOCK="$RUN/.local/share/connective/connectived.sock"
SS1P=$(freeport) SS2P=$(freeport) SUBP=$(freeport) MIXP=$(freeport) CLASHP=$(freeport)

cleanup() {
  kill "$D" "$S1" "$S2" "$SUB" 2>/dev/null
  sleep 2
  kill -9 "$D" "$S1" "$S2" "$SUB" 2>/dev/null
  # Belt and braces: exact RUN-dir patterns only (never broad names).
  python3 "$E2E/pkill.py" "$RUN" 2>/dev/null
  rm -rf "$RUN"
}
trap cleanup EXIT
fail() {
  echo "FAIL: $1"
  cp -r "$RUN" "/tmp/e2e30-fail-$(date +%s)" 2>/dev/null
  exit 1
}

ipc() {
  if [ $# -ge 2 ]; then python3 "$E2E/e2e_ipc.py" "$1" "$2"; else python3 "$E2E/e2e_ipc.py" "$1"; fi
}
state_of() { ipc state.get | python3 -c "import json,sys; print(json.load(sys.stdin)['state'])"; }

sed "s/18388/$SS1P/" "$LAB/ss1.json" > "$RUN/ss1.json"
sed "s/18389/$SS2P/" "$LAB/ss2.json" > "$RUN/ss2.json"
B64="YWVzLTEyOC1nY206Y29ubmVjdGl2ZS1lMmUtdGVzdA=="
printf "ss://%s@127.0.0.1:%s#S1\nss://%s@127.0.0.1:%s#S2\n" "$B64" "$SS1P" "$B64" "$SS2P" > "$RUN/sub.txt"
export CONNECTIVE_LAB="$RUN" CONNECTIVE_SUBPORT="$SUBP"

"$SB" check -c "$RUN/ss1.json" > /dev/null || fail "bad ss config"
"$SB" run -c "$RUN/ss1.json" > "$RUN/ss1.log" 2>&1 < /dev/null & S1=$!
"$SB" run -c "$RUN/ss2.json" > "$RUN/ss2.log" 2>&1 < /dev/null & S2=$!
python3 "$E2E/subserve.py" > "$RUN/sub.log" 2>&1 < /dev/null & SUB=$!
sleep 2
"$LAB_BIN/connectived" > "$RUN/daemon.log" 2>&1 < /dev/null & D=$!
sleep 3

[ "$(ipc ping | python3 -c "import json,sys; print(json.load(sys.stdin)['version'])")" = "1" ] || fail "ping"
ipc settings.update "{\"autoMode\":true,\"routingMode\":\"global\",\"tunEnabled\":false,\"killSwitch\":false,\"mixedPort\":$MIXP,\"testTimeoutMs\":4000,\"updateOnStart\":false,\"urlTestIntervalMin\":10,\"connectionTestUrl\":\"http://connectivitycheck.gstatic.com/generate_204\",\"clashApiPort\":$CLASHP,\"corePath\":\"$SB\",\"mtu\":9000,\"testConcurrency\":5,\"healthIntervalSec\":30}" > /dev/null || fail "settings"
ipc subscriptions.add "{\"name\":\"T\",\"url\":\"http://127.0.0.1:$SUBP/sub\"}" > /dev/null || fail "sub add"
[ "$(ipc subscriptions.update '{"id":""}' | python3 -c "import json,sys; print(json.load(sys.stdin)['updated'])")" = "1" ] || fail "sub update"
[ "$(ipc servers.list | python3 -c "import json,sys; print(len(json.load(sys.stdin)))")" = "2" ] || fail "server count"
[ "$(ipc connection.connect '{"serverId":"auto"}' | python3 -c "import json,sys; print(json.load(sys.stdin)['state'])")" = "connected" ] || fail "connect"
sleep 4
[ "$(ipc state.get | python3 -c "import json,sys; print(json.load(sys.stdin)['state'])")" = "connected" ] || fail "not connected"
code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 -x http://127.0.0.1:$MIXP http://example.com/)
[ "$code" = "200" ] || fail "traffic via proxy: $code"
down=$(ipc stats.get | python3 -c "import json,sys; print(json.load(sys.stdin)['downTotal'])")
[ "$down" -gt 0 ] || fail "no traffic stats"
ipc connection.disconnect > /dev/null || fail "disconnect"
sleep 2
[ "$(state_of)" = "disconnected" ] || fail "not disconnected"
pgrep -f "core.json" > /dev/null && fail "core process leaked" || true
echo "PASS e2e-30s (traffic=$code down=$down)"
