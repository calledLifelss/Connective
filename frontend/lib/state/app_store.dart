import 'dart:async';

import 'package:flutter/foundation.dart';

import 'package:launch_at_startup/launch_at_startup.dart';

import '../models/connection_state.dart';
import '../models/server.dart';
import '../models/settings.dart';
import '../models/subscription.dart';
import '../models/stats.dart';
import '../models/update.dart';
import '../services/daemon_client.dart';
import '../services/notifier.dart';

/// Friendly error mapping (§19): backend technical errors become
/// understandable messages; details stay in Logs.
String friendlyError(Object e) {
  final msg = e.toString();
  if (msg.contains('no servers available')) {
    return 'No servers yet. Add a subscription first.';
  }
  if (msg.contains('could not connect to')) {
    final m = RegExp(r'could not connect to ([^:.]+)').firstMatch(msg);
    final who = m != null ? m.group(1) : 'server';
    return 'Could not connect to $who. Connective will try another server.';
  }
  if (msg.contains('Another VPN/TUN interface is active')) {
    // Backend safe-failure is already user-facing; show it verbatim
    // (truncated) instead of the generic TUN message.
    final short = msg.replaceFirst('Exception: ', '');
    return short.length > 260 ? '${short.substring(0, 260)}…' : short;
  }
  if (msg.contains('TUN device') || msg.contains('TUN')) {
    return 'TUN setup failed (elevation or device issue). See Logs.';
  }
  if (msg.contains('kill switch')) {
    return 'Kill-switch setup failed. See Logs.';
  }
  if (msg.contains('subscription')) {
    return 'Subscription update failed. Check the URL and see Logs.';
  }
  if (msg.contains('already connected') || msg.contains('busy:')) {
    return 'A connection change is already in progress.';
  }
  if (msg.contains('server "') && msg.contains('not found')) {
    return 'That server no longer exists. Refresh subscriptions.';
  }
  if (msg.contains('belongs to a subscription')) {
    return 'Subscription servers return on next update. Remove the subscription instead.';
  }
  if (msg.contains('Disconnect first')) {
    return 'Disconnect before removing the active server.';
  }
  if (msg.contains('timed out')) {
    return 'The backend did not answer in time. Is connectived running?';
  }
  if (msg.contains('not connected to daemon') ||
      msg.contains('daemon gone')) {
    return 'Lost connection to the backend. Restart the app.';
  }
  final short = msg.replaceFirst('Exception: ', '');
  return short.length > 220 ? '${short.substring(0, 220)}…' : short;
}

/// Central UI store. Presentation state only; every mutation goes
/// through [DaemonClient] and every state change originates from daemon
/// events (docs/IPC_CONTRACT.md). All backend work stays asynchronous;
/// the UI thread never blocks (§18).
class AppStore extends ChangeNotifier {
  final DaemonClient client = DaemonClient();
  StreamSubscription<Map<String, dynamic>>? _events;
  Timer? _uiSaveDebounce;
  bool _disposed = false;

  @override
  void notifyListeners() {
    if (_disposed) return;
    super.notifyListeners();
  }

  // --- connection ---
  String connectionState = ConnectionStates.disconnected;

  /// Connected (or last-connected) server reported by the daemon state
  /// machine (`state.get` → `server`, `event.state` → `server`).
  String activeServerId = '';

  /// Manual user selection (`state.get` → `selectedServer`,
  /// `servers.select` result). Distinct from [activeServerId] so the
  /// Dashboard can show "CONNECTED: X" vs "SELECTED: Y" side by side.
  /// Empty while AUTO is active.
  String selectedServerId = '';
  bool autoMode = true;
  String? lastError;

  /// Foreign tunnel devices visible on the host (another VPN running).
  /// Informational: connecting may conflict; the backend still fails
  /// safely instead of corrupting routes.
  List<String> foreignTun = const [];

  /// Dismiss the current error banner.
  void dismissError() {
    lastError = null;
    notifyListeners();
  }

  /// Set the log viewer minimum level.
  void setLogLevel(int v) {
    logMinLevel = v;
    notifyListeners();
  }

