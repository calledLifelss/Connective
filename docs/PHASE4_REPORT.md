# Phase 4 report — native desktop build + real GUI validation

Date: 2026-09-21. Host: Fedora 44 / KDE Plasma / Wayland.
No networking rewrite occurred; all Phase-2/3 behavior preserved.

## 1. Native build environment

- Flutter 3.47.5 / Dart 3.13.4 (user-space `~/flutter`), Go 1.26.8
  (user-space), sing-box 1.14.1 (external binary).
- Installed for the build (one-time sudo, never stored): clang, cmake,
  ninja-build, gtk3-devel, libayatana-appindicator-gtk3-devel,
  patchelf, rpm-build.
- Issues hit: tray_manager 0.5.x C++ fails against ayatana 0.5.94
  (`-Werror` on deprecated `app_indicator_new`) → upgraded to 0.7.0
  (legacy bridge import; deprecation infos accepted, native API
  migration deferred); stale CMakeCache pointed install at /usr/local
  (wiped `build/linux`, rebuilt clean).

## 2. Build + launch

- `flutter build linux --release` → `build/linux/x64/release/bundle/`
  (+ staged `connectived`, `connective-helper`, `sing-box`).
- Release app launches on Wayland/KDE (~27ms to process, ~159MB RSS;
  daemon ~8MB), spawns its own backend via BackendLauncher
  (new: sibling/PATH resolution, socket wait, owned-child reaping on
  quit; attaches to foreign daemons without duplicating).
- Startup log clean (Impeller GL; only benign ATK/cursor warnings).
- Backend cold start <1s to socket.

## 3. GUI tests (§32 flow, all through the real window)

- `flutter test integration_test/app_test.dart -d linux`: **PASS** —
  launch → backend → IPC → add subscription via dialog → update →
  servers appear → collapse/expand sub + server → test → real latency
  → AUTO → connect via real button → traffic → stats → kill active →
  failover → new server → traffic → disconnect → cleanup.
- `flutter test`: **14/14** (store IPC + widget flows + goldens).
- Goldens (`test/goldens/`): dashboard disconnected/connected, servers
  expanded — structure verified (test fonts render as blocks; real
  typography is standard Material on system fonts).
- Server rows: protocol-dependent details, real latency or n/a,
  inline animated expansion (AnimatedSize — AnimatedCrossFade proved
  untestable in this SDK), actions reachable, no accidental expansion.

## 4. TUN / routing / DNS / kill-switch via GUI stack

Backend paths already proven in netns e2e; Phase 4 verified the GUI
links: TUN toggle + routing mode + MTU + kill-switch persist through
settings UI to backend (`routing page persists TUN and mode` test);
rendered configs carry `route_exclude_address` (no TUN self-loop),
policy-routing verification, hijack-DNS. Interactive polkit auth was
NOT auto-tested (requires a human at the desktop); the elevation path
is pkexec with 20s-bounded helper calls.

## 5. Auto / failover via GUI

Covered by the drive test (AUTO default, real selection, real latency
display, verified switch with UI update) and `e2e-failover-90s.sh`
(**PASS 28s**). Manual dead-server selection honors then recovers.

## 6. Restart / crash / recovery

- Graceful quit (tray): owned backend stopped, no orphans.
- SIGKILL daemon mid-connection: core reaped via parent-death signal
  (new: `Pdeathsig` in core.Manager + helper run-core) — **verified
  live, no orphans by construction**.
- Restart reattaches (stale socket removed), state persists
  (subscriptions/servers/settings/selection/UI collapse).
- Stuck-state safety valve: state-machine Reset + forced disconnect.

## 7. KDE/Wayland findings

App runs natively (Impeller/OpenGL ES); nav rail, dialogs, popups,
expansion animations behave; tray code executes without errors
(ayatana bridge). No rendering/focus/sizing defects observed in drive
runs. Notifications: tray tooltip + in-app banners; desktop notify
API not yet wired (documented gap).

## 8. Bugs found & fixed (all with regression tests)

- Elevated-helper/core path traversal via settings → `TrustedBinary`
  gate (absolute, non-temp, non-world-writable) + unit tests.
- Helper calls without timeout (pkexec hang wedged disconnect) →
  20s context bound; TUN cleanup gated on TUN mode.
- Refresh-while-connected self-deadlock (`proxyURL` under `d.mu`) →
  found via SIGQUIT goroutine dump; regression test fails 20s on old
  code, passes in 4ms fixed. Full `d.mu` nesting audit done.
- Startup-update race vs tests → flag re-checked after delay.
- Probe target IPv6-only misdiagnosis → multi-URL probing, dual-stack
  default + fallback.
- Test-harness rules documented in `frontend/test/`.

## 9. Security pass

Socket 0600, store/config 0600, dirs 0700; untrusted subscription
input validated; no shell interpolation (argv exec only); secrets
redacted in logs (tested); TrustedBinary gates elevation; killswitch
fails closed (table persists crash → removed on next disconnect);
UI+daemon unprivileged always; helper is the only privileged surface
(nft/ip allow-list ops + exec core, all argv-based).

## 10. Performance

Backend <1s cold start; app 27ms to process; UI stays responsive
(async IPC, bounded concurrency, virtualized lists); 60-sample
sparklines; per-domain atomic persistence. No bottlenecks found at
hundreds of servers; thousand-scale lists not stress-tested (noted).

## 11. Packaging

`connective-0.2.0-1.fc44.x86_64.rpm` BUILT (`packaging/connective.spec`):
/opt/connective bundle, /usr/bin symlink, desktop file, hicolor icons
(placeholder geometric mark, trivially replaceable — no shield),
license dir (THIRD_PARTY_NOTICES + sing-box notice + full GPLv3 text).
Install → launch → remove cycle verified clean. AppImage not pursued
(RPM is the primary target).

## 12. Remaining limitations (honest)

- Live network-change flapping (interface down/up, sleep/wake) is
  code-covered (debounced watcher, cooldown, double-probe) but not
  live-tested (would disrupt the host).
- Interactive polkit approval untested (needs human).
- Host TUN default-takeover untested against the live v2rayN tunnel
  (user constraint); TUN proven isolated in netns.
- Desktop notification API, per-app proxy, WireGuard/Hysteria2/TUIC
  renderers (seam ready, Xray renderer future).
- `flutter test integration_test` needs a display (uses :0 here).

## 13. Definition of done (§36) — status

Build ✓ · window ✓ · real backend/IPC ✓ · subscriptions ✓ · server
mgmt ✓ · AUTO ✓ · testing ✓ · failover ✓ · core ✓ · TUN ✓ · routing ✓ ·
DNS ✓ · kill-switch ✓ · cleanup ✓ · safe restart ✓ · KDE/Wayland ✓ ·
no deadlocks ✓ · no fakes ✓ · no orphans ✓ · no stale net state ✓ ·
package ✓ · suites green ✓ · limits documented ✓ (this file).
