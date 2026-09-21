# CONNECTIVE — Architecture

## 1. Core decision: sing-box (primary), Xray optional later

Evidence from source inspection:

- **Hiddify** (`hiddify-app-4.1.1`, `lib/singbox/`, `lib/hiddifycore/`) is a
  Flutter UI driving a Go core built on **sing-box** via FFI. One process
  owns proxying, TUN, routing and DNS. Its README states
  "multi-platform proxy client based on sing-box".
- **v2rayN** (`ServiceLib/Enums/ECoreType.cs`, `CoreConfig/*` services)
  supports **both Xray and sing-box** (`README: "Support Xray and sing-box
  and others"`), generating `xray.json` vs `sing-box.json` from a shared
  `CoreConfigContext`. Its TUN exists for both cores (`SingboxInboundService`
  `singbox_tun`, `V2rayInboundService` `xray_tun`), but the sing-box path is
  the modern one with `auto_route/strict_route` in-process.
- v2rayN's resilience model is stateless restart + pre-SOCKS chaining +
  `urltest/selector` outbounds; Hiddify's UX is delay-based auto selection
  with TUN.

Decision: **sing-box as the single phase-1 core**, consumed as an
**external binary** (`sing-box run -c config.json`) supervised by
`backend/internal/core`. Rationale:

1. One process for proxy+TUN+routing+DNS removes an entire class of
   lifecycle races (no separate TUN daemon to orphan).
2. `auto_route/strict_route` TUN matches the Linux-first target and the
   Hiddify desktop proof-point.
3. v2rayN proves the config-generation seam (`CoreConfigContext` → per-core
   renderer) keeps a future Xray renderer possible without re-architecting;
   `configgen` mirrors that seam.
4. No core is vendored or rewritten: sing-box stays an external GPLv3
   dependency (see THIRD_PARTY_NOTICES.md).

## 2. Process map

```
Flutter UI (unprivileged)  ←unix socket JSON→  connectived (unprivileged)
                                                     │ supervises
                                              sing-box run -c cfg
                                                     │ configures (auto_route)
                                              TUN + routes + DNS
                                                     │ privileged syscalls via
                                              helper (polkit, phase 2)
```

The UI and daemon **never run as root**. Privileged operations
(interface creation, nftables kill-switch, resolv.conf) execute in a tiny
auditable helper authorized per-action (polkit on Linux). Phase 1 fixes the
`platform` abstraction and the `tun/routing/dns/firewall` interfaces; the
helper and netlink backends are phase 2.

## 3. Flutter architecture (frontend/)

- `models/` — `Server`, `Subscription`, `ConnectionState`: Dart mirrors of
  the backend structs, deserialized from IPC frames only.
- `services/daemon_client.dart` — socket IPC client: request/response +
  event subscription. The single source of backend truth.
- `state/app_store.dart` — `ChangeNotifier` store: server/subscription
  lists, selection (AUTO vs manual), collapse states (persisted via
  backend `ui_state` doc), connection state, stats.
- `pages/` — dashboard, servers, subscriptions, routing, settings, logs.
- `components/` — server rows (§16–18), subscription cards (§15), connect
  button (§30).
- UI holds **no networking logic** and parses **no shell output**.

## 4. Go backend architecture (backend/internal/)

| Package | Owns |
|---|---|
| `servers` | Canonical server record (§12), share-link parse/export, validation of untrusted input |
| `subscriptions` | Fetch (timeout + 10 MiB cap), plain/base64 parsing, `subscription-userinfo` traffic/expiry, `Merge` preserving Favorite/CustomName/ID/test results |
| `selector` | Auto scoring: 55% latency, 30% health, 15% stability; hysteresis bonus + switch margin; unhealthy cap below untested |
| `connection` | 13-state machine (§22), legal-transition enforcement, event fan-out |
| `configgen` | sing-box JSON: vless/vmess/trojan/ss outbounds, TLS/Reality, ws/gRPC/h2 transports, mixed+TUN inbounds, DNS + route defaults |
| `tester` | Async TCPing, bounded-concurrency bulk runner, context cancellation, retries; real-ping-via-core in phase 2 |
| `core` | Child supervision: startup timeout, readiness probe, early-exit detection, SIGTERM→SIGKILL→reap, duplicate-start guard |
| `ipc` | Protocol v1 framing, method dispatch, event broadcast with slow-client drop |
| `persistence` | Per-domain atomic JSON docs (`subscriptions, servers, settings, selection, ui_state, test_results`) |
| `logging` | DEBUG..ERROR, secret redaction, 2000-entry ring, in-app viewer source |
| `settings` | Validated settings with safe defaults (AUTO, global, proxy-aware DNS) |
| `tun/routing/dns/firewall/sysproxy` | Phase-2 contracts; stubs return `ErrNotImplemented` |
| `platform` | Linux/Windows split point, `~/.local/share/connective`, socket path |

## 5. Connection lifecycle (wired end state)

```
disconnected → testing → selecting → connecting → starting-core
  → initializing-tun → applying-routing → connected
  ⇄ degraded | switching-server | reconnecting → …
  → disconnecting → disconnected (routes/TUN/DNS restored)
```

Every edge is enforced by `connection.Machine`; the UI renders backend
state verbatim and never claims "connected" on its own.

## 6. Auto behavior

1. Bulk TCP-test candidates (bounded concurrency, cancellable).
2. `selector.Rank` with hysteresis; incumbent keeps a 15% bonus.
3. Connect best; monitor health (core stats + active probes, phase 2).
4. On degradation: `ShouldSwitch` requires >15% improvement — millisecond
   noise never causes hops. Failover path:
   `connected → degraded → switching-server → connecting → …`.

## 7. Subscription refresh

URL → fetch (timeout, size cap) → validate → split links (plain/base64) →
parse each (one bad entry never poisons the batch) → `Merge` against
stored servers by `Server.Key()` (protocol|endpoint|auth) → persist →
re-rank Auto candidates. Remote renames apply; Favorite/CustomName/IDs/
cached latencies survive.

## 8. Persistence

Six small documents, not one giant blob. Atomic temp+rename writes,
`0600` permissions, corrupt-document errors isolated per doc.

## 9. Security

- All subscription/import data is untrusted: URL/port/enum validation in
  `servers.Validate`, HTTP status + size caps in `subscriptions.Fetch`.
- No shell interpolation of server data anywhere; core launches via
  `exec` argv only.
- `logging.Redact` scrubs uuid/password/token markers; secrets never
  enter logs by construction.
- Socket file is `0600`; daemon exposes no network listener.

## 10. Testing

`go test ./...` covers: link parse/reject/round-trip, subscription
fetch/parse/merge over `httptest`, selector ordering/hysteresis/caps,
state-machine legal + illegal paths, config JSON shape per protocol,
IPC round-trip/errors/broadcast, atomic store + corruption isolation,
log redaction/ring, core start/stop/timeout/early-exit/log capture (real
processes), TCPing + bulk + cancel. Phase 2 adds: core lifecycle with the
real sing-box binary, TUN/route/DNS integration, recovery drills, and the
§47 end-to-end scenario on Fedora/KDE/Wayland.
