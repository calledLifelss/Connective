/// Dart mirrors of backend/internal/update status JSON. The daemon is
/// the source of truth; the UI renders these verbatim and keeps no
/// side state machine.
class UpdateProgress {
  final int bytesDone;
  final int bytesTotal;
  final double percent;
  final int speedBps;
  final int etaSeconds;

  const UpdateProgress({
    this.bytesDone = 0,
    this.bytesTotal = 0,
    this.percent = 0,
    this.speedBps = 0,
    this.etaSeconds = 0,
  });

  factory UpdateProgress.fromJson(Map<String, dynamic> j) =>
      UpdateProgress(
        bytesDone: (j['bytesDone'] as num?)?.toInt() ?? 0,
        bytesTotal: (j['bytesTotal'] as num?)?.toInt() ?? 0,
        percent: (j['percent'] as num?)?.toDouble() ?? 0,
        speedBps: (j['speedBps'] as num?)?.toInt() ?? 0,
        etaSeconds: (j['etaSeconds'] as num?)?.toInt() ?? 0,
      );
}

class UpdateInfo {
  final String version;
  final String channel;
  final String releaseDate;
  final List<String> releaseNotes;
  final int sizeBytes;
  final String artifactType;
  final String minVersion;

  const UpdateInfo({
    required this.version,
    this.channel = 'stable',
    this.releaseDate = '',
    this.releaseNotes = const [],
    this.sizeBytes = 0,
    this.artifactType = 'full',
    this.minVersion = '',
  });

  factory UpdateInfo.fromJson(Map<String, dynamic> j) => UpdateInfo(
        version: j['version'] as String? ?? '',
        channel: j['channel'] as String? ?? 'stable',
        releaseDate: j['releaseDate'] as String? ?? '',
        releaseNotes: [
          for (final e in (j['releaseNotes'] as List? ?? []))
            e.toString()
        ],
        sizeBytes: (j['sizeBytes'] as num?)?.toInt() ?? 0,
        artifactType: j['artifactType'] as String? ?? 'full',
        minVersion: j['minVersion'] as String? ?? '',
      );

  String get sizeLabel {
    if (sizeBytes <= 0) return '—';
    if (sizeBytes < 1024) return '$sizeBytes B';
    if (sizeBytes < 1048576) {
      return '${(sizeBytes / 1024).toStringAsFixed(1)} KB';
    }
    return '${(sizeBytes / 1048576).toStringAsFixed(1)} MB';
  }
}

/// Backend update states (mirrors internal/update/state.go).
class UpdateStates {
  static const idle = 'idle';
  static const checking = 'checking';
  static const noUpdate = 'no-update';
  static const available = 'update-available';
  static const downloading = 'downloading';
  static const verifying = 'verifying';
  static const staging = 'staging';
  static const installing = 'installing';
  static const restarting = 'restarting';
  static const updated = 'updated';
  static const cancelled = 'cancelled';
  static const failed = 'failed';
  static const rolledBack = 'rolled-back';

  static const _busy = {
    checking,
    downloading,
    verifying,
    staging,
    installing,
    restarting,
  };

  static String label(String state) {
    switch (state) {
      case idle:
        return 'Up to date';
      case checking:
        return 'Checking…';
      case noUpdate:
        return 'Up to date';
      case available:
        return 'Update available';
      case downloading:
        return 'Downloading…';
      case verifying:
        return 'Verifying…';
      case staging:
        return 'Preparing…';
      case installing:
        return 'Installing…';
      case restarting:
        return 'Restarting…';
      case updated:
        return 'Updated';
      case cancelled:
        return 'Cancelled';
      case failed:
        return 'Update failed';
      case rolledBack:
        return 'Rolled back';
      default:
        return state;
    }
  }

  static bool isBusy(String s) => _busy.contains(s);
}
