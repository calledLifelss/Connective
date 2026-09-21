import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Shared live-backend harness: loopback sing-box servers + real
/// connectived, proxy-only (no privileges, no system changes).
/// Nothing is mocked; ports are unique per run.
class Lab {
  late Directory dir;
  late String sock;
  Process? daemon;

  /// PID of the test daemon for precise signaling (never pattern-match).
  int? get daemonPid => daemon?.pid;
  final List<Process> procs = [];

  late int ss1Port;
  late int ss2Port;
  late int subPort;
  late int mixedPort;
  late int clashPort;

  static Future<int> _freePort() async {
    final s = await ServerSocket.bind('127.0.0.1', 0);
    final p = s.port;
    await s.close();
    return p;
  }

  static String get home {
    // GitHub Windows runners (and Windows generally) have no $HOME;
    // PowerShell ~ and our CI layout use USERPROFILE instead.
    return Platform.environment['HOME'] ??
        Platform.environment['USERPROFILE'] ??
        Directory.systemTemp.path;
  }

  String get labBin => '$home/.local/share/connective/lab-bin';

  String get daemonBin =>
      Platform.isWindows ? '$labBin/connectived.exe' : '$labBin/connectived';

  String get singBox {
    final dir = Platform.isWindows
        ? 'sing-box-1.14.1-windows-amd64'
        : 'sing-box-1.14.1-linux-amd64';
    final bin = Platform.isWindows ? 'sing-box.exe' : 'sing-box';
    return '$labBin/$dir/$bin';
  }

  /// Extra environment for the daemon (merged over the test process
  /// environment). Update tests use it to point CONNECTIVE_UPDATE_PROVIDER
  /// at fixture dirs and enable CONNECTIVE_UPDATE_TEST_APPLY.
  Future<void> start({Map<String, String>? extraEnv}) async {
    ss1Port = await _freePort();
    ss2Port = await _freePort();
    subPort = await _freePort();
    mixedPort = await _freePort();
    clashPort = await _freePort();
    dir = await Directory.systemTemp.createTemp('conntest');
    final sub = File('${dir.path}/sub.txt');
    const b64 =
        'YWVzLTEyOC1nY206Y29ubmVjdGl2ZS1lMmUtdGVzdA==';
    await sub.writeAsString(
        'ss://$b64@127.0.0.1:$ss1Port#T1\nss://$b64@127.0.0.1:$ss2Port#T2\n');
    final srv = await HttpServer.bind('127.0.0.1', subPort);
    srv.listen((req) async {
      if (req.uri.path != '/sub') {
        req.response.statusCode = 404;
        await req.response.close();
        return;
      }
      final mode = req.uri.queryParameters['mode'] ?? 'ok';
      if (mode == 'garbage') {
        req.response.write('this is not a subscription {{{{{');
        await req.response.close();
        return;
      }
      if (mode == 'empty') {
        req.response.write('\n\n');
        await req.response.close();
        return;
      }
      final count =
          int.tryParse(req.uri.queryParameters['n'] ?? '0') ?? 0;
      if (count > 0) {
        // Deterministic synthetic nodes for large-list tests.
        final buf = StringBuffer();
        for (var i = 0; i < count.clamp(1, 2000); i++) {
          final uuid =
              '11111111-2222-4333-8444-${i.toRadixString(16).padLeft(12, '0')}';
          buf.writeln(
              'vless://$uuid@10.99.0.${(i % 250) + 1}:${1000 + (i % 60000)}?encryption=none#Perf-$i');
        }
        req.response.write(buf.toString());
        await req.response.close();
        return;
      }
      req.response.headers.set('subscription-userinfo',
          'upload=1; download=2; total=100; expire=1780000000');
      await req.response.addStream(sub.openRead());
      await req.response.close();
    });

    Future<Process> ss(int port) async {
      final cfg = File('${dir.path}/ss$port.json');
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
      final p = await Process.start(singBox, ['run', '-c', cfg.path]);
      procs.add(p);
      return p;
    }

    await ss(ss1Port);
    await ss(ss2Port);
    await Future.delayed(const Duration(seconds: 1));

    // Windows daemons ignore $HOME (LOCALAPPDATA rules), so point the
    // data dir explicitly for isolation on every OS.
    dataDir = Platform.isWindows
        ? '${dir.path}/data'
        : '${dir.path}/.local/share/connective';
    daemon = await Process.start(daemonBin, [], environment: {
      'HOME': dir.path,
      'CONNECTIVE_DATA_DIR': dataDir,
      ...?extraEnv
    });
    // Daemon stderr -> file for post-mortem (flutter swallows child
    // stdio). Kept on failure via CONNECTIVE_KEEP_LAB=1.
    _daemonLog =
        File('${dir.path}/daemon-stderr.log').openWrite();
    daemon!.stderr.listen(_daemonLog!.add);
    sock = '$dataDir/connectived.sock';
    for (var i = 0; i < 50 && !await File(sock).exists(); i++) {
      await Future.delayed(const Duration(milliseconds: 200));
    }
    assert(await File(sock).exists(), 'daemon socket missing');

    // Listener to keep the HttpServer alive.
    _subServer = srv;
  }

  /// Data dir the daemon was pointed at (for tests that restart it).
  late String dataDir;

  HttpServer? _subServer;
  IOSink? _daemonLog;

  Future<void> stop() async {
    daemon?.kill(ProcessSignal.sigterm);
    for (final p in procs) {
      p.kill(ProcessSignal.sigterm);
    }
    await Future.delayed(const Duration(seconds: 1));
    daemon?.kill(ProcessSignal.sigkill);
    for (final p in procs) {
      p.kill(ProcessSignal.sigkill);
    }
    await _subServer?.close(force: true);
    await _daemonLog?.flush();
    await _daemonLog?.close();
    if (Platform.environment['CONNECTIVE_KEEP_LAB'] == '1') {
      // ignore: avoid_print
      print('LAB kept at ${dir.path}');
      return;
    }
    try {
      await dir.delete(recursive: true);
    } catch (_) {}
  }
}
