import 'dart:async';
import 'dart:io';

/// Launches and supervises the Go backend (connectived) for the desktop
/// app: if no daemon answers the socket, start the bundled (or
/// configured) binary; kill our child on app exit so no backend is ever
/// orphaned. An already-running daemon (second window, manual start) is
/// attached to, never duplicated.
class BackendLauncher {
  Process? _child;

  /// Socket path the UI will use.
  static String socketPath() {
    final override = Platform.environment['CONNECTIVE_SOCK'];
    if (override != null && override.isNotEmpty) return override;
    final home = Platform.environment['HOME'] ?? '/tmp';
    return '$home/.local/share/connective/connectived.sock';
  }

  /// Backend binary resolution: explicit path, sibling of the Flutter
  /// executable (bundled layout), then PATH.
  static String? resolveBackend([String? explicit]) {
    if (explicit != null && explicit.isNotEmpty) {
      final f = File(explicit);
      if (f.existsSync()) return f.path;
    }
    try {
      final exeDir =
          File(Platform.resolvedExecutable).parent.path;
      for (final name in ['connectived', 'connectived.exe']) {
        final f = File('$exeDir/$name');
        if (f.existsSync()) return f.path;
      }
    } catch (_) {}
    try {
      final which = Process.runSync(
          Platform.isWindows ? 'where' : 'which', ['connectived']);
      final out = (which.stdout as String).trim().split('\n').first;
      if (which.exitCode == 0 && out.isNotEmpty) return out;
    } catch (_) {}
    return null;
  }

  static Future<bool> socketAlive(String path) async {
    try {
      final s = await Socket.connect(
          InternetAddress(path, type: InternetAddressType.unix), 0);
      s.destroy();
      return true;
    } catch (_) {
      return false;
    }
  }

  /// Ensure a daemon answers; spawn one if needed. Returns true when the
  /// socket answers afterwards.
  Future<bool> ensure(String? explicitPath) async {
    final sock = socketPath();
    if (await socketAlive(sock)) return true;
    final bin = resolveBackend(explicitPath);
    if (bin == null) return false;
    try {
      _child = await Process.start(bin, [],
          mode: ProcessStartMode.detachedWithStdio);
    } catch (_) {
      return false;
    }
    for (var i = 0; i < 50; i++) {
      await Future.delayed(const Duration(milliseconds: 200));
      if (await socketAlive(sock)) return true;
    }
    return await socketAlive(sock);
  }

  /// Stop a daemon WE started (never touch foreign ones).
  Future<void> stopOwned() async {
    final c = _child;
    _child = null;
    if (c == null) return;
    try {
      c.kill(ProcessSignal.sigterm);
      await c.exitCode.timeout(const Duration(seconds: 3),
          onTimeout: () {
        c.kill(ProcessSignal.sigkill);
        return c.exitCode;
      });
    } catch (_) {}
  }
}
