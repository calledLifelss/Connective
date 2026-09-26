import 'package:flutter/foundation.dart' show listEquals;
import 'package:flutter/material.dart';

import '../theme/connective_theme.dart';

/// Compact status tag: small dot + uppercase label on a flat surface.
/// Muted by default — color is reserved for the dot only.
class StatusBadge extends StatelessWidget {
  final String label;
  final Color color;

  const StatusBadge({super.key, required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: ConnectiveTheme.surfaceElevated,
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: ConnectiveTheme.border),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration:
                BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 6),
          Text(
            label.toUpperCase(),
            style: const TextStyle(
              color: ConnectiveTheme.textSecondary,
              fontSize: 11,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.4,
            ),
          ),
        ],
      ),
    );
  }
}

/// Section header: small uppercase label, generous spacing.
class SectionHeader extends StatelessWidget {
  final String title;
  final Widget? action;

  const SectionHeader({super.key, required this.title, this.action});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 12, bottom: 6),
      child: Row(
        children: [
          Expanded(
            child: Text(title.toUpperCase(),
                style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    letterSpacing: 0.8,
                    color: ConnectiveTheme.textMuted)),
          ),
          if (action != null) action!,
        ],
      ),
    );
  }
}

/// Dismissible message banner with a flat surface and thin border.
class ErrorBanner extends StatelessWidget {
  final String message;
  final VoidCallback onDismiss;

  const ErrorBanner(
      {super.key, required this.message, required this.onDismiss});

  @override
  Widget build(BuildContext context) {
    return Card(
      color: ConnectiveTheme.surface,
      child: Padding(
        padding: const EdgeInsets.symmetric(
            horizontal: 12, vertical: 10),
        child: Row(
          children: [
            Container(
              width: 3,
              height: 32,
              decoration: BoxDecoration(
                color: ConnectiveTheme.danger,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(width: 10),
            const Icon(Icons.error_outline,
                color: ConnectiveTheme.danger, size: 18),
            const SizedBox(width: 8),
            Expanded(
                child: Text(message,
                    style: const TextStyle(fontSize: 13))),
            IconButton(
              icon: const Icon(Icons.close, size: 16),
              onPressed: onDismiss,
              tooltip: 'Dismiss',
            ),
          ],
        ),
      ),
    );
  }
}

/// Loading row with shimmer-free progress indicator.
class LoadingRow extends StatelessWidget {
  final String label;

  const LoadingRow({super.key, required this.label});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(strokeWidth: 2)),
          const SizedBox(width: 10),
          Text(label,
              style: const TextStyle(color: ConnectiveTheme.textSecondary)),
        ],
      ),
    );
  }
}

/// Health indicator: color AND icon AND text, never color alone.
/// ● Healthy / ⚠ Degraded / × Offline-or-worse / ○ Unknown.
class HealthBadge extends StatelessWidget {
  final String health;
  final bool compact;

  const HealthBadge({super.key, required this.health, this.compact = false});

  @override
  Widget build(BuildContext context) {
    final Color c;
    final IconData icon;
    final String label;
    switch (health) {
      case 'healthy':
        c = ConnectiveTheme.success;
        icon = Icons.circle;
        label = 'Healthy';
        break;
      case 'degraded':
        c = ConnectiveTheme.warning;
        icon = Icons.warning_amber;
        label = 'Degraded';
        break;
      case 'unhealthy':
        c = ConnectiveTheme.danger;
        icon = Icons.close;
        label = 'Offline';
        break;
      default:
        c = ConnectiveTheme.textMuted;
        icon = Icons.circle_outlined;
        label = 'Unknown';
        break;
    }
    final double iconSize = health == 'healthy' ? 8 : 13;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: iconSize, color: c, semanticLabel: label),
        const SizedBox(width: 6),
        Text(label,
            style: TextStyle(
                fontSize: compact ? 12 : 12.5,
                color: ConnectiveTheme.textSecondary,
                fontWeight: FontWeight.w500)),
      ],
    );
  }
}

/// Functional dual-series traffic graph: gridlines, y-axis peak labels,
/// time axis, hover tooltip with current values, down/up legend.
/// Decorative minimalism is avoided — every element aids reading.
class TrafficGraph extends StatefulWidget {
  final List<double> down;
  final List<double> up;

