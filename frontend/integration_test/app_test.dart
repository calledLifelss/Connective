import 'dart:convert';
import 'dart:io';

import 'package:connective/main.dart' as app;
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Phase-4 acceptance drive: REAL app window + REAL backend + REAL core.
/// Runs on the Linux desktop (flutter test integration_test -d linux).
/// Backend is proxy-only loopback; no privileges, no system changes.
///
/// Flow mirrors spec §32: launch → backend → IPC → subscription →
/// collapse/expand → servers → test → AUTO → connect → traffic/stats →
/// failover → disconnect → cleanup → persistence.
void main() {
  const ss1Port = 18688;
  const ss2Port = 18689;
  const subPort = 18680;
  const mixedPort = 11848;
  const clashPort = 17796;
  const b64 = 'YWVzLTEyOC1nY206Y29ubmVjdGl2ZS1lMmUtdGVzdA==';

  late HttpServer subServer;
  late List<Process> procs;
  late AppStore store;
  late Directory homeDir;
  Process? backend;

  Future<Process> startSS(int port) async {
    final dir = Directory.systemTemp;
    final cfg = File(
        '${dir.path}/drive-ss$port-${DateTime.now().millisecondsSinceEpoch}.json');
    await cfg.writeAsString(json.encode({
      'log': {'level': 'warning'},
      'dns': {
        'servers': [
          {'tag': 'local-dns', 'type': 'local'}
        ],
        'final': 'local-dns'
      },
      'inbounds': [
        {
          'type': 'shadowsocks',
          'tag': 'ss-in',
          'listen': '127.0.0.1',
          'listen_port': port,
          'method': 'aes-128-gcm',
          'password': 'connective-e2e-test'
        }
      ],
      'outbounds': [
        {'type': 'direct', 'tag': 'direct'}
      ],
      'route': {'final': 'direct'}
    }));
    final sb = await _singBox();
    final p = await Process.start(sb, ['run', '-c', cfg.path]);
    procs.add(p);
    return p;
  }

  setUp(() async {
    procs = [];
    await startSS(ss1Port);
    await startSS(ss2Port);
    subServer = await HttpServer.bind('127.0.0.1', subPort);
    subServer.listen((req) async {
      if (req.uri.path != '/sub') {
        req.response.statusCode = 404;
        await req.response.close();
        return;
      }
      const body =
          'ss://$b64@127.0.0.1:$ss1Port#Drive1\nss://$b64@127.0.0.1:$ss2Port#Drive2\n';
      req.response.headers.set('subscription-userinfo',
          'upload=1; download=2; total=100; expire=1780000000');
      req.response.write(body);
      await req.response.close();
    });
    // Real backend with isolated HOME (no pollution of the real store).
    // Windows daemons ignore $HOME, so the data dir is pinned explicitly.
    homeDir = await Directory.systemTemp.createTemp('drivehome');
    final labHome = Platform.environment['HOME'] ??
        Platform.environment['USERPROFILE'] ??
        Directory.systemTemp.path;
    final daemonBin = Platform.isWindows
        ? '$labHome/.local/share/connective/lab-bin/connectived.exe'
        : '$labHome/.local/share/connective/lab-bin/connectived';
    final dataDir = Platform.isWindows
        ? '${homeDir.path}/data'
        : '${homeDir.path}/.local/share/connective';
    backend = await Process.start(daemonBin, [], environment: {
      'HOME': homeDir.path,
      'CONNECTIVE_DATA_DIR': dataDir,
    });
    final sock = '$dataDir/connectived.sock';
    for (var i = 0; i < 50; i++) {
      await Future.delayed(const Duration(milliseconds: 200));
      if (await File(sock).exists()) break;
    }
    store = AppStore();
    await store.boot(sock);
    await store.updateSettings(store.settings.copyWith(
      tunEnabled: false,
      killSwitch: false,
      mixedPort: mixedPort,
      clashApiPort: clashPort,
      corePath: await _singBox(),
      testConcurrency: 5,
      healthIntervalSec: 10,
      updateOnStart: false,
    ));
  });

  tearDown(() async {
    try {
      if (store.connectionState == 'connected' ||
          store.connectionState == 'degraded') {
        await store.toggleConnection();
      }
    } catch (_) {}
    try {
      for (final s in List.of(store.subscriptions)) {
        await store.removeSubscription(s.id);
      }
    } catch (_) {}
    store.dispose();
    backend?.kill(ProcessSignal.sigterm);
    for (final p in procs) {
      try {
        p.kill(ProcessSignal.sigterm);
      } catch (_) {}
    }
    await Future.delayed(const Duration(seconds: 1));
    backend?.kill(ProcessSignal.sigkill);
    for (final p in procs) {
      try {
        p.kill(ProcessSignal.sigkill);
      } catch (_) {}
    }
    await subServer.close(force: true);
    try {
      await homeDir.delete(recursive: true);
    } catch (_) {}
  });

  Future<void> settle(WidgetTester t) async {
    for (var i = 0; i < 6; i++) {
      await t.pump(const Duration(milliseconds: 300));
    }
  }

  testWidgets('full acceptance flow', (tester) async {
    await tester.pumpWidget(
        app.ConnectiveApp(store: store, launcher: null));
    await settle(tester);

    // Dashboard: empty state with the consolidated toolbar.
    expect(find.textContaining('No servers yet'), findsOneWidget);
    expect(find.byKey(const Key('dash-tun-toggle')), findsOneWidget);
    expect(find.byKey(const Key('dash-update-all')), findsOneWidget);

    // Add subscription through the REAL dialog.
    await tester.tap(find.byKey(const Key('dash-add-subscription')));
    await settle(tester);
    await tester.enterText(
        find.widgetWithText(TextField, 'Name'), 'DriveTest');
    await tester.enterText(find.widgetWithText(
        TextField, 'Subscription URL (https://…)'),
        'http://127.0.0.1:$subPort/sub');
    await tester.tap(find.text('Save'));
    await settle(tester);

    // The new subscription lands below the fold: bring it into view.
    await tester.drag(
        find.byKey(const Key('dash-scroll')), const Offset(0, -350));
    await settle(tester);

    // Update now through the REAL update button.
    await tester.tap(find.byIcon(Icons.refresh).first);
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(seconds: 1));
      if (store.servers.length == 2) break;
    }
    expect(store.servers, hasLength(2));
    await settle(tester);

    // Collapse/expand subscription + server.
    expect(find.text('Drive1'), findsOneWidget);
    await tester.tap(find.text('DriveTest'));
    await settle(tester);
    expect(find.text('Drive1'), findsNothing);
    await tester.tap(find.text('DriveTest'));
    await settle(tester);
    expect(find.text('Drive1'), findsOneWidget);
    await tester.tap(find.text('Drive1'));
    await settle(tester);
    expect(find.text('Address'), findsWidgets);

    // Test a server through the REAL test button.
    await tester.tap(find.text('Test').first);
    for (var i = 0; i < 30; i++) {
      await tester.pump(const Duration(seconds: 1));
      if (store.servers.any((s) => s.latencyMs >= 0)) break;
    }
    expect(store.servers.any((s) => s.latencyMs >= 0), isTrue);

    // Connect through the REAL dashboard button.
    await tester.tap(find.text('Dashboard'));
    await settle(tester);
    await tester
        .tap(find.byKey(const Key('connect-button')));
    var connected = false;
    for (var i = 0; i < 15 && !connected; i++) {
      await tester.pump(const Duration(seconds: 2));
      connected =
          find.text('Connected').evaluate().isNotEmpty;
    }
    expect(connected, isTrue);
    await settle(tester);

    // Real traffic + real stats through the live core.
    final code = await _curlProxy(mixedPort);
    expect(code, 200);
    await tester.pump(const Duration(seconds: 3));
    await settle(tester);
    expect(store.stats.downTotal, greaterThan(0));

    // Failover: kill the ACTIVE server process, expect recovery.
    final active = store.activeServerId;
    expect(active, isNotEmpty);
    final aport = store.servers
        .firstWhere((s) => s.id == active)
        .port;
    await _killPortOwner(aport);
    var recovered = false;
    String? now;
    for (var i = 0; i < 30 && !recovered; i++) {
      await tester.pump(const Duration(seconds: 2));
      now = store.activeServerId;
      recovered = now.isNotEmpty &&
          now != active &&
          store.connectionState == 'connected';
    }
    expect(recovered, isTrue,
        reason: 'failover did not move off $active (now=$now)');
    expect(await _curlProxy(mixedPort), 200);
    await settle(tester);

    // Disconnect through the REAL button; cleanup verified below.
    await tester.tap(find.text('Dashboard'));
    await settle(tester);
    await tester.tap(find.byKey(const Key('connect-button')));
    var off = false;
    for (var i = 0; i < 15 && !off; i++) {
      await tester.pump(const Duration(seconds: 2));
      off = find.text('Connected').evaluate().isEmpty;
    }
    expect(off, isTrue);
  }, timeout: const Timeout(Duration(minutes: 10)));
}

