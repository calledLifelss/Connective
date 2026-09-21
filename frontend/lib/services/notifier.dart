import 'package:flutter_local_notifications/flutter_local_notifications.dart';

/// Desktop notifications (§23): connected, unexpected disconnect,
/// failover, subscription update success/failure. Best-effort and
/// silent on failure (tests, headless, missing daemons); never spam —
/// only state transitions and update completions notify.
class Notifier {
  static final FlutterLocalNotificationsPlugin _plugin =
      FlutterLocalNotificationsPlugin();
  static bool _ready = false;

  /// Initialize once at startup. Never throws.
  static Future<void> init() async {
    try {
      const settings = InitializationSettings(
        linux: LinuxInitializationSettings(
            defaultActionName: 'Open Connective'),
      );
      await _plugin.initialize(settings: settings);
      _ready = true;
    } catch (_) {
      _ready = false;
    }
  }

  static Future<void> show(String title, String body) async {
    if (!_ready) return;
    try {
      await _plugin.show(
        id: DateTime.now().millisecondsSinceEpoch ~/ 1000,
        title: title,
        body: body,
        notificationDetails: const NotificationDetails(
          linux: LinuxNotificationDetails(),
        ),
      );
    } catch (_) {}
  }

  /// Test hook: whether the backend accepted initialization.
  static bool get ready => _ready;
}
