<div align="center">
  <img src="packaging/icons/ConnectiveBanner.png" alt="Connective banner" />
  <p><strong>A modern desktop VPN/proxy client — Flutter UI, Go backend, sing-box core.</strong></p>
  <p>Linux first (Fedora / KDE Plasma / Wayland). No root runtime. Self-updating.</p>
</div>

---

## Overview

Connective is a production-grade desktop VPN/proxy client. A Flutter
UI drives a Go backend daemon over a versioned local IPC socket; the
daemon supervises **sing-box** for proxying, TUN, routing, and DNS.
All networking truth lives in the backend — the UI is presentation
only and never claims a state the daemon didn't report.

## Features

- **Real core integration** — drives sing-box as a supervised child
  process (start/stop/restart, no orphans).
- **TUN** — full-tunnel mode with foreign-tunnel conflict detection
  (fails safely instead of corrupting routes).
- **Routing** — global and rules modes applied through the core.
- **DNS** — system, custom, and proxy-aware modes with leak-aware
  handling.
- **Kill switch** — optional traffic lockdown around the tunnel.
- **Auto server selection** — scoring with hysteresis (never naive
  lowest-ping), plus one-tap manual selection.
- **Automatic failover** — health monitoring with recovery and
  server switching on degraded paths.
- **Subscriptions** — add/edit/update, traffic (`subscription-userinfo`)
  and expiry metadata, local-preserving merge, per-subscription refresh.
- **Server management** — import/export share links, edit, duplicate,
  delete, per-server latency testing, favorites, custom names.
- **Protocols** — VLESS, VMess, Trojan, Shadowsocks, SOCKS (plus
  WireGuard / Hysteria2 / TUIC model support), share-link parsing.
- **Traffic statistics** — live up/down rates, session totals, duration
  with a compact live graph.
- **Logging** — leveled, secret-redacted logs with ring buffer and
  viewer (filter, clear).
- **System tray** — minimize-to-tray, tray menu, desktop notifications
  for connect/failover/update events.
- **Notifications & startup** — optional login autostart and desktop
  alerts.
- **Self-updater** — checks GitHub Releases from inside the app with
  signed manifests, delta/full artifacts, staging, atomic activation,
  and rollback (see [Updates](#updates)).

## Architecture

```
Flutter UI (presentation only)
    ↓  versioned JSON-over-unix-socket IPC (docs/IPC_CONTRACT.md)
Go daemon (connectived): state machine, selector, tester, updater
    ↓  supervises
sing-box core + connective-helper (polkit, privileged TUN/firewall ops)
```

- `backend/` — Go, stdlib-only: connection lifecycle, subscriptions,
  selector, config generation, tester, core supervisor, IPC,
  persistence, logging, settings, update engine.
- `frontend/` — Flutter: dashboard home (connect + subscriptions +
  servers on one page), management pages, routing, settings, logs.
- `packaging/` — Fedora RPM (flat `/opt/connective` layout), icons,
  desktop entry, license notices.
- `scripts/` — release automation skeleton (manifest/sign/delta/verify).

Key docs: `docs/ARCHITECTURE.md`, `docs/IPC_CONTRACT.md`,
`docs/UPDATE_SYSTEM.md`, `docs/UPDATE_SECURITY.md`,
`docs/UPDATE_ARCHITECTURE.md`, `docs/GITHUB_RELEASE.md`.

## Platform support

| Platform | Status |
|---|---|
| Linux x86_64 (Fedora / KDE Plasma / Wayland) | ✅ Released (RPM) |
| Windows x64 (10 1809+ / 11) | 🧪 Prerelease (`Connective-*-windows-x64-setup.exe`) |

## Installation

**Linux** — download the Fedora RPM from
[GitHub Releases](https://github.com/calledLifelss/Connective/releases)
and install:

```bash
sudo rpm -Uvh connective-0.2.0-*.fc44.x86_64.rpm
```

Launch **Connective** from the application menu. The UI never runs as
root — privileged TUN/firewall operations go through a small
polkit-authorized helper (one auth prompt per connect).

**Windows** — download `Connective-<version>-windows-x64-setup.exe`
from [GitHub Releases](https://github.com/calledLifelss/Connective/releases)
and run it (UAC elevation is requested by the installer). The app
itself never runs elevated: TUN/routes/firewall go through a
per-action UAC helper. Start Menu entry included, optional desktop
shortcut. Uninstall keeps `%LOCALAPPDATA%\Connective` (subscriptions,
servers, settings) unless you tick removal. Details, build
instructions, and troubleshooting: [`docs/WINDOWS.md`](docs/WINDOWS.md).

## Building

Requires Flutter (stable) and Go ≥ 1.26.

```bash
# Backend (add GOOS=windows for the cross-compile proof)
cd backend
go build ./...
go test ./...

# UI (Linux desktop)
cd ../frontend
flutter pub get
flutter analyze
flutter test
flutter run -d linux        # attaches to the running backend
flutter build linux --release
```

Windows builds run on GitHub Actions (`windows.yml`): Go + Flutter
builds, unit/widget tests, Inno Setup installer, smoke
install/launch/uninstall, and update-asset publishing. See
[`docs/WINDOWS.md`](docs/WINDOWS.md).

RPM layout: `flutter build linux --release` bundle + `connectived`,
`connective-helper`, `sing-box` → `packaging/connective.spec`.

## Testing

```bash
cd backend && go test ./...                        # backend suite
cd ../frontend && flutter analyze && flutter test  # UI suite
flutter test integration_test -d linux             # desktop integration
./tests/e2e-30s.sh && ./tests/e2e-failover-90s.sh  # live E2E (loopback)
```

Widget/integration tests run against the real backend (no mocks);
update tests use deterministic signed local fixtures.

## Updates

Connective checks **GitHub Releases** from inside the application:

- update manifests are **cryptographically verified** (Ed25519) and
  every artifact is **SHA-256 checked** before anything is staged;
- **full or delta** artifacts may be used — the smallest safe payload
  wins, full is always the fallback;
- installation is staged, atomically activated, health-checked, and
  **rolled back** automatically on failure;
- channels: **stable** (default), **beta**, **dev** (Settings →
  UPDATES).

A failed or forged update can never replace the working installation.

## Roadmap

- Versioned `/opt` layout migration for the RPM (updater-ready).
- Windows release.
- CI-built release artifacts on every release tag.
- Per-app split routing rules UI.

## License

Connective's own code is original work. The reference clients studied
for behavior only (Hiddify, v2rayN) are credited — and not copied — in
`docs/THIRD_PARTY_NOTICES.md`.

The bundled **sing-box** binary is GPLv3 (Sagernet); its license and
source pointers ship in the package (`packaging/LICENSE.sing-box`,
`packaging/LICENSE.GPLv3`). Country flag artwork is MIT
(`flag-icons`, see notices).

## Third-party notices

Full details in [`docs/THIRD_PARTY_NOTICES.md`](docs/THIRD_PARTY_NOTICES.md)
and `packaging/THIRD_PARTY_NOTICES.md`.
