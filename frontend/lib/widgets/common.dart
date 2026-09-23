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

/// Slim monitoring sparkline: thin stroke, faint fill, no neon.
class Sparkline extends StatelessWidget {
  final List<double> values;
  final Color color;
  final double height;

  const Sparkline(
      {super.key,
      required this.values,
      required this.color,
      this.height = 32});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: height,
      width: double.infinity,
      child: CustomPaint(painter: _SparkPainter(values, color)),
    );
  }
}

class _SparkPainter extends CustomPainter {
  final List<double> values;
  final Color color;

  _SparkPainter(this.values, this.color);

  @override
  void paint(Canvas canvas, Size size) {
    if (values.length < 2) {
      // Empty state: faint baseline so the panel doesn't look broken.
      canvas.drawLine(
        Offset(0, size.height - 1),
        Offset(size.width, size.height - 1),
        Paint()
          ..color = const Color(0xFF2D333D)
          ..strokeWidth = 1,
      );
      return;
    }
    final max =
        values.reduce((a, b) => a > b ? a : b).clamp(1.0, double.infinity);
    final dx = size.width / (values.length - 1);
    final path = Path();
    for (var i = 0; i < values.length; i++) {
      final x = i * dx;
      final y = size.height - (values[i] / max) * (size.height - 4) - 2;
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
        fill,
        Paint()
          ..color = color.withValues(alpha: 0.08)
          ..style = PaintingStyle.fill);
    canvas.drawPath(
        path,
        Paint()
          ..color = color.withValues(alpha: 0.9)
          ..style = PaintingStyle.stroke
          ..strokeWidth = 1.3
          ..strokeJoin = StrokeJoin.round
          ..strokeCap = StrokeCap.round);
  }

  @override
  bool shouldRepaint(covariant _SparkPainter old) =>
      old.values != values || old.color != color;
}