  // --- data ---
  List<Subscription> subscriptions = const [];
  List<Server> servers = const [];
  AppSettings settings = const AppSettings();
  TrafficStats stats = const TrafficStats();
  List<LogEntry> logs = const [];

  // --- updates (backend-owned; see internal/update) ---
  String updateState = UpdateStates.idle;
  UpdateInfo? updateInfo;
  UpdateProgress updateProgress = const UpdateProgress();
  String? updateError;
  String updateChannel = 'stable';
  String updateCurrent = '';
  int updateLastCheck = 0;
  bool updateStaged = false;

  bool get updateAvailable =>
      updateState == UpdateStates.available && updateInfo != null;
  bool get updateBusy => UpdateStates.isBusy(updateState);

  // --- presentation ---
  final Map<String, bool> subscriptionExpanded = {};
  final Set<String> expandedServers = {};
  final Set<String> testingServers = {};
  final Map<String, bool> updatingSubs = {};
  String serverSearch = '';
  int logMinLevel = 0;

  /// UI-local preferences (persisted via ui.get/ui.update, no backend
  /// changes needed — presentation concerns only).
  bool notificationsEnabled = true;
  bool launchOnLogin = false;

  bool get backendAlive => _backendAlive;
  bool _backendAlive = false;
  String? _socketPath;

  bool isExpandedSub(String id) => subscriptionExpanded[id] ?? true;
  bool isExpandedServer(String id) => expandedServers.contains(id);

  void toggleSubscription(String id) {
    subscriptionExpanded[id] = !isExpandedSub(id);
    notifyListeners();
    persistUi();
  }

  void toggleServer(String id) {
    if (!isExpandedServer(id)) {
      expandedServers.add(id);
    } else {
      expandedServers.remove(id);
    }
    notifyListeners();
  }

  List<Server> serversOf(String subscriptionId) =>
      servers.where((s) => s.subscriptionId == subscriptionId).toList();

  List<Server> get localServers =>
      servers.where((s) => s.subscriptionId.isEmpty).toList();

  Server? get activeServer {
    for (final s in servers) {
      if (s.id == activeServerId) return s;
    }
    return null;
  }

  /// The manually selected server (null in AUTO mode or when the
  /// selection no longer exists, e.g. after a subscription refresh).
  Server? get selectedServer {
    if (autoMode || selectedServerId.isEmpty) return null;
    for (final s in servers) {
      if (s.id == selectedServerId) return s;
    }
    return null;
  }

  /// What a Connect press would use: AUTO, or the manual selection.
  /// Falls back to the active server when the stored selection vanished.
  Server? get connectTarget {
    if (autoMode) return activeServer;
    return selectedServer ?? activeServer;
  }

  /// True when the user picked a specific server that still exists.
  bool get hasManualSelection => selectedServer != null;

  /// Latency label for the connection header: prefers the connected
  /// server, then the manual selection, else nothing.
  String get headerLatency {
    final a = activeServer;
    if (a != null && a.latencyMs >= 0) return a.latencyLabel;
    final t = selectedServer;
    if (t != null) return t.latencyLabel;
    return '';
  }

  List<Server> search(String query) {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return servers;
    return servers.where((s) {
      final sub = _subName(s.subscriptionId).toLowerCase();
      return s.displayName.toLowerCase().contains(q) ||
          s.country.toLowerCase().contains(q) ||
          s.address.toLowerCase().contains(q) ||
          s.protocol.toLowerCase().contains(q) ||
          sub.contains(q);
    }).toList();
  }

  String _subName(String id) {
    for (final s in subscriptions) {
      if (s.id == id) return s.name;
    }
    return '';
  }

  String subscriptionName(String id) {
    final n = _subName(id);
    return n.isEmpty ? 'Local' : n;
  }

  // --- lifecycle ---

  /// Connect the socket, attach events, load everything.
  Future<void> boot(String socketPath) async {
    _socketPath = socketPath;
    await client.connect(socketPath);
    _backendAlive = true;
    attachEvents();
    await refreshAll();
    await loadUiState();
    await refreshLogs();
    await refreshUpdateStatus();
  }

  void attachEvents() {
    _events?.cancel();
    _events = client.events.listen(applyEvent);
  }

