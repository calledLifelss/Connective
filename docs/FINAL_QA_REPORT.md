# FINAL QA REPORT — release verification on the build machine (2026-09-21)

Host: Fedora 44 / KDE Plasma / Wayland (`wayland`, `DISPLAY=:0`).
Toolchains: Go 1.26.8 (`~/.local/golang-root/usr/lib/golang/bin`),
Flutter 3.47.5 / Dart 3.13.4 (`~/flutter/flutter/bin`),
clang 22.1.8, cmake 4.3.0, ninja 1.13.2, pkg-config 2.5.1.
Core: sing-box 1.14.1. Package: `connective-0.2.0-1.fc44.x86_64.rpm`
(rebuilt this session from current tree; payload verified to contain
the new safety fixes via `strings`).

Live host condition: the user's own `singbox_tun` VPN was UP for the
whole session — used as the real foreign tunnel for the safety test
and left untouched throughout.

## 1. Build — PASS

- `go build ./...` OK; `flutter build linux --release` OK
  (`build/linux/x64/release/bundle/connective`).

## 2. Static/unit — PASS (with two minimal test-revealed fixes)

- `go test ./...`: all packages OK (incl. new `connective-helper`
  whitelist test + extended `platform` tests).
- `go vet ./...`: clean. `gofmt -l .`: clean (one whitespace-only
  fix in `tuncheck.go` required and applied).
- `flutter analyze`: no errors (10 pre-existing tray-bridge
  deprecation infos, migration already deferred in PHASE5).
- `flutter test`: **17/17 PASS**. One initial failure —
  `golden dashboard disconnected` (42.9% pixel diff) — root-caused
  to environment, not code: the Lab backend reports the live host's
  `foreignTun=['singbox_tun']`, so the warning banner rendered into
  a layout golden recorded without it. Fix: the golden now clears
  `store.foreignTun` before pumping (commented as host-state
  hermeticity; banner logic remains covered by the dashboard
  conditional + `OtherTunInterfaces` unit tests). Re-ran: all green.
  No test weakened (no assertions removed).

## 3. GUI integration — PASS

- `flutter test integration_test -d linux`: **All tests passed**
  (real window on this KDE/Wayland host: subscription add/update,
  collapse/expand, test, AUTO connect, traffic, failover, disconnect).

## 4. Backend E2E — PASS

- `tests/e2e-30s.sh`: **PASS** (traffic=200).
- `tests/e2e-failover-90s.sh`: **PASS** (active killed → degraded →
  verified switch to survivor → traffic=200). Covers Auto selection,
  health monitoring, failover, traffic restore, cleanup.

## 5. Foreign-TUN safety — PASS (real foreign tunnel, real backend)

With `singbox_tun` UP (foreign VPN active), fresh `connectived`
built from the current tree, loopback servers, TUN enabled:

- `state.get` reports `foreignTun=['singbox_tun']`.
- `connection.connect` is **blocked before core start** with the
  exact message: "Another VPN/TUN interface is active
  (singbox_tun). Connective cannot safely initialize its TUN
  connection in the current network state. …".
- Machine stays `disconnected` (no wedged transient; consistent with
  the pre-existing "no servers" path — there is no
  disconnected→error edge, and the IPC error is the UI channel via
  the error banner). Retry immediately possible.
- Verified untouched: `singbox_tun` still UP, no `connective0`
  created, default route unchanged
  (`default via 192.168.1.1 dev enp7s0 … metric 100`), no nft
  changes (unprivileged; kill-switch off), no core process started.
- Proxy-only mode (TUN off) with the foreign VPN up: connect →
  traffic 200 → disconnect, all clean (coexistence allowed where
  safe).
- Post-test: foreign VPN intact, host Internet 200.

## 6. Privilege-failure path — PASS (automated); interactive
approve/cancel — HUMAN-DEPENDENT, not executed

- Forced failing elevation runner + `killSwitch:true` (forces the
  elevated core path): connect fails in **2s** (well under the new
  20s `CommandContext` guard) with a friendly error, machine in
  `error` (never wedged), retry behaves identically, no orphan
  helper/core, no stale TUN.
- Interactive polkit APPROVE (real dialog → TUN/routes/DNS/traffic)
  and CANCEL/REJECT via the real dialog were **not executed**: they
  need a human at the desktop approving a root prompt, and the live
  TUN-takeover leg would require disconnecting the user's active
  VPN — deliberately not disrupted (same constraint as PHASE5 §6).
  The code path is identical to the verified failure path
  (helper error → friendly error → safe state → retry).

## 7. Networking / kill-switch live (root) — prior results stand

- Proxy-mode traffic verified live today (e2e + §5). TUN
  device/routing/DNS/kill-switch enforcement was verified
  isolated-in-netns in earlier phases; live host takeover vs the
  user's VPN deferred (see §6). No code in those paths changed
  except the additive foreign-TUN block + helper timeout.

