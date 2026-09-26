/// Dart mirrors of backend/internal/subscriptions JSON.
class TrafficInfo {
  final int upload;
  final int download;
  final int total;
  final bool hasLimit;
  final DateTime? expire;
  final bool hasExp;

  const TrafficInfo({
    this.upload = 0,
    this.download = 0,
    this.total = 0,
    this.hasLimit = false,
    this.expire,
    this.hasExp = false,
  });

  /// "118 GB / 120 GB · Expires 23.09.2026" style summary for cards.
  String summary() {
    final used = _gb(upload + download);
    final parts = <String>[];
    if (hasLimit) {
      parts.add('$used / ${_gb(total)}');
    } else if (upload + download > 0) {
      parts.add(used);
    }
    if (hasExp && expire != null) {
      final e = expire!;
      final d =
          '${e.day.toString().padLeft(2, '0')}.${e.month.toString().padLeft(2, '0')}.${e.year}';
      parts.add('Expires $d');
    }
    return parts.join(' · ');
  }

  /// Total consumed bytes (upload + download).
  int get usedBytes => upload + download;

  /// Fraction of the quota consumed, clamped to 0..1. Zero when the
  /// subscription reports no limit (no bar to draw).
  double get usedFraction {
    if (!hasLimit || total <= 0) return 0;
    return (usedBytes / total).clamp(0.0, 1.0);
  }

  /// "118 GB / 120 GB" label drawn centered on the traffic bar.
  String usageLabel() => '${_gb(usedBytes)} / ${_gb(total)}';

  /// "2 GB / 21 GB used" label for the compact account summary.
  String usageShort() {
    if (hasLimit && total > 0) return '${_gb(usedBytes)} / ${_gb(total)} used';
    if (upload + download > 0) return '${_gb(usedBytes)} used';
    return '';
  }

  /// "Expires Oct 21, 2026" or empty when unknown.
  String expiryLong() {
    if (!hasExp || expire == null) return '';
    final e = expire!;
    return 'Expires ${_mon(e.month)} ${e.day}, ${e.year}';
  }

  static String _mon(int m) {
    const names = [
      '',
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec'
    ];
    if (m < 1 || m > 12) return '';
    return names[m];
  }

  /// "Expires 23.09.2026" or empty when unknown.
  String expiryLabel() {
    if (!hasExp || expire == null) return '';
    final e = expire!;
    final d =
        '${e.day.toString().padLeft(2, '0')}.${e.month.toString().padLeft(2, '0')}.${e.year}';
    return 'Expires $d';
  }

  static String _gb(int bytes) {
    if (bytes <= 0) return '0 GB';
    return '${(bytes / 1e9).toStringAsFixed(0)} GB';
  }

  factory TrafficInfo.fromJson(Map<String, dynamic> j) => TrafficInfo(
        upload: (j['upload'] as num?)?.toInt() ?? 0,
        download: (j['download'] as num?)?.toInt() ?? 0,
        total: (j['total'] as num?)?.toInt() ?? 0,
        hasLimit: j['hasLimit'] as bool? ?? false,
        expire: j['expire'] != null
            ? DateTime.tryParse(j['expire'] as String)
            : null,
        hasExp: j['hasExp'] as bool? ?? false,
      );
}

class Subscription {
  final String id;
  final String name;
  final String url;
  final bool enabled;
  final TrafficInfo traffic;
  final int serverCount;
  final String lastError;

  const Subscription({
    required this.id,
    required this.name,
    required this.url,
    this.enabled = true,
    this.traffic = const TrafficInfo(),
    this.serverCount = 0,
    this.lastError = '',
  });

  factory Subscription.fromJson(Map<String, dynamic> j) => Subscription(
        id: j['id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        url: j['url'] as String? ?? '',
        enabled: j['enabled'] as bool? ?? true,
        traffic: j['traffic'] is Map
            ? TrafficInfo.fromJson(
                Map<String, dynamic>.from(j['traffic'] as Map))
            : const TrafficInfo(),
        serverCount: (j['serverCount'] as num?)?.toInt() ?? 0,
        lastError: j['lastError'] as String? ?? '',
      );
}