  void applyEvent(Map<String, dynamic> frame) {
    final type = frame['type'] as String? ?? '';
    final p = frame['payload'];
    switch (type) {
      case 'event.state':
        if (p is Map) {
          final m = Map<String, dynamic>.from(p);
          final prev = connectionState;
          connectionState = m['to'] as String? ?? connectionState;
          final srv = m['server'] as String?;
          if (srv != null && srv.isNotEmpty) activeServerId = srv;
          if (!ConnectionStates.isBusy(connectionState)) lastError = null;
          _announceTransition(prev, connectionState);
          notifyListeners();
        }
      case 'event.servers':
        if (p is List) {
          servers = [
            for (final e in p)
              Server.fromJson(Map<String, dynamic>.from(e as Map))
          ];
          testingServers.clear();
          notifyListeners();
        }
      case 'event.subscriptions':
        if (p is List) {
          subscriptions = [
            for (final e in p)
              Subscription.fromJson(Map<String, dynamic>.from(e as Map))
          ];
          updatingSubs.clear();
          notifyListeners();
        }
      case 'event.stats':
        if (p is Map) {
          stats =
              TrafficStats.fromJson(Map<String, dynamic>.from(p));
          notifyListeners();
        }
      case 'event.health':
        if (p is Map) {
          final m = Map<String, dynamic>.from(p);
          final id = m['serverId'] as String? ?? '';
          servers = [
            for (final s in servers)
              if (s.id != id)
                s
              else
                _withHealth(s, m),
          ];
          testingServers.remove(id);
          notifyListeners();
        }
      case 'event.log':
        if (p is Map) {
          final e = LogEntry.fromJson(Map<String, dynamic>.from(p));
          logs = [...logs, e];
          if (logs.length > 2000) logs = logs.sublist(logs.length - 2000);
          notifyListeners();
        }
      case 'event.update':
        if (p is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(p));
        }
    }
  }

  /// Announce user-meaningful transitions (§6, §23). Only genuine
  /// state changes notify — never progress noise.
  void _announceTransition(String prev, String next) {
    if (prev == next) return;
    final name = activeServer?.displayName ?? '';
    if (next == ConnectionStates.connected) {
      _notify('Connective: connected',
          name.isEmpty ? 'VPN is active.' : 'Connected via $name.');
    } else if (next == ConnectionStates.degraded &&
        prev == ConnectionStates.connected) {
      _notify('Connective: connection degraded',
          'Looking for a better server…');
    } else if (next == ConnectionStates.switchingServer) {
      _notify('Connective: switching server',
          name.isEmpty ? 'Finding another server…' : 'Switching to $name…');
    } else if (next == ConnectionStates.error &&
        (prev == ConnectionStates.connected ||
            prev == ConnectionStates.degraded)) {
      _notify('Connective: disconnected unexpectedly',
          lastError ?? 'The connection dropped.');
    }
  }

  Server _withHealth(Server s, Map<String, dynamic> m) => Server(
        id: s.id,
        name: s.name,
        customName: s.customName,
        subscriptionId: s.subscriptionId,
        address: s.address,
        port: s.port,
        protocol: s.protocol,
        transport: s.transport,
        security: s.security,
        country: s.country,
        favorite: s.favorite,
        latencyMs: (m['latencyMs'] as num?)?.toInt() ?? s.latencyMs,
        health: m['health'] as String? ?? s.health,
        uuid: s.uuid,
        password: s.password,
        method: s.method,
        flow: s.flow,
        sni: s.sni,
        fingerprint: s.fingerprint,
        publicKey: s.publicKey,
        shortId: s.shortId,
        path: s.path,
        host: s.host,
        serviceName: s.serviceName,
      );

  Future<void> _guarded(Future<void> Function() fn) async {
    try {
      await fn();
      if (!_backendAlive) {
        _backendAlive = true;
        notifyListeners();
      }
    } catch (e) {
      // Any IPC failure marks the backend lost so the UI never shows
      // stale "Connected" indefinitely (§7); recovery via reconnect().
      _backendAlive = false;
      lastError = friendlyError(e);
      notifyListeners();
    }
  }

  /// Re-establish the backend connection (after death or at retry).
  /// Safe to call repeatedly; never issues duplicate commands.
  Future<void> reconnect() => _guarded(() async {
        final sock = _socketPath;
        if (sock == null) {
          throw StateError('no backend address known');
        }
        if (client.isConnected) {
          try {
            await client.call('ping');
            await refreshAll();
            return;
          } catch (_) {
            client.close();
          }
        }
        await client.connect(sock);
        attachEvents();
        await refreshAll();
      });

  // --- loading ---

  Future<void> refreshAll() => _guarded(() async {
        final rawState = await client.call('state.get');
        final state = rawState is Map
            ? Map<String, dynamic>.from(rawState)
            : <String, dynamic>{};
        connectionState = state['state'] as String? ?? connectionState;
        activeServerId = state['server'] as String? ?? activeServerId;
        autoMode = state['auto'] as bool? ?? autoMode;
        // Newer daemons report the manual selection separately from the
        // connected server. Older daemons omit the key: keep the
        // optimistic local selection instead of clobbering it.
        if (state.containsKey('selectedServer')) {
          selectedServerId =
              state['selectedServer'] as String? ?? '';
          if (autoMode) selectedServerId = '';
        } else if (autoMode) {
          selectedServerId = '';
        }
        final ft = state['foreignTun'];
        if (ft is List) {
          foreignTun = [for (final e in ft) e.toString()];
        }
        if (state['settings'] is Map) {
          settings = AppSettings.fromJson(
              Map<String, dynamic>.from(state['settings'] as Map));
        }
        final list = await client.call('servers.list');
        if (list is List) {
          servers = [
            for (final e in list)
              Server.fromJson(Map<String, dynamic>.from(e as Map))
          ];
        }
        final subs = await client.call('subscriptions.list');
        if (subs is List) {
          subscriptions = [
            for (final e in subs)
              Subscription.fromJson(Map<String, dynamic>.from(e as Map))
          ];
        }
        notifyListeners();
      });

  Future<void> loadUiState() async {
    try {
      final raw = await client.call('ui.get');
      if (raw is Map) {
        final m = Map<String, dynamic>.from(raw);
        final exp = m['expandedSubs'];
        if (exp is Map) {
          for (final k in exp.keys) {
            subscriptionExpanded[k as String] = exp[k] == true;
          }
        }
        if (m['notifications'] is bool) {
          notificationsEnabled = m['notifications'] as bool;
        }
        notifyListeners();
      }
    } catch (_) {}
  }

  void persistUi() {
    _uiSaveDebounce?.cancel();
    _uiSaveDebounce = Timer(const Duration(milliseconds: 500), () async {
      try {
        await client.call('ui.update', {
          'expandedSubs': Map<String, bool>.from(subscriptionExpanded),
          'notifications': notificationsEnabled,
        });
      } catch (_) {}
    });
  }

  Future<void> setNotifications(bool v) async {
    notificationsEnabled = v;
    notifyListeners();
    persistUi();
  }

  Future<void> setLaunchOnLogin(bool v) async {
    launchOnLogin = v;
    notifyListeners();
    try {
      if (v) {
        await LaunchAtStartup.instance.enable();
      } else {
        await LaunchAtStartup.instance.disable();
      }
    } catch (_) {
      // Headless/test environments: optional integration.
    }
  }

  Future<void> refreshLoginState() async {
    try {
      launchOnLogin =
          await LaunchAtStartup.instance.isEnabled();
      notifyListeners();
    } catch (_) {}
  }

  Future<void> _notify(String title, String body) async {
    if (!notificationsEnabled) return;
    await Notifier.show(title, body);
  }

  // --- connection ---

  Future<void> toggleConnection() => _guarded(() async {
        if (ConnectionStates.isConnected(connectionState)) {
          await client.call('connection.disconnect');
        } else {
          // Manual mode connects to the user's selection — never
          // silently back to AUTO. Falls back to the active server id
          // when the stored selection vanished; empty means AUTO.
          final target =
              autoMode ? 'auto' : selectedServerIdOrActive;
          await client.call(
              'connection.connect', {'serverId': target});
        }
      });

  /// Server id a manual Connect press would send ('' must not happen:
  /// empty falls back to 'auto' semantics server-side).
  String get selectedServerIdOrActive =>
      selectedServerId.isNotEmpty ? selectedServerId : activeServerId;

  Future<void> selectServer(String id) => _guarded(() async {
        final out = await client.call('servers.select', {'serverId': id});
        // The daemon echoes {auto, serverId}; older daemons return the
        // selection too (same shape since introduction), but stay
        // optimistic if the payload is unexpected.
        if (out is Map) {
          final m = Map<String, dynamic>.from(out);
          if (m['auto'] is bool) autoMode = m['auto'] as bool;
          if (m['serverId'] is String) {
            selectedServerId = m['serverId'] as String;
          } else if (id == 'auto') {
            selectedServerId = '';
          } else {
            selectedServerId = id;
          }
        } else {
          autoMode = id == 'auto';
          selectedServerId = autoMode ? '' : id;
        }
        if (autoMode) selectedServerId = '';
        notifyListeners();
      });

  Future<void> testServers([List<String>? ids]) => _guarded(() async {
        final targets = ids ?? servers.map((s) => s.id).toList();
        testingServers.addAll(targets);
        notifyListeners();
        await client.call('servers.test', {'serverIds': targets});
      });

  // --- subscriptions ---

  Future<void> addSubscription(String name, String url) =>
      _guarded(() async {
        await client.call(
            'subscriptions.add', {'name': name, 'url': url});
        await refreshSubs();
      });

  Future<void> editSubscription(String id,
      {String? name,
      String? url,
      bool? enabled,
      int? updateIntervalMin}) =>
      _guarded(() async {
        final payload = <String, dynamic>{'id': id};
        if (name != null) payload['name'] = name;
        if (url != null) payload['url'] = url;
        if (enabled != null) payload['enabled'] = enabled;
        if (updateIntervalMin != null) {
          payload['updateIntervalMin'] = updateIntervalMin;
        }
        await client.call('subscriptions.edit', payload);
        await refreshSubs();
      });

  Future<void> removeSubscription(String id) => _guarded(() async {
        await client.call('subscriptions.remove', {'id': id});
        await refreshAll();
      });

  Future<void> updateSubscriptions([String? id]) => _guarded(() async {
        if (id != null) {
          updatingSubs[id] = true;
        } else {
          for (final s in subscriptions) {
            updatingSubs[s.id] = true;
          }
        }
        notifyListeners();
        var ok = 0;
        try {
          final out = await client.call(
              'subscriptions.update', {'id': id ?? ''});
          if (out is Map) {
            ok = (out['updated'] as num?)?.toInt() ?? 0;
          }
        } finally {
          updatingSubs.clear();
        }
        await refreshAll();
        if (ok > 0) {
          _notify('Connective: subscriptions updated',
              '$ok subscription${ok == 1 ? '' : 's'} refreshed.');
        }
      });

  Future<void> refreshSubs() async {
    try {
      final subs = await client.call('subscriptions.list');
      if (subs is List) {
        subscriptions = [
          for (final e in subs)
            Subscription.fromJson(Map<String, dynamic>.from(e as Map))
        ];
        notifyListeners();
      }
    } catch (e) {
      lastError = friendlyError(e);
      notifyListeners();
    }
  }

  // --- servers ---

  Future<void> importServer(String link) => _guarded(() async {
        await client.call('servers.add', {'link': link});
        await refreshServers();
      });

  Future<void> saveServer(Server s) => _guarded(() async {
        if (s.id.isEmpty) {
          await client.call('servers.add', {'server': s.toJson()});
        } else {
          await client.call('servers.update', {'server': s.toJson()});
        }
        await refreshServers();
      });

  Future<void> removeServer(String id) => _guarded(() async {
        await client.call('servers.remove', {'id': id});
        await refreshServers();
      });

  Future<void> duplicateServer(String id) => _guarded(() async {
        await client.call('servers.duplicate', {'id': id});
        await refreshServers();
      });

  Future<String?> exportServer(String id) async {
    try {
      final out = await client.call('servers.export', {'id': id});
      if (out is Map) return out['link'] as String?;
      return null;
    } catch (e) {
      lastError = friendlyError(e);
      notifyListeners();
      return null;
    }
  }

  Future<void> refreshServers() async {
    try {
      final list = await client.call('servers.list');
      if (list is List) {
        servers = [
          for (final e in list)
            Server.fromJson(Map<String, dynamic>.from(e as Map))
        ];
        notifyListeners();
      }
    } catch (e) {
      lastError = friendlyError(e);
      notifyListeners();
    }
  }

  // --- updates ---

  void applyUpdateStatus(Map<String, dynamic> m) {
    updateState = m['state'] as String? ?? updateState;
    final info = m['info'];
    updateInfo = info is Map
        ? UpdateInfo.fromJson(Map<String, dynamic>.from(info))
        : null;
    final prog = m['progress'];
    updateProgress = prog is Map
        ? UpdateProgress.fromJson(Map<String, dynamic>.from(prog))
        : const UpdateProgress();
    final err = m['error'] as String?;
    updateError = (err == null || err.isEmpty) ? null : err;
    updateChannel = m['channel'] as String? ?? updateChannel;
    updateCurrent = m['currentVersion'] as String? ?? updateCurrent;
    updateLastCheck =
        (m['lastCheckUnix'] as num?)?.toInt() ?? updateLastCheck;
    updateStaged = m['staged'] as bool? ?? false;
    notifyListeners();
  }

  Future<void> refreshUpdateStatus() async {
    try {
      final out = await client.call('update.status');
      if (out is Map) {
        applyUpdateStatus(Map<String, dynamic>.from(out));
      }
    } catch (e) {
      // Offline or old backend: updates stay quiet, never block the UI.
    }
  }

  Future<void> checkForUpdates() => _guarded(() async {
        final out = await client.call('update.check');
        if (out is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(out));
        }
      });

  Future<void> downloadUpdate() => _guarded(() async {
        final out = await client.call('update.download');
        if (out is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(out));
        }
      });

  Future<void> cancelUpdate() => _guarded(() async {
        final out = await client.call('update.cancel');
        if (out is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(out));
        }
      });

  Future<void> installUpdate() => _guarded(() async {
        final out = await client.call('update.install');
        if (out is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(out));
        }
      });

  Future<void> dismissUpdate() => _guarded(() async {
        final out = await client.call('update.dismiss');
        if (out is Map) {
          applyUpdateStatus(Map<String, dynamic>.from(out));
        } else {
          updateState = UpdateStates.idle;
          updateInfo = null;
          updateStaged = false;
          notifyListeners();
        }
      });

  String updateLastCheckLabel() {
    if (updateLastCheck <= 0) return 'Never';
    final at =
        DateTime.fromMillisecondsSinceEpoch(updateLastCheck * 1000);
    final now = DateTime.now();
    final diff = now.difference(at);
    if (diff.inMinutes < 1) return 'Just now';
    if (diff.inHours < 1) return '${diff.inMinutes}m ago';
    if (diff.inDays < 1) return '${diff.inHours}h ago';
    return '${at.day.toString().padLeft(2, '0')}.${at.month.toString().padLeft(2, '0')}.${at.year}';
  }

  // --- settings ---

  Future<void> updateSettings(AppSettings next) => _guarded(() async {
        final out = await client.call('settings.update', next.toJson());
        if (out is Map) {
          settings =
              AppSettings.fromJson(Map<String, dynamic>.from(out));
          notifyListeners();
        }
      });

  // --- logs ---

  Future<void> refreshLogs() async {
    try {
      final out = await client.call('logs.get');
      if (out is List) {
        logs = [
          for (final e in out)
            LogEntry.fromJson(Map<String, dynamic>.from(e as Map))
        ];
        notifyListeners();
      }
    } catch (e) {
      lastError = friendlyError(e);
      notifyListeners();
    }
  }

  Future<void> clearLogs() => _guarded(() async {
        await client.call('logs.clear');
        logs = const [];
        notifyListeners();
      });

  List<LogEntry> get visibleLogs =>
      logs.where((e) => e.level >= logMinLevel).toList();

  @override
  void dispose() {
    _disposed = true;
    _events?.cancel();
    _uiSaveDebounce?.cancel();
    client.dispose();
    super.dispose();
  }
}
