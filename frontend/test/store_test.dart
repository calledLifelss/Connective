import 'dart:io';

import 'package:connective/state/app_store.dart';
import 'package:flutter_test/flutter_test.dart';

import 'lab.dart';

/// Store-level integration: every IPC method the UI uses, against the
/// real backend. No mocks.
void main() {
  late Lab lab;
  late AppStore store;

  setUp(() async {
    lab = Lab();
    await lab.start();
    store = AppStore();
    await store.boot(lab.sock);
  });

  tearDown(() async {
    store.dispose();
    await lab.stop();
  });

  Future<void> seedServers() async {
    await store.addSubscription(
        'Lab', 'http://127.0.0.1:${lab.subPort}/sub');
    await store.updateSubscriptions();
  }

  test('settings round-trip', () async {
    expect(store.settings.mixedPort, isNonZero);
    await store.updateSettings(
        store.settings.copyWith(testConcurrency: 7));
    expect(store.settings.testConcurrency, 7);
  });

  test('subscription add + update + list', () async {
    await store.addSubscription(
        'Lab', 'http://127.0.0.1:${lab.subPort}/sub');
    expect(store.subscriptions, hasLength(1));
    await store.updateSubscriptions();
    expect(store.servers, hasLength(2));
    final sub = store.subscriptions.first;
    expect(sub.traffic.hasLimit, isTrue);
    expect(sub.traffic.total, 100);
  });

  test('server test produces real latency', () async {
    await seedServers();
    await store.testServers();
    for (var i = 0;
        i < 40 && store.testingServers.isNotEmpty;
        i++) {
      await Future.delayed(const Duration(milliseconds: 250));
    }
    expect(store.testingServers, isEmpty);
    expect(store.servers.every((s) => s.latencyMs >= 0), isTrue);
  });

  test('server import / duplicate / export / remove', () async {
    await seedServers();
    const link =
        'vless://11111111-2222-4333-8444-555555555555@10.9.9.9:443?encryption=none#Imp';
    await store.importServer(link);
    expect(store.servers.any((s) => s.address == '10.9.9.9'), isTrue);
    final imp =
        store.servers.firstWhere((s) => s.address == '10.9.9.9');
    await store.duplicateServer(imp.id);
    expect(
        store.servers
            .where((s) => s.address == '10.9.9.9')
            .length,
        2);
    final exported = await store.exportServer(imp.id);
    expect(exported, startsWith('vless://'));
    await store.removeServer(imp.id);
    expect(store.servers.any((s) => s.id == imp.id), isFalse);
    // Subscription-owned servers are protected.
    final owned = store.servers.first;
    store.lastError = null;
    await store.removeServer(owned.id);
    expect(store.lastError, isNotNull);
    expect(store.servers.any((s) => s.id == owned.id), isTrue);
  });

  test('subscription edit + ui state', () async {
    await seedServers();
    final id = store.subscriptions.first.id;
    await store.editSubscription(id, name: 'Renamed', enabled: false);
    expect(store.subscriptions.first.name, 'Renamed');
    expect(store.subscriptions.first.enabled, isFalse);
    store.subscriptionExpanded[id] = false;
    store.persistUi();
    await Future.delayed(const Duration(milliseconds: 800));
    final raw = await store.client.call('ui.get');
    expect((raw as Map)['expandedSubs'][id], isFalse);
  });

  test('connect + traffic + disconnect', () async {
    await seedServers();
    await store.updateSettings(store.settings.copyWith(
      tunEnabled: false,
      killSwitch: false,
      mixedPort: lab.mixedPort,
      clashApiPort: lab.clashPort,
      corePath: lab.singBox,
      healthIntervalSec: 30,
    ));
    await store.toggleConnection();
    await Future.delayed(const Duration(seconds: 5));
    expect(store.connectionState, 'connected');
    await store.refreshAll();
    // Real traffic through the live core is covered by stats ticks;
    // poll once for cumulative totals.
    final stats =
        await store.client.call('stats.get') as Map<String, dynamic>;
    expect(stats['connected'], isTrue);
    await store.toggleConnection();
    var off = false;
    for (var i = 0; i < 10 && !off; i++) {
      await Future.delayed(const Duration(seconds: 3));
      await store.refreshAll();
      off = store.connectionState == 'disconnected';
    }
    expect(store.connectionState, 'disconnected');
  }, timeout: const Timeout(Duration(minutes: 2)));

  test('logs flow and clear', () async {
    await store.refreshLogs();
    expect(store.logs, isNotEmpty);
    await store.clearLogs();
    expect(store.logs, isEmpty);
  });

  test('subscription failures preserve working data', () async {
    await seedServers();
    expect(store.servers, hasLength(2));
    // Unreachable URL.
    store.lastError = null;
    await store.addSubscription('Bad', 'http://127.0.0.1:1/sub');
    await store.updateSubscriptions('__nonexistent__');
    await store.updateSubscriptions(
        store.subscriptions.firstWhere((s) => s.name == 'Bad').id);
    expect(store.lastError, isNotNull);
    expect(store.servers, hasLength(2));
    // Malformed import links are rejected with friendly errors.
    store.lastError = null;
    await store.importServer('not-a-link-at-all');
    expect(store.lastError, isNotNull);
    expect(store.servers, hasLength(2));
    store.lastError = null;
    await store.importServer('wireguard://key@host:1234#W');
    expect(store.lastError, isNotNull);
    expect(store.servers, hasLength(2));
    // Cleanup the bad sub.
    await store.removeSubscription(
        store.subscriptions.firstWhere((s) => s.name == 'Bad').id);
    expect(store.servers, hasLength(2));
    // Garbage and empty bodies fail gracefully, preserving data.
    await store.addSubscription(
        'Garbage', 'http://127.0.0.1:${lab.subPort}/sub?mode=garbage');
    final gid =
        store.subscriptions.firstWhere((s) => s.name == 'Garbage').id;
    store.lastError = null;
    await store.updateSubscriptions(gid);
    expect(store.lastError, isNotNull);
    expect(store.servers, hasLength(2));
    await store.removeSubscription(gid);
    await store.addSubscription(
        'Empty', 'http://127.0.0.1:${lab.subPort}/sub?mode=empty');
    final eid =
        store.subscriptions.firstWhere((s) => s.name == 'Empty').id;
    store.lastError = null;
    await store.updateSubscriptions(eid);
    expect(store.lastError, isNotNull);
    expect(store.servers, hasLength(2));
    await store.removeSubscription(eid);
  }, timeout: const Timeout(Duration(minutes: 2)));

  test('backend death marks stale, reconnect recovers', () async {
    await seedServers();
    expect(store.backendAlive, isTrue);
    // SIGKILL this test's daemon by exact PID (never pattern-match).
    Process.killPid(lab.daemonPid!);
    await Future.delayed(const Duration(seconds: 1));
    await store.refreshAll();
    expect(store.backendAlive, isFalse);
    expect(store.lastError, isNotNull);
    // Restart the daemon on the same HOME (persisted state reloads).
    final d = await Process.start(lab.daemonBin, [],
        environment: {'HOME': lab.dir.path});
    try {
      for (var i = 0; i < 30; i++) {
        await Future.delayed(const Duration(milliseconds: 300));
        await store.reconnect();
        if (store.backendAlive) break;
      }
      expect(store.backendAlive, isTrue);
      expect(store.servers, hasLength(2));
    } finally {
      d.kill(ProcessSignal.sigterm);
    }
  }, timeout: const Timeout(Duration(minutes: 2)));
}
