# Phase 5 report — release hardening + real-world QA + polish

Date: 2026-09-21. Host: Fedora 44 / KDE Plasma / Wayland.
Backend architecture untouched; all changes are additive with
regression coverage.

## 1. Baseline (start of phase)

Go 16 pkgs OK · flutter analyze (infos only) · flutter test 14/14 ·
e2e-30s PASS 14s · e2e-failover PASS 28s.

## 2. Icon

User-supplied `~/Desktop/ConnectiveIcon.jpg` adopted as master
(`packaging/icons/connective.jpg` + hicolor 16–256 PNGs +
`frontend/assets/tray.png`). Placeholder geometric SVG removed from
the package. Rendering verified on the live desktop (spectacle shot:
real text, correct theme).

## 3. Privilege flow (§3)

- No credentials stored anywhere (repo-wide scan: only the log
  redaction marker list matches).
- Elevation-failure path tested live (forced failing runner):
  friendly error in 7s, machine in `error` (never wedged), retry
  behaves identically, no helper processes left behind.
- Success path (interactive polkit dialog) NOT auto-tested — requires
  a human approving a root prompt; documented, not faked.
- Timeout guard (20s) on every helper invocation (regression: an
  unbounded pkexec wait once wedged disconnect).

## 4. Failure testing (§6–9)

- Crash semantics: parent-death signal in core.Manager + helper
  run-core; verified live (SIGKILL daemon → core reaped, restart
  reattaches, disconnect clean). No orphans by construction.
- IPC death: UI marks backend lost (no stale "Connected"), banner +
  Retry → reconnect() restores (integration test kills/restarts the
  real daemon).
- Subscriptions: unreachable/garbage/empty/partial/dupe nodes all
  preserve working data (backend dedupe added + tests).
- Server import: malformed/unsupported rejected with friendly errors.
- Auto QA: scoring invariant fixed (failed can never outrank unknown),
  flap guard + revival checks; e2e-failover settles first try.

## 5. UI/UX (§11–23)

- Independent subscription collapse + inline server expansion kept;
  AnimatedSize replaces AnimatedCrossFade (untestable in this SDK).
- No fake latency (n/a until tested); real-data audit clean.
- Dashboard/backend-lost banner, error banners everywhere, empty
  states (subs/servers/search/logs), per-row test spinners.
- Notifications implemented (connect/degraded/switching/error,
  update results; configurable; silent on failure).
- Tray: Show/Quit, minimize-to-tray, tooltips; autostart toggle.
- Settings grouped (Connection/Auto/Appearance/Notifications/
  Advanced); every control persists through the backend except the
  two UI-local prefs (notifications, collapse), persisted via ui state.
- Goldens: dashboard disconnected/connected, servers expanded.

## 6. Coexistence (§5)

v2rayN ran throughout all proxy-mode testing without interference;
TUN work stayed in namespaces. Host default route untouched
(metric-100 DHCP preserved). No host firewall modifications outside
namespaces. Live interface-flap and host-TUN-takeover tests deferred
(would disrupt the user's live VPN) — recovery code is unit-covered
(debounced watcher, cooldown, double-probe).

## 7. Security (§28–29)

IPC socket 0600, store/config 0600, dirs 0700; TrustedBinary gate on
elevated helper/core paths (+ tests); argv-only exec (no shells);
untrusted subscription input validated; secrets redacted (tested);
kill-switch fails closed; UI+daemon unprivileged always. Deps audited
(flutter: MIT×3, BSD×1, all used; Go backend stdlib-only) and
recorded in THIRD_PARTY_NOTICES.md (tray legacy-bridge deprecation
noted for future migration).

## 8. Performance (§30–31)

Backend <1s cold start; app 27ms to process; RSS ~159MB app / ~8MB
daemon; 500-node subscription updates + renders + searches in ~2s
(backend ms-level, widget render measured in-test).

## 9. Packaging (§26–27, §34–36)

`connective-0.2.0-1.fc44.x86_64.rpm` (in `packaging/`): install →
launch from /usr/bin → desktop entry + hicolor icons + licenses →
remove, all verified clean. Issues fixed: plugin RPATHs (patchelf),
sing-box missing build-id (debug_package disabled). AppImage
deferred (RPM is primary). Branding consistently "Connective".

## 10. Final regression (§39)

go test 16 pkgs ✓ · flutter test 17 ✓ · analyze clean ✓ ·
e2e-30s ✓ · e2e-failover ✓ · drive acceptance ✓ · hygiene verified
(no procs/routes/tables/sockets left; host 200 OK).

## 11. Remaining limitations (honest)

Interactive polkit approval, live interface-flap, host TUN takeover
vs live VPN, desktop notify bubbles, AppImage, thousand-scale UI
stress beyond 500 rows, Xray/custom-outbound renderers.

## 12. Addendum 2026-09-21 — foreign-TUN safe-fail hardening (this session)

Review found the previous foreign-TUN change was warning-only:
`platform.OtherTunInterfaces()` was reported via `state.get`
(`foreignTun`) and shown as a dashboard banner, but `connectLocked`
never blocked, so the "fail safely" banner text was aspirational.
Also `runHelper` had no timeout despite §3 claiming a 20s guard,
and `tun-cleanup` accepted any `--if` value.

Fixed (minimal, additive, no architecture change):

- `platform/tuncheck.go`: detection now uses kernel `tun_flags`
  (name-independent) plus name heuristics extended to
  nordlynx/proton; container/bridge plumbing
  (docker/veth/virbr/br-/lxcbr) explicitly excluded; new
  `CheckSafeForTun()` returns the stable user-facing error
  "Another VPN/TUN interface is active (…)… cannot safely
  initialize its TUN connection…".
- `cmd/connectived/main.go` `connectLocked`: when TUN is enabled, a
  foreign tunnel aborts the connect before core start (state →
  error, warn log, friendly error to UI). Proxy-only mode unaffected.
- `cmd/connectived/main.go` `runHelper`: 20s `CommandContext`
  timeout on every helper call (pkexec cancel can no longer wedge
  disconnect); run-core path untouched (own startup timeout).
- `cmd/connective-helper/main.go` `tunCleanup`: whitelisted to
  `connective0` only — any other `--if` is refused, so foreign
  interfaces can never be deleted by mistake. Daemon only ever
  passes `connective0` (both disconnect paths verified).
- `frontend/lib/state/app_store.dart` `friendlyError`: the
  "Another VPN/TUN…" backend error is shown verbatim (truncated)
  instead of being collapsed into the generic TUN message.
- Regression tests: `tuncheck_test` (noise exclusions, heuristics,
  stable message) + new `helper/main_test.go` (foreign refusal,
  missing flag).

Verification in THIS environment: Go/Flutter toolchains are absent,
so `go test`/`flutter test`/e2e/GUI/polkit/live-VPN runs were NOT
re-executed here — code review + static checks only (brace balance,
import presence, caller audit). Must be re-run on the build machine:
`go test ./...`, `go vet`, `gofmt`, `flutter analyze/test`,
`tests/e2e-30s.sh`, `tests/e2e-failover-90s.sh`, plus the real GUI
polkit approve/cancel, kill-switch, Auto/failover, and RPM
install/launch/remove flows before release.
