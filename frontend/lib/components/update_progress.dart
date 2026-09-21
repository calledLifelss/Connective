import 'package:flutter/material.dart';

import '../models/update.dart';
import '../theme/connective_theme.dart';

/// Real backend-driven progress (§14): bytes, percent, speed, ETA.
/// Never invents numbers; shows an indeterminate bar when the total is
/// still unknown.
class UpdateProgressBar extends StatelessWidget {
  final String state;
  final UpdateProgress progress;

  const UpdateProgressBar(
      {super.key, required this.state, required this.progress});

  @override
  Widget build(BuildContext context) {
    final known = progress.bytesTotal > 0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(UpdateStates.label(state),
                  style:
                      const TextStyle(fontWeight: FontWeight.w600)),
            ),
            if (known)
              Text('${progress.percent.toStringAsFixed(0)}%',
                  style: const TextStyle(
                      color: ConnectiveTheme.textSecondary)),
          ],
        ),
        const SizedBox(height: 8),
        known
            ? LinearProgressIndicator(
                value: (progress.percent / 100).clamp(0.0, 1.0))
            : const LinearProgressIndicator(),
        const SizedBox(height: 6),
        Text(
          _detail(),
          style: const TextStyle(
              color: ConnectiveTheme.textSecondary, fontSize: 12),
        ),
      ],
    );
  }

  String _detail() {
    if (progress.bytesTotal <= 0) {
      return _rate();
    }
    final done = _bytes(progress.bytesDone);
    final total = _bytes(progress.bytesTotal);
    final rate = _rate();
    final eta = progress.etaSeconds > 0
        ? ' · ${progress.etaSeconds}s left'
        : '';
    return '$done / $total${rate.isEmpty ? '' : ' · $rate'}$eta';
  }

  String _rate() {
    if (progress.speedBps <= 0) return '';
    return '${_bytes(progress.speedBps)}/s';
  }

  String _bytes(int n) {
    if (n < 1024) return '$n B';
    if (n < 1048576) return '${(n / 1024).toStringAsFixed(1)} KB';
    return '${(n / 1048576).toStringAsFixed(1)} MB';
  }
}
