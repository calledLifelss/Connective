import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';

/// The dominant main control (§3, §30): a big circular power button
/// with orbit rings. Renders backend state verbatim — never claims
/// connected unless the daemon says so; disabled while a transition
/// is in flight to prevent duplicate connects.
///
/// Tap target carries `Key('connect-button')` so tests tap it without
/// depending on its label text.
class ConnectButton extends StatelessWidget {
  final AppStore store;
  final VoidCallback? onPressed;

  const ConnectButton({super.key, required this.store, this.onPressed});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final state = store.connectionState;
        final busy = ConnectionStates.isBusy(state);
        final connected = ConnectionStates.isConnected(state);
        final degraded = state == ConnectionStates.degraded;
        final error = state == ConnectionStates.error;

        final Color stateColor;
        if (connected && !degraded) {
          stateColor = ConnectiveTheme.success;
        } else if (degraded) {
          stateColor = ConnectiveTheme.warning;
        } else if (error) {
          stateColor = ConnectiveTheme.danger;
        } else if (busy) {
          stateColor = ConnectiveTheme.accent;
        } else {
          stateColor = ConnectiveTheme.textSecondary;
        }

        return Semantics(
          button: true,
          enabled: !busy,
          label: 'Connection: ${ConnectionStates.label(state)}',
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 12),
            child: Center(
              child: SizedBox(
                width: 216,
                height: 216,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    // Outer faint orbit.
                    Container(
                      width: 216,
                      height: 216,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: ConnectiveTheme.border,
                          width: 1.5,
                        ),
                      ),
                    ),
                    // Dashed orbit in the state color.
                    CustomPaint(
                      size: const Size(188, 188),
                      painter: _DashedRing(
                        stateColor.withValues(alpha: 0.55),
                      ),
                    ),
                    // The tappable core.
                    Material(
                      color: Colors.transparent,
                      shape: const CircleBorder(),
                      child: InkWell(
                        key: const Key('connect-button'),
                        customBorder: const CircleBorder(),
                        onTap: busy ? null : onPressed,
                        child: Container(
                          width: 150,
                          height: 150,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            gradient: LinearGradient(
                              begin: Alignment.topCenter,
                              end: Alignment.bottomCenter,
                              colors: [
                                (connected || degraded
                                        ? stateColor
                                        : ConnectiveTheme.surface2)
                                    .withValues(
                                        alpha: connected || degraded
                                            ? 0.28
                                            : 1.0),
                                ConnectiveTheme.surface,
                              ],
                            ),
                            border: Border.all(
                              color: connected || degraded || busy || error
                                  ? stateColor.withValues(alpha: 0.8)
                                  : ConnectiveTheme.border,
                              width: 1.5,
                            ),
                            boxShadow: [
                              if (connected || degraded)
                                BoxShadow(
                                  color: stateColor.withValues(
                                      alpha: 0.35),
                                  blurRadius: 32,
                                  spreadRadius: 2,
                                ),
                            ],
                          ),
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              if (busy)
                                const SizedBox(
                                  width: 30,
                                  height: 30,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 3,
                                    valueColor:
                                        AlwaysStoppedAnimation<Color>(
                                      ConnectiveTheme.accent,
                                    ),
                                  ),
                                )
                              else
                                Icon(
                                  Icons.power_settings_new,
                                  size: 44,
                                  color: connected || degraded || error
                                      ? stateColor
                                      : ConnectiveTheme.textSecondary,
                                ),
                              const SizedBox(height: 8),
                              Text(
                                _actionLabel(
                                    state, busy, connected, degraded, error),
                                style: TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w700,
                                  letterSpacing: 1.5,
                                  color: connected || degraded
                                      ? stateColor
                                      : ConnectiveTheme.textSecondary,
                                ),
                                textAlign: TextAlign.center,
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  static String _actionLabel(String state, bool busy, bool connected,
      bool degraded, bool error) {
    if (busy) return ConnectionStates.label(state);
    if (degraded) return 'DEGRADED';
    if (connected) return 'CONNECTED';
    if (error) return 'TAP TO RETRY';
    return 'TAP TO CONNECT';
  }
}

/// Dotted orbit ring for the connect button.
class _DashedRing extends CustomPainter {
  final Color color;

  const _DashedRing(this.color);

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    final radius = size.width / 2;
    final center = Offset(radius, radius);
    const dashes = 48;
    const step = (math.pi * 2) / dashes;
    for (var i = 0; i < dashes; i++) {
      canvas.drawArc(
        Rect.fromCircle(center: center, radius: radius),
        i * step,
        step * 0.55,
        false,
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _DashedRing old) =>
      old.color != color;
}