  /// Wall-clock time of each sample, aligned index-for-index with
  /// [down]/[up]. Axis and hover readouts use the real sample times —
  /// never "now minus index", which lies as soon as the sampling rate
  /// is not exactly one sample per second.
  final List<DateTime> times;
  final String downLabel;
  final String upLabel;
  final int windowSec;

  const TrafficGraph({
    super.key,
    required this.down,
    required this.up,
    required this.times,
    required this.downLabel,
    required this.upLabel,
    this.windowSec = 60,
  });

  @override
  State<TrafficGraph> createState() => _TrafficGraphState();
}

class _TrafficGraphState extends State<TrafficGraph> {
  int? _hover;

  List<double> _window(List<double> v) {
    if (v.length <= widget.windowSec) return v;
    return v.sublist(v.length - widget.windowSec);
  }

  List<DateTime> _windowT(List<DateTime> v) {
    if (v.length <= widget.windowSec) return v;
    return v.sublist(v.length - widget.windowSec);
  }

  static String _clock(DateTime t, bool short) {
    final h = t.hour.toString().padLeft(2, '0');
    final m = t.minute.toString().padLeft(2, '0');
    if (short) return '$h:$m';
    return '$h:$m:${t.second.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final down = _window(widget.down);
    final up = _window(widget.up);
    final times = _windowT(widget.times);
    double peak = 1;
    for (final v in [...down, ...up]) {
      if (v > peak) peak = v;
    }
    final peakLabel = _axisRate(peak);
    final midLabel = _axisRate(peak / 2);

    final n = down.length > up.length ? down.length : up.length;
    String xLabel(int i) {
      if (n <= 1 || i < 0 || i >= times.length) return '';
      return _clock(times[i], widget.windowSec > 90);
    }

    // 5 evenly spaced ticks.
    final tickIdx = <int>[
      0,
      n ~/ 4,
      n ~/ 2,
      (n * 3) ~/ 4,
      n - 1
    ];

    final tooltip = '↓ ${widget.downLabel}   ↑ ${widget.upLabel}';
    return Tooltip(
      message: tooltip,
      preferBelow: false,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            height: 120,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: 52,
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(peakLabel, style: _axisStyle),
                      Text(midLabel, style: _axisStyle),
                      const Text('0 KB/s', style: _axisStyle),
                    ],
                  ),
                ),
                Expanded(
                  child: MouseRegion(
                    onHover: (e) {
                      final box = context.findRenderObject() as RenderBox?;
                      if (box == null || n < 2) return;
                      // `localPosition` is relative to the MouseRegion,
                      // which is the plot area itself (the 52px axis
                      // gutter is its left sibling) — so no gutter
                      // subtraction, only the plot width for scaling.
                      final w = box.size.width - 52;
                      if (w <= 0) return;
                      final dx = e.localPosition.dx.clamp(0.0, w);
                      final raw =
                          (dx / w * (n - 1)).round();
                      final idx =
                          raw < 0 ? 0 : (raw > n - 1 ? n - 1 : raw);
                      if (idx != _hover) setState(() => _hover = idx);
                    },
                    onExit: (_) => setState(() => _hover = null),
                    child: CustomPaint(
                      painter: _TrafficPainter(
                        down: down,
                        up: up,
                        peak: peak,
                        hover: _hover,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              const SizedBox(width: 52),
              Expanded(
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    for (final i in tickIdx)
                      Text(
                        xLabel(i < 0
                            ? 0
                            : (i > n - 1 ? n - 1 : i)),
                        style: _axisStyle,
                      ),
                  ],
                ),
              ),
            ],
          ),
          if (_hover != null && n > 1)
            Padding(
              padding: const EdgeInsets.only(top: 4, left: 52),
              child: Text(
                '${xLabel(_hover! < 0 ? 0 : (_hover! > n - 1 ? n - 1 : _hover!))}  ·  ↓ ${_rateAt(down, _hover!)}  ·  ↑ ${_rateAt(up, _hover!)}',
                style: const TextStyle(
                    fontSize: 11,
                    color: ConnectiveTheme.textSecondary,
                    fontFeatures: [FontFeature.tabularFigures()]),
              ),
            ),
          const SizedBox(height: 4),
          const Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              _LegendDot(color: ConnectiveTheme.success, label: 'Download'),
              SizedBox(width: 12),
              _LegendDot(color: ConnectiveTheme.info, label: 'Upload'),
            ],
          ),
        ],
      ),
    );
  }

  static String _rateAt(List<double> v, int i) {
    if (v.isEmpty) return '—';
    final j = i < 0 ? 0 : (i > v.length - 1 ? v.length - 1 : i);
    final x = v[j];
    return ConnectiveTheme.rate(x.round());
  }

  static String _axisRate(double bps) {
    if (bps < 1024) return '${bps.toStringAsFixed(0)} B/s';
    if (bps < 1024 * 1024) {
      final kb = bps / 1024;
      return '${kb >= 100 ? kb.toStringAsFixed(0) : kb.toStringAsFixed(1)} KB/s';
    }
    return '${(bps / 1048576).toStringAsFixed(1)} MB/s';
  }
}