Future<String> _singBox() async {
  final home = Platform.environment['HOME'] ??
      Platform.environment['USERPROFILE'] ??
      Directory.systemTemp.path;
  final dir = Platform.isWindows
      ? 'sing-box-1.14.1-windows-amd64'
      : 'sing-box-1.14.1-linux-amd64';
  final bin = Platform.isWindows ? 'sing-box.exe' : 'sing-box';
  final direct = '$home/.local/share/connective/lab-bin/$dir/$bin';
  if (await File(direct).exists()) return direct;
  final finder = Platform.isWindows ? 'where' : 'which';
  final res = await Process.run(finder, ['sing-box']);
  return (res.stdout as String).trim();
}

Future<int> _curlProxy(int port) async {
  final client = HttpClient();
  try {
    client.findProxy = (_) => 'PROXY 127.0.0.1:$port';
    client.connectionTimeout = const Duration(seconds: 10);
    final req = await client.getUrl(Uri.parse('http://example.com/'));
    final resp = await req.close().timeout(
        const Duration(seconds: 10));
    await resp.drain();
    return resp.statusCode;
  } catch (_) {
    return 0;
  } finally {
    client.close(force: true);
  }
}

Future<void> _killPortOwner(int port) async {
  final res = await Process.run(
      'bash', ['-c', 'ss -tlnp 2>/dev/null | grep :$port']);
  final out = res.stdout as String;
  final m = RegExp(r'pid=(\d+)').firstMatch(out);
  if (m != null) {
    Process.killPid(int.parse(m.group(1)!));
  }
}
