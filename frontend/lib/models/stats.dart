/// Live traffic snapshot (stats.get / event.stats).
class TrafficStats {
  final bool connected;
  final int upRate;
  final int downRate;
  final int upTotal;
  final int downTotal;
  final int durationMs;

  const TrafficStats({
    this.connected = false,
    this.upRate = 0,
    this.downRate = 0,
    this.upTotal = 0,
    this.downTotal = 0,
    this.durationMs = 0,
  });

  factory TrafficStats.fromJson(Map<String, dynamic> j) => TrafficStats(
        connected: j['connected'] as bool? ?? false,
        upRate: (j['upRate'] as num?)?.toInt() ?? 0,
        downRate: (j['downRate'] as num?)?.toInt() ?? 0,
        upTotal: (j['upTotal'] as num?)?.toInt() ?? 0,
        downTotal: (j['downTotal'] as num?)?.toInt() ?? 0,
        durationMs: (j['durationMs'] as num?)?.toInt() ?? 0,
      );
}

/// One backend log entry (logging.Entry JSON; level arrives numeric).
class LogEntry {
  final DateTime at;
  final int level; // 0 debug, 1 info, 2 warn, 3 error
  final String message;

  const LogEntry({required this.at, required this.level, required this.message});

  String get levelName => const ['DEBUG', 'INFO', 'WARN', 'ERROR'][
      level.clamp(0, 3)];

  factory LogEntry.fromJson(Map<String, dynamic> j) => LogEntry(
        at: DateTime.tryParse(j['at'] as String? ?? '') ?? DateTime.now(),
        level: (j['level'] as num?)?.toInt() ?? 1,
        message: j['message'] as String? ?? '',
      );
}