const _axisStyle = TextStyle(
  fontSize: 10.5,
  color: ConnectiveTheme.textMuted,
  fontFeatures: [FontFeature.tabularFigures()],
);

class _LegendDot extends StatelessWidget {
  final Color color;
  final String label;
  const _LegendDot({required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 6),
        Text(label,
            style: const TextStyle(
                fontSize: 11.5, color: ConnectiveTheme.textSecondary)),
      ],
    );
  }
}

class _TrafficPainter extends CustomPainter {
  final List<double> down;
  final List<double> up;
  final double peak;
  final int? hover;

  _TrafficPainter(
      {required this.down, required this.up, required this.peak, this.hover});

  @override
  void paint(Canvas canvas, Size size) {
    // Gridlines.
    final grid = Paint()
      ..color = const Color(0xFF232932)
      ..strokeWidth = 1;
    for (var i = 0; i < 3; i++) {
      final y = 4 + (size.height - 8) * i / 2;
      canvas.drawLine(Offset(0, y), Offset(size.width, y), grid);
    }
    if (down.length < 2 && up.length < 2) {
      canvas.drawLine(
        Offset(0, size.height - 1),
        Offset(size.width, size.height - 1),
        Paint()
          ..color = const Color(0xFF2D333D)
          ..strokeWidth = 1,
      );
      return;
    }
    _stroke(canvas, size, up, ConnectiveTheme.info, 0.85, true);
    _stroke(canvas, size, down, ConnectiveTheme.success, 0.95, false);
    if (hover != null) {
      final n = down.length > up.length ? down.length : up.length;
      if (n > 1) {
        final h = hover! < 0 ? 0 : (hover! > n - 1 ? n - 1 : hover!);
        final x = h / (n - 1) * size.width;
        canvas.drawLine(
          Offset(x, 0),
          Offset(x, size.height),
          Paint()
            ..color = const Color(0xFF3A424D)
            ..strokeWidth = 1,
        );
        for (final series in [down, up]) {
          if (series.isEmpty) continue;
          final si = h > series.length - 1 ? series.length - 1 : h;
          final v = series[si];
          final y = size.height - 4 - (v / peak) * (size.height - 8);
          canvas.drawCircle(Offset(x, y), 3.5,
              Paint()..color = const Color(0xFFE8EBEF));
          canvas.drawCircle(Offset(x, y), 2,
              Paint()..color = ConnectiveTheme.success);
        }
      }
    }
  }

  void _stroke(Canvas canvas, Size size, List<double> values, Color color,
      double alpha, bool dashed) {
    if (values.length < 2) return;
    final dx = size.width / (values.length - 1);
    final path = Path();
    for (var i = 0; i < values.length; i++) {
      final x = i * dx;
      final y = size.height - 4 - (values[i] / peak) * (size.height - 8);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    final fill = Path.from(path)
      ..lineTo(size.width, size.height)
      ..lineTo(0, size.height)
      ..close();
    canvas.drawPath(
        fill, Paint()..color = color.withValues(alpha: 0.10) ..style = PaintingStyle.fill);
    final stroke = Paint()
      ..color = color.withValues(alpha: alpha)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.4
      ..strokeJoin = StrokeJoin.round
      ..strokeCap = StrokeCap.round;
    if (dashed) {
      // Subtle distinction for upload: thinner, slightly transparent.
      stroke.strokeWidth = 1.2;
    }
    canvas.drawPath(path, stroke);
  }

  @override
  bool shouldRepaint(covariant _TrafficPainter old) =>
      !listEquals(old.down, down) ||
      !listEquals(old.up, up) ||
      old.peak != peak ||
      old.hover != hover;
}
