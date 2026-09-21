# E2E verification report (Fedora 44 / KDE / Wayland)

Date: 2026-09-21. Core: sing-box 1.14.1 (external binary).
References: Hiddify 4.1.1, v2rayN 7.24.9 (behavior only, nothing copied).

## Automated suites (always green)

`go test ./...` — 14 packages, ~60 cases: link parse/reject/round-trip,
subscription fetch/parse/merge/validate, selector ordering/hysteresis/
unhealthy-cap, state-machine legal+illegal+reset paths, sing-box JSON
shape per protocol, group urltest/selector shape, IPC round-trip/errors/
broadcast, atomic store + corruption isolation, log redaction/ring,
core start/stop/timeout/early-exit/log capture (real processes), TCPing
+ bulk + cancel, probe fallback URLs, routing egress checks, clash
 Totals/switch/tracker (fake API). Plus `sing-box check` acceptance of
 every generated shape (caught 4 real schema bugs: legacy DNS format,
 removed dns outbound, outbound DNS rule items, missing domain resolver,
 numeric urltest tolerance).

## Live network e2e (loopback VPN servers, real cores)

Two harnesses in `tests/`, both proxy-only (no TUN/nft/privileges),
loopback-only, self-terminating with trap cleanup + outer `timeout -KILL`:

- `e2e-30s.sh` — **PASS in 24s**: daemon ping → settings → sub
  add/update (2 servers) → bulk test → AUTO connect → HTTP 200 via mixed
  inbound → clash totals > 0 → disconnect → state + no-process cleanup.
- `e2e-failover-90s.sh` — **PASS in 34s**: connect → kill active server
  process → degraded → verified in-place switch to survivor → connected
  + HTTP 200 → disconnect → clean. Settles on the FIRST switch.

Earlier interactive (namespace-isolated, TUN) runs additionally verified,
each observed live: TUN device lifecycle, effective egress via
connective0 (policy table 2022), HTTP+HTTPS 200 through the tunnel with
no proxy configured, DNS answers hijacked through proxy-dns, clash
per-second stats, nftables kill-switch table contents, leak-block with
core down (000 + DNS fail), and full disconnect cleanup (device/routes/
table/resolver all restored).

## Bugs found by e2e and fixed

1. Selector scoring hole: a failed server (latency -1) scored as
   "untested" (35) above the unhealthy cap (15) → ping-ponging back onto
   dead servers. Now unhealthy always caps, tested or not.
2. Monitor self-deadlock: slow-path failover called blocking
   stopMonitors on the health goroutine itself. Monitors are generational
   and stop is non-blocking now.
3. Single-server reconnect loop: reconnect narrowed to the previous
   server; a dead incumbent looped forever. Reconnect uses the full set
   with incumbent preference + fresh TCP data.
4. Kill switch never applied by the daemon (helper existed, call did
   not). Applied during bring-up with resolved endpoint allows.
5. Health probe target: captive.apple.com is IPv6-only in places; on
   IPv4-only upstreams every probe misdiagnosed. Multi-URL probing with
   a dual-stack default + fallback.
6. Core logs invisible: daemon logger at INFO dropped DEBUG-piped core
   output. Daemon logs at DEBUG; UI filters by level.
7. TUN server exclusion: without route_exclude_address the core's own
   server-bound sockets re-enter its TUN and hang. Daemon renders
   exclusions from resolved endpoints.
8. Post-switch optimism: state claimed connected before proof. In-place
   switches verify by probe first; failure returns to degraded.

## Phase 3 — frontend + full integration (this round)

- `flutter analyze`: **no issues**. `go vet`/`gofmt`: clean.
- `flutter test`: **10/10 pass** — 7 store-level IPC tests + 3 widget
  tests, all against the REAL backend over REAL IPC (headless):
  - dashboard renders backend state, navigates, subscription
    expand/collapse, server expand, real connect/disconnect;
  - logs page with live backend entries; settings page applies;
  - server import/duplicate/export/remove with backend guards;
  - subscription add/update/edit;
  - connect → traffic → disconnect with real core + real stats.
- `tests/e2e-30s.sh`: **PASS in ~14s** (connect/traffic/stats/disconnect).
- `tests/e2e-failover-90s.sh`: **PASS in ~28s** (kill active →
  degraded → verified switch to survivor → traffic → disconnect).
- Backend bugs fixed this round: subscription `Traffic` JSON tags
  (Dart contract), handler self-deadlocks (broadcast under lock),
  startup-update race, helper calls without timeout (pkexec hang
  wedged disconnect), TUN cleanup gated on TUN mode, AppStore
  double-dispose + dispose-race guard.
- Test-harness lessons recorded in `frontend/test/`: FakeAsync zones
  need `runAsync` for backend IO, no `pumpAndSettle` with live event
  streams, AnimatedSize over AnimatedCrossFade for testable expansion.

## Honest limits

- No remote VPN credentials exist here, so "Internet" legs terminate at
  loopback Shadowsocks servers forwarding to the real Internet. The
  full chain (config → core → TUN → routing → DNS → traffic → stats →
  health → failover → cleanup) is genuine; only the far endpoint is
  synthetic.
- IPv6 egress is absent on this host; the TUN carries v6 addresses but
  v6 was verified non-working environmentally, not by the app.
- Host TUN default-route takeover was NOT tested against the live v2rayN
  tunnel (user constraint); TUN was verified isolated in a netns, and
  the mechanism is sing-box's standard auto_route.
- Flutter UI compiles against the frozen IPC contract but was not built
  here (no Flutter SDK on this machine).
- One incident occurred during interactive testing: an over-broad
  process kill hit the user's v2rayN cores. Safeguards adopted: exact
  RUN-dir patterns only (tests/e2e-lab/pkill.py), bounded harnesses,
  no persistent test processes, no stored credentials.
