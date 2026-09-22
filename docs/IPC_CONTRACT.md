# IPC contract — Flutter ↔ Go (protocol v1)

Transport: newline-delimited JSON frames over a local socket
(`0600` on Linux):
- Linux: `~/.local/share/connective/connectived.sock`;
- Windows: `%LOCALAPPDATA%\Connective\connectived.sock` (Win10 1803+).
`CONNECTIVE_SOCK` overrides the path and `CONNECTIVE_DATA_DIR`
overrides the data dir on every OS (tests, portable installs).

Frame: `{"v":"1","id":"<req id>","type":"<method|event>","payload":{...},"error":"..."}`

- Requests carry `id`; responses echo it (out-of-order arrival is
  fine — clients match by `id`). Version mismatch and unknown
  methods are errors, never silent.
- Handlers run concurrently: a slow call (`connection.connect`,
  `subscriptions.update`) never blocks `ping`/`state.get` behind it.
- The daemon broadcasts events to all clients; slow clients are dropped
  rather than stalling the backend.
- The UI never parses shell output; everything in §5 (commands, selection,
  subscriptions, tests, settings, TUN/connection state, logs, stats,
  errors, events, health) travels as typed frames below.

## Methods (UI → daemon)

| Method | Payload | Result |
|---|---|---|
| `ping` | — | `{"version":"1"}` |
| `state.get` | — | `{state, server, auto, selectedServer, settings, foreignTun}` (`server` = connected server, `selectedServer` = manual selection, may differ) |
| `connection.connect` | `{serverId?}` (empty = AUTO) | state |
| `connection.disconnect` | — | state |
| `servers.list` | — | `[Server]` |
| `servers.select` | `{serverId}` (`"auto"` = AUTO) | selection |
| `servers.test` | `{serverIds[]}` | async; per-server `event.health` |
| `servers.add` | `{link}` or `{server}` | Server |
| `servers.update` | `{server}` | Server (test results preserved) |
| `servers.remove` | `{id}` | guards: active connection, subscription-owned |
| `servers.duplicate` | `{id}` | Server copy |
| `servers.export` | `{id}` | `{link}` |
| `subscriptions.list` | — | `[Subscription]` |
| `subscriptions.add` | `{name, url}` | Subscription |
| `subscriptions.edit` | `{id, name?, url?, enabled?, updateIntervalMin?}` | Subscription |
| `subscriptions.update` | `{id}` | `{servers, traffic}` |
| `subscriptions.remove` | `{id}` | ok |
| `settings.get` | — | Settings |
| `settings.update` | Settings | Settings |
| `logs.get` | `{n?, level?}` | `[Entry]` |
| `logs.clear` | — | ok |
| `stats.get` | — | `{up, down, upTotal, downTotal, durationMs}` |
| `ui.get` / `ui.update` | — / `{state}` | UI state (collapse maps etc.) |
| `update.check` | — | snapshot; check runs async, result via `event.update` |
| `update.status` | — | `{state, info?, progress, error?, channel, currentVersion, lastCheckUnix}` |
| `update.download` | — | snapshot; download runs async, progress via `event.update` |
| `update.cancel` | — | snapshot after cancelling in-flight work |
| `update.install` | — | snapshot; stage/activate async, result via `event.update` |
| `update.dismiss` | — | snapshot after hiding the available update |

Phase-1 daemon implements: `ping`, `state.get`, `settings.get`,
`settings.update`, `logs.get`. Remaining handlers land with their
subsystems in phase 2; the names are frozen now so the UI can be built
against them.

## Events (daemon → UI)

| Event | Payload |
|---|---|
| `event.state` | `{from, to, reason, server}` — every state-machine edge |
| `event.servers` | `[Server]` — list changed (refresh/test result) |
| `event.subscriptions` | `[Subscription]` — metadata changed |
| `event.stats` | traffic tick (1 Hz while connected) |
| `event.log` | single log Entry (viewer tail) |
| `event.health` | `{serverId, latencyMs, health}` — test/failover updates |
| `event.update` | update `Status` snapshot — every state/progress edge |

## Types

`Server`, `Subscription`, `Traffic`, `Settings` mirror
`backend/internal/{servers,subscriptions,settings}` JSON exactly; the Dart
models in `frontend/lib/models/` are field-for-field copies of those
shapes (see `daemon_client.dart`).
