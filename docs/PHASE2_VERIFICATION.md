# Phase-2 verification against reference sources

Inspected `~/Desktop/Sources` before writing phase-2 code. Behavior only;
no reference code copied (see THIRD_PARTY_NOTICES.md).

## V1 — Auto server selection

### v2rayN
- `Services/CoreConfig/Singbox/SingboxOutboundService.cs:593`
  `BuildSelectorOutbounds`: multi-server configs get a **`urltest`**
  outbound (`tag = "<base>-auto"`, `interrupt_exist_connections = false`;
  `tolerance = 5000` in Fallback mode) plus a **`selector`** outbound whose
  list starts with the urltest tag (auto) followed by every proxy (manual
  override). In-process, core-native auto-pick — not external ping.
- `Services/CoreConfig/V2ray/V2rayBalancerService.cs:84-106`: Xray-side
  equivalent is a `leastPing` balancer (`tolerance = 0.2` for Fallback).
- `Services/SpeedtestService.cs` + `Global.cs:94`: UI-side testing is
  two-stage (Tcping vs Realping-through-ephemeral-core), batched
  (`SpeedTestPageSize = 1000`), bounded concurrency
  (`MixedConcurrencyCount` default **5**), cancellable; results persist in
  `ProfileExManager` and the list sorts by delay (`ProfilesViewModel:203`).
- No auto-switch daemon: resilience = manual sort + core urltest/selector.
- Background: `Manager/TaskManager.cs` runs subscription/geo upkeep on a
  1-minute loop; per-sub auto-update when
  `now - UpdateTime >= AutoUpdateInterval*60`.

### Hiddify
- Testing is **core-native**: `features/proxy/data/proxy_repository.cs:87`
  `urlTest(groupTag)` calls `singbox.urlTest(groupTag)` (gRPC
  `hcore_service.pbgrpc.cs:201`); UI reads per-proxy `urlTestDelay`,
  `> 65000` = timeout (`active_proxy_delay_indicator.dart:25`,
  `proxy_tile.dart:52`). Throttled to 1/second
  (`active_proxy_notifier.dart:91`).
- Defaults (`features/settings/data/config_option_repository.dart`):
  `url-test-interval` = **10 min**, `connection-test-url` =
  `http://captive.apple.com/hotspot-detect.html` (alternatives include
  `generate_204` endpoints), `clash-api-port` = **16756** (enabled),
  `mtu` = **9000**, `strict-route` = true, TUN stack default **gVisor**.
- Reconnect on profile change (`connection_notifier.dart:49-53,96-104`);
  no OS network-change listener found in `lib/` — recovery is
  reconnect-on-demand + periodic url tests.

### Connective consequences (applied)
1. Multi-server configs render core-native **`urltest` + `selector`**
   outbounds (v2rayN pattern, `interrupt_exist_connections: false`,
   tolerance to suppress flapping) — `configgen.GenerateGroup`.
2. Daemon-side `selector` (latency/health/stability + hysteresis) stays as
   the **pre-connect ranker and failover decider**; per-server TCP tests
   stay for the server list (v2rayN Tcping parity). Not "lowest ping":
   hysteresis margin + unhealthy cap + stability weight.
3. Adopted numbers: test concurrency **5**, url-test interval **10 min**,
   clash API on (port **16756** default), health probe via
   connection-test URL through the mixed inbound, timeout marker 65 s.
4. Failover/reconnect on health failure + interface-change detection
   (poll-based; Hiddify has no OS listener, we add a small poller since
   desktop sleep/Wi-Fi hops need it).

## V2 — Subscription updating

### v2rayN (`Handler/SubscriptionHandler.cs`, `Handler/ConfigHandler.cs`)
- Per-sub loop over all subs (or one by id): skip when id/url empty, URL
  must be http(s), **skip disabled**, proxy-then-direct download fallback
  (`TryDownloadString(url, blProxy)` → retry direct), main URL + `MoreUrl`
  concatenation (base64-aware), then `AddBatchServers(..., isSub: true`).
- `AddBatchServers` (line 2040): for subscriptions it **removes all
  existing servers of that sub first** (`RemoveServersViaSubid`) and
  re-adds; test results survive separately in `ProfileEx` sidecar;
  per-sub `Filter` regex supported; short bodies echoed for diagnostics.

### Hiddify
- Profile DB (`core/db`, `profile_entries`): url, updateInterval,
  upload/download/expire persisted; traffic + expiry shown in UI;
  auto-update on interval.

### Connective consequences (applied)
- `Subscription` gains `Enabled`, `UpdateInterval`, `MoreURLs`,
  `UserAgent`; update-all + update-one; disabled skipped; http(s)-only
  validation; proxy-then-direct fallback (proxy = live mixed inbound).
- Refresh **merges, not wipes** (`Merge` by `Server.Key()`): remote fields
  update, `ID/Favorite/CustomName/cached results` survive. This is a
  deliberate improvement over v2rayN's remove-then-add (same net effect
  for server lists, better for user customizations); test results also
  persist in the `test_results` doc, mirroring `ProfileEx` separation.

## V3 — Core selection: is sing-box alone sufficient?

- Hiddify ships **sing-box-only** on every desktop OS (README "based on
  sing-box"; `lib/singbox/`, TUN/routing/DNS/stats all via that core).
- v2rayN generates **complete sing-box configs** (TUN inbound with
  auto_route/strict_route, routing, DNS, urltest/selector) and treats
  sing-box as co-equal modern core; its per-type default is Xray only for
  historical reasons (`OptionSettingViewModel:238-242` defaults every
  `EConfigType` to Xray; users switch types to sing-box explicitly).
- **Verdict: sing-box alone is sufficient for phase 1.** Everything in
  scope (VLESS/VMess/Trojan/Shadowsocks; TCP/WS/gRPC/H2; TLS/Reality;
  TUN; rule routing; proxy-aware DNS; urltest/selector; clash-API stats)
  is covered — proven by Hiddify production use.
- **Known Xray gaps (deferred, seam kept):** raw custom-JSON outbounds,
  Xray-only transports/options (mKCP variants, legacy XTLS flows),
  `PolicyGroup/ProxyChain` equivalents, Xray observatory/balancer mode.
  `configgen` keeps the per-core renderer seam (mirroring v2rayN's
  `CoreConfigContext`) so an Xray renderer can be added without
  re-architecting; unsupported protocols fail loudly at generation time.
