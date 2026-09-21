# CONNECTIVE

A production-grade desktop VPN/proxy client: **Flutter/Dart UI + Go backend +
sing-box core**, Linux first (Fedora / KDE Plasma / Wayland), Windows later.

> **Status: Phase 3 — full application.** The Go backend domain layer,
> connection lifecycle, privileged helper, stats, monitoring and
> failover are implemented and tested (`go test ./...` green). The
> Flutter UI (dashboard, servers, subscriptions, routing, settings,
> logs, tray integration) talks to the backend over the versioned IPC
> contract; `flutter analyze` is clean and `flutter test` (10 tests:
> store IPC + widget flows) runs against the real backend.
> TUN/routing/DNS enforcement and kill-switch were verified live in an
> isolated netns; IPv6 egress is host-limited (see docs/E2E_REPORT.md).

## Layout

```
connective/
  backend/            Go backend (module connective/backend, stdlib only)
    cmd/connectived/  daemon entrypoint (IPC socket, state machine, store)
    internal/
      servers/        server model + share-link parse/export (vless/vmess/trojan/ss)
      subscriptions/  fetch / parse / userinfo / local-preserving merge
      selector/       Auto scoring with hysteresis (no naive lowest-ping)
      connection/     centralized connection state machine + events
      configgen/      sing-box JSON config generation
      tester/         async TCP testing + bulk runner with cancellation
      core/           core process supervisor (start/stop/restart, no orphans)
      ipc/            versioned JSON-over-unix-socket Flutter↔Go protocol
      persistence/    atomic per-domain JSON store
      logging/        leveled logs with secret redaction + ring buffer
      settings/       validated application settings
      tun/ routing/ dns/ firewall/ sysproxy/  phase-2 contracts (stubs)
      platform/       OS abstraction, data dirs, socket path
  frontend/           Flutter skeleton (models mirror backend, IPC client, pages)
  docs/
    ARCHITECTURE.md
    REFERENCE_IMPLEMENTATION_NOTES.md
    THIRD_PARTY_NOTICES.md
    IPC_CONTRACT.md
```

## Quickstart (backend)

Requires Go ≥ 1.24 (1.26.8 used; a user-space install lives at
`~/.local/golang-root` on the build machine):

```bash
cd backend
go build ./...
go test ./...
./connectived &   # via: go run ./cmd/connectived
```

No external Go dependencies — the backend is stdlib-only, so it builds
offline.

## Core decision

**sing-box** (single integrated core for proxy + TUN + routing + DNS),
integrated as an external binary dependency. Xray support may follow.
Rationale in `docs/ARCHITECTURE.md`; reference findings in
`docs/REFERENCE_IMPLEMENTATION_NOTES.md`.

## License posture

Reference sources (`~/Desktop/Sources`: Hiddify GPLv3+extra-conditions,
v2rayN GPLv3) were studied for **behavior only**. No reference code is
copied into this tree; all implementation is original. Details in
`docs/THIRD_PARTY_NOTICES.md`.
