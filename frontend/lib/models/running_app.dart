/// One distinct running program, as reported by the daemon's
/// `apps.list` (backend/internal/apps). Identity is the normalized exe
/// basename; the UI keeps no side inventory.
class RunningApp {
  final String id;
  final String name;
  final int count;

  const RunningApp({required this.id, this.name = '', this.count = 1});

  factory RunningApp.fromJson(Map<String, dynamic> j) => RunningApp(
        id: (j['id'] as String? ?? '').trim(),
        name: j['name'] as String? ?? '',
        count: (j['count'] as num?)?.toInt() ?? 1,
      );

  String get displayName => name.isEmpty ? id : name;
}

/// One merged split-tunnel picker row: running apps first, then
/// configured-but-not-running entries.
class SplitAppRow {
  final String id;
  final String name;
  final bool running;
  final int count;
  final bool selected;

  const SplitAppRow({
    required this.id,
    required this.name,
    this.running = false,
    this.count = 0,
    this.selected = false,
  });
}
