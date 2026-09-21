# Reference implementation notes

Sources inspected under `~/Desktop/Sources` (read-only; behavior studied,
**no code copied** — see THIRD_PARTY_NOTICES.md for license constraints).

## Hiddify (`hiddify-app-4.1.1`)

| Area | Relevant source | Observed behavior | Connective implementation |
|---|---|---|---|
| App shape | `lib/` (Flutter) + `hiddify-core` submodule (empty checkout; Go + sing-box per README/pubspec) | Flutter UI, Go core, sing-box data plane | Same layering, but Go backend is a **socket daemon** instead of FFI (cleaner privilege split, UI crash can't take down tunnel) |
| Core choice | `README.md` ("based on sing-box"), `lib/singbox/`, `lib/hiddifycore/` | Single sing-box core incl. TUN/routing/DNS | `configgen` renders sing-box JSON; `core` supervises the binary |
| Auto selection | UI + core delay-based selection ("Delay based node selection") | Delay-tested selection, stable (no flap in UX) | `selector`: latency/health/stability weights + 15% hysteresis + switch margin (independent algorithm, same stability goal) |
| Profiles/subs | `lib/features/profile/`, remaining-days/traffic display, auto update | Subscription as object with traffic + expiry + auto refresh | `subscriptions`: `Subscription` + `Traffic`, userinfo header, `Merge` preserving local state |
| Connection state | `lib/features/connection/`, `lib/features/home/` | Rich connection-state UX | `connection`: 13-state machine, UI renders backend state |
| Platform | `linux/`, `windows/`, tray (`lib/features/system_tray/`) | Desktop integration, per-OS code | `platform/` split; tray/autostart deferred to phase 2 |

## v2rayN (`v2rayN-7.24.9`)

| Area | Relevant source | Observed behavior | Connective implementation |
|---|---|---|---|
| Multi-core | `ServiceLib/Enums/ECoreType.cs`, `Manager/CoreInfoManager.cs`, `README` | Xray + sing-box (+others); per-type defaults, fallback Xray | sing-box first; `configgen` seam allows an Xray renderer later |
| Server model | `Models/Entities/ProfileItem.cs`, `ProtocolExtraItem.cs`, `TransportExtraItem.cs`, `Enums/EConfigType.cs` | Flat record + proto/transport extras; VMess/VLESS/Trojan/SS/Hysteria2/TUIC/WireGuard/… | `servers.Server`: same coverage for phase-1 protocols, extensible enums |
| Link parsing | `Handler/Fmt/*Fmt.cs` via `FmtHandler` prefix dispatch | Per-protocol share-link parse/emit; vmess base64-json; SS SIP008; dedup | `servers/parse.go` + `export.go`: prefix dispatch, same accepted shapes, strict validation |
| Batch ingest | `Handler/ConfigHandler.cs` (`AddBatchServers`, `DedupServerList`) | Batch import with dedup | `subscriptions.Update` + `servers.Key()` dedup |
| Subscriptions | `Handler/SubscriptionHandler.cs`, `Services/DownloadService.cs`, `Manager/TaskManager.cs` | Validate → download (proxy-then-direct) → optional subconverter → batch add; periodic auto-update | `subscriptions.Update` + `SplitLinks`; scheduler in phase 2 |
| Config gen | `Services/CoreConfig/Singbox/*`, `V2ray/*`, `Handler/Builder/CoreConfigContextBuilder.cs` | Shared context → per-core renderer; validation (`NodeValidator`); chain/group expansion | `configgen.Generate`: sing-box renderer from canonical record; validation in `servers.Validate` |
| Core lifecycle | `Manager/CoreManager.cs`, `Services/ProcessService.cs` | Generate → stop → start → wait-for-port → pre-service; log streaming; sudo elevation for TUN | `core.Manager`: start/stop/restart, readiness probe, log pipes, no orphans; elevation via phase-2 helper |
| TUN | `SingboxInboundService` (`singbox_tun`, auto_route/strict_route), `V2rayInboundService` (`xray_tun`), `Common/WindowsUtils.cs` cleanup | Core-owned TUN device, stale cleanup | `configgen` emits TUN inbound; `tun.Manager` owns verify/cleanup (phase 2) |
| Routing/DNS | `*RoutingService.cs`, `*DnsService.cs`, `SingboxRulesetService.cs`, `Handler/SysProxy/*` | Per-core routing/DNS, rulesets, system-proxy backends per OS | `configgen` defaults + `routing/dns/sysproxy` contracts (phase 2) |
| Testing | `Services/SpeedtestService.cs` (Tcping vs Realping vs Speedtest, batched, cancellable) | Two-stage testing, concurrency, cancel | `tester`: TCPing + bulk runner + cancel; real-ping-via-core in phase 2 |
| Health/stats | `Manager/StatisticsManager.cs`, `Services/Statistics/*`, `ConnectionHandler.RunAvailabilityCheck()` | Core-stats APIs, no auto-switch daemon (manual sort + urltest/selector outbounds) | `connection` machine + `selector.ShouldSwitch` add the auto-failover v2rayN lacks |

## What was deliberately NOT reused

- No Hiddify/v2rayN source files, snippets, or translations exist in this
  tree (both are GPL-family with Hiddify carrying extra fork/naming/
  non-commercial conditions incompatible with a clean product).
- sing-box itself is **not vendored**: it remains an external binary
  dependency fetched at install/packaging time (phase 2).
- GeoIP/rule-set data (`*.srs`, geo files) are referenced by URL at
  runtime, never copied in.
