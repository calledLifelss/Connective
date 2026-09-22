import 'dart:async';
import 'dart:io';

/// Launches and supervises the Go backend (connectived) for the desktop
/// app: if no daemon answers the socket, start the bundled (or
/// configured) binary; kill our child on app exit so no backend is ever
/// orphaned. An already-running daemon (second window, manual start) is
/// attached to, never duplicated.
class BackendLauncher {
  Process? _child;

  /// Socket path the UI will use. Must mirror the daemon's
  /// `platform.SocketPath()` exactly, or the UI can never find its
  /// backend:
  /// - `CONNECTIVE_SOCK` wins (tests, portable layouts);
  /// - else `CONNECTIVE_DATA_DIR/connectived.sock` (the daemon honors
  ///   this override on every OS);
  /// - else the platform default: `%LOCALAPPDATA%\Connective` on
  ///   Windows (the daemon ignores $HOME there), `$HOME/.local/share`
  ///   elsewhere.
  static String socketPath() {
    final override = Platform.environment['CONNECTIVE_SOCK'];
    if (override != null && override.isNotEmpty) return override;
    final dataDir = Platform.environment['CONNECTIVE_DATA_DIR'];
    if (dataDir != null && dataDir.isNotEmpty) {
      return _join(dataDir, 'connectived.sock');
    }
    if (Platform.isWindows) {
      final base = Platform.environment['LOCALAPPDATA'] ??
          Platform.environment['USERPROFILE'] ??
          Platform.environment['HOME'] ??
          Directory.systemTemp.path;
      return _join(_join(base, 'Connective'), 'connectived.sock');
    }
    final home = Platform.environment['HOME'] ?? '/tmp';
    return '$home/.local/share/connective/connectived.sock';
  }

  static String _join(String a, String b) {
    final sep = Platform.pathSeparator;
    if (a.endsWith(sep)) return '$a$b';
    return '$a$sep$b';
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
      // kill() without args is portable (SIGTERM on POSIX, terminate
      // on Windows); explicit signals throw on Windows.
      c.kill();
      await c.exitCode.timeout(const Duration(seconds: 3),
          onTimeout: () {
        try {
          c.kill(ProcessSignal.sigkill);
        } catch (_) {
          c.kill();
        }
        return c.exitCode;
      });
    } catch (_) {}
  }
}