## 8. Subscriptions / servers / UI — PASS via automated suites

- add/update/edit/duplicate/import/export/test, failure
  preservation, collapse/expand, persistence: covered by `flutter
  test` (store + widget) and the passing integration drive test.
- No fake data found (grep audit; only `ErrNotImplemented` stubs
  and "never faked" doc comments).

## 9. Desktop — PARTIAL (automated part PASS)

- Release-mode app window driven live on KDE/Wayland via the
  integration test: PASS. Tray minimize/restore, autostart toggle,
  notification bubbles remain manually verified per PHASE5 with the
  known gap (desktop notify API not wired; in-app banners + tray
  tooltips used).

## 10. Package — PAYLOAD VERIFIED, system install needs root

- Fresh RPM rebuilt: `~/rpmbuild/RPMS/x86_64/` → copied to
  `packaging/connective-0.2.0-1.fc44.x86_64.rpm` (32.4 MB).
- Payload verified via `rpm2cpio`: `/opt/connective` layout,
  755 binaries, `/usr/bin/connective` symlink, desktop entry, all
  hicolor icons, licenses, sing-box 1.14.1, post/postun icon-cache
  scripts. New binaries confirmed by `strings` (old RPM lacks the
  new messages).
- System `dnf install → launch → connect → disconnect → remove →
  reinstall` was **not executed**: no passwordless sudo in this
  shell and interactive root auth is human-dependent. Structure is
  unchanged from the PHASE5-verified package (same spec, only
  binary contents refreshed).

## 11. Host cleanliness — PASS

- No orphan connectived/helper/core processes; two pre-existing
  orphaned loopback `subserve.py` test servers found and reaped by
  exact PID; no `connective0`; default route intact; no stale
  sockets in real HOME; host Internet 200 after every suite.

## Remaining limitations (honest)

1. Interactive polkit APPROVE + CANCEL via the real dialog.
2. Live host TUN-takeover / kill-switch enforcement vs a real VPN
   (netns-verified; live-deferred to protect the user's VPN).
3. System-wide RPM install/launch/remove (payload verified;
   spec unchanged since PHASE5's verified install).
4. Tray minimize/restore + notification bubbles manual pass;
   desktop notify API still unwired (known gap).
5. Thousand-scale UI stress beyond 500 rows; AppImage; Xray renderers.

## 12. Addendum 2026-09-21 ~18:00 — live elevated-disonnect hang
(found while debugging the user's installed app, fixed same session)

Symptoms on the installed RPM: after a successful elevated (TUN)
connect, Disconnect wedged the UI on "Disconnecting..." and the
backend stopped answering (client timeout → "Backend unavailable").

Root cause: the core runs as ROOT (via pkexec helper) while the
daemon runs as the user. `core.Manager.Stop()` sent SIGTERM/SIGKILL
(EPERM, silently ignored) then waited on `<-m.done` with NO timeout
after Kill — forever. The first live elevated-disconnect ever
attempted; all prior disconnect tests ran root-daemon (netns) or
proxy-mode where signals work.

Fix (minimal, additive):

- `core.Manager.Stop()`: 5s bounded reap-wait after SIGKILL, then
  proceed instead of hanging; new `Pid()` accessor.
- New helper `stop-core --pid N` (root): refuses missing/bad pids,
  pid 1, its own pid, and any live pid whose cmdline is not a core
  (PID-reuse guard); already-gone = success; TERM, 5s grace, KILL.
- Daemon `stopCore()` now calls `ensureCoreGone(pid)`: reaped → done;
  surviving PID → `runHelper("stop-core", ...)` (20s-bounded).
  Covers disconnect (both branches), failover and reconnect, which
  share stopCore.
- Live relief verified same session: daemon SIGKILL → Pdeathsig
  SIGTERM reaped the root core by itself (connective0 removed,
  default route intact, host 200) → fresh daemon → clean
  `disconnected` state.
- Regression tests (all green): core `TestPidLifecycle`,
  `TestStopBoundedOnIgnorantChild` (TERM-ignoring child),
  connectived `TestProcessGone` (+ prior sibling-binary tests),
  helper `TestStopCore{RequiresPid,RefusesBadPids,RefusesSelf,
  GonePidSucceeds,RefusesLiveForeignProcess}` (leaves the decoy
  alive). The true EPERM case cannot be simulated unprivileged and
  was verified live by the incident itself.
- RPM rebuilt with both this fix and the sibling-sing-box fix
  (`packaging/connective-0.2.0-1.fc44.x86_64.rpm`, payload symbols
  verified). Needs `sudo dnf reinstall` + app restart on the user's
  machine, then a live connect (approve) → disconnect (approve) to
  close the loop.
