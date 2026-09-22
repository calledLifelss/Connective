import 'dart:async';
import 'dart:convert';
import 'dart:io';

/// Socket IPC client speaking protocol v1 (docs/IPC_CONTRACT.md).
///
/// Phase-1: full framing, request/response, event stream plumbing.
/// Wiring into [AppStore] happens in phase 2 alongside the daemon's
/// remaining method handlers.
class DaemonClient {
  static const protocolVersion = '1';

  /// Calls that legitimately outlive the snappy default: connect runs
  /// latency probes, core startup and TUN/routing verification
  /// (tens of seconds); disconnect stops the core plus privileged
  /// cleanup; a subscription refresh fetches over the network with a
  /// 5-minute daemon-side budget. Timing these out at 10s turned every
  /// real-world connect into a false "Backend unavailable".
  static const _timeouts = {
    'connection.connect': Duration(seconds: 120),
    'connection.disconnect': Duration(seconds: 60),
    'subscriptions.update': Duration(seconds: 330),
  };
  static const _defaultTimeout = Duration(seconds: 10);

  Socket? _socket;
  final _pending = <String, Completer<Map<String, dynamic>?>>{};
  final _events = StreamController<Map<String, dynamic>>.broadcast();
  int _nextId = 0;
  String _buffer = '';

  /// Broadcast stream of daemon events (event.state, event.stats, …).
  Stream<Map<String, dynamic>> get events => _events.stream;

  bool get isConnected => _socket != null;

  Future<void> connect(String socketPath) async {
    final address = InternetAddress(socketPath, type: InternetAddressType.unix);
    _socket = await Socket.connect(address, 0);
    _socket!.listen(_onData,
        onError: (_) => _drop(), onDone: () => _drop(), cancelOnError: true);
  }

  void _onData(List<int> chunk) {
    _buffer += utf8.decode(chunk, allowMalformed: true);
    var idx = _buffer.indexOf('\n');
    while (idx >= 0) {
      final line = _buffer.substring(0, idx);
      _buffer = _buffer.substring(idx + 1);
      _onFrame(line);
      idx = _buffer.indexOf('\n');
    }
  }

  void _onFrame(String line) {
    if (line.trim().isEmpty) return;
    Map<String, dynamic> frame;
    try {
      frame = Map<String, dynamic>.from(json.decode(line) as Map);
    } catch (_) {
      return;
    }
    final id = frame['id'] as String?;
    if (id != null && _pending.containsKey(id)) {
      _pending.remove(id)!.complete(frame);
    } else {
      _events.add(frame);
    }
  }

  /// One request/response round trip. Returns the decoded payload
  /// (Map or List) or null. Throws on daemon-side errors.
  Future<dynamic> call(String method,
      [Map<String, dynamic>? payload]) {
    final socket = _socket;
    if (socket == null) throw StateError('not connected to daemon');
    final id = '${++_nextId}';
    final frame = {
      'v': protocolVersion,
      'id': id,
      'type': method,
      if (payload != null) 'payload': payload,
    };
    final done = Completer<Map<String, dynamic>?>();
    _pending[id] = done;
    socket.write('${json.encode(frame)}\n');
    final timeout = _timeouts[method] ?? _defaultTimeout;
    return done.future.timeout(timeout, onTimeout: () {
      _pending.remove(id);
      throw TimeoutException('ipc $method timed out');
    }).then((resp) {
      if (resp != null &&
          resp['error'] is String &&
          (resp['error'] as String).isNotEmpty) {
        throw Exception(resp['error'] as String);
      }
      return resp?['payload'];
    });
  }

  void _drop() {
    _socket?.destroy();
    _socket = null;
    for (final c in _pending.values) {
      if (!c.isCompleted) c.completeError(StateError('daemon gone'));
    }
    _pending.clear();
  }

  /// Drop the socket but keep the client reusable (see reconnect).
  /// Call [dispose] once the client is permanently retired.
  Future<void> close() async {
    _drop();
  }

  Future<void> dispose() async {
    _drop();
    await _events.close();
  }
}
