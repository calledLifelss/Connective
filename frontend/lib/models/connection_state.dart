/// Backend connection states (mirrors backend/internal/connection).
/// The UI renders these verbatim and never invents its own.
class ConnectionStates {
  static const disconnected = 'disconnected';
  static const testing = 'testing';
  static const selecting = 'selecting';
  static const connecting = 'connecting';
  static const startingCore = 'starting-core';
  static const initializingTun = 'initializing-tun';
  static const applyingRouting = 'applying-routing';
  static const connected = 'connected';
  static const degraded = 'degraded';
  static const reconnecting = 'reconnecting';
  static const switchingServer = 'switching-server';
  static const disconnecting = 'disconnecting';
  static const error = 'error';

  static const _busy = {
    testing,
    selecting,
    connecting,
    startingCore,
    initializingTun,
    applyingRouting,
    reconnecting,
    switchingServer,
    disconnecting,
  };

  /// Human label for the connect button / status header.
  static String label(String state) {
    switch (state) {
      case disconnected:
        return 'Disconnected';
      case testing:
        return 'Testing…';
      case selecting:
        return 'Selecting server…';
      case connecting:
        return 'Connecting…';
      case startingCore:
        return 'Starting core…';
      case initializingTun:
        return 'Initializing TUN…';
      case applyingRouting:
        return 'Applying routing…';
      case connected:
        return 'Connected';
      case degraded:
        return 'Degraded';
      case reconnecting:
        return 'Reconnecting…';
      case switchingServer:
        return 'Switching server…';
      case disconnecting:
        return 'Disconnecting…';
      case error:
        return 'Error';
      default:
        return state;
    }
  }

  static bool isConnected(String s) =>
      s == connected || s == degraded;

  static bool isBusy(String s) => _busy.contains(s);
}
