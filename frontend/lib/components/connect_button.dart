import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';

/// The main connection control: a restrained desktop VPN switch.
///
/// A single horizontal row — 44px power affordance, status text, and
/// one primary Connect/Disconnect button. Renders backend state
/// verbatim — never claims connected unless the daemon says so;
/// disabled while a transition is in flight to prevent duplicates.
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
          stateColor = ConnectiveTheme.textSecondary;
        } else {
          stateColor = ConnectiveTheme.textSecondary;
        }

        final label = _actionLabel(state, busy, connected, degraded, error);

        return Semantics(
          button: true,
          enabled: !busy,
          label: 'Connection: ${ConnectionStates.label(state)}',
          child: Container(
            decoration: BoxDecoration(
              color: ConnectiveTheme.background,
              borderRadius: BorderRadius.circular(
                  ConnectiveTheme.radiusSmall),
              border: Border.all(
                color: connected
                    ? stateColor.withValues(alpha: 0.55)
                    : ConnectiveTheme.border,
                width: connected ? 1.2 : 1,
              ),
            ),
            padding: const EdgeInsets.symmetric(
                horizontal: 12, vertical: 10),
            child: Row(
              children: [
                // Power affordance — same action, no glow, no rings.
                Material(
                  color: Colors.transparent,
                  borderRadius: BorderRadius.circular(22),
                  child: InkWell(
                    borderRadius: BorderRadius.circular(22),
                    onTap: busy ? null : onPressed,
                    child: Container(
                      width: 44,
                      height: 44,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: connected && !degraded
                            ? ConnectiveTheme.accentDim
                                .withValues(alpha: 0.45)
                            : ConnectiveTheme.surfaceElevated,
                        border: Border.all(
                          color: connected || degraded || error
                              ? stateColor.withValues(alpha: 0.7)
                              : ConnectiveTheme.border,
                          width: 1.2,
                        ),
                      ),
                      child: Center(
                        child: busy
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(
                                    strokeWidth: 2),
                              )
                            : Icon(
                                Icons.power_settings_new,
                                size: 22,
                                color: connected || degraded || error
                                    ? stateColor
                                    : ConnectiveTheme.textSecondary,
                              ),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        ConnectionStates.label(state),
                        style: const TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: ConnectiveTheme.textPrimary,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        _subLabel(
                            state, busy, connected, degraded, error),
                        style: const TextStyle(
                          fontSize: 12,
                          color: ConnectiveTheme.textSecondary,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 12),
                SizedBox(
                  height: 38,
                  child: connected
                      ? OutlinedButton(
                          key: const Key('connect-button'),
                          onPressed: busy ? null : onPressed,
                          child: const Text('Disconnect'),
                        )
                      : FilledButton(
                          key: const Key('connect-button'),
                          onPressed: busy ? null : onPressed,
                          child: busy
                              ? const SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      valueColor:
                                          AlwaysStoppedAnimation<
                                                  Color>(
                                              ConnectiveTheme
                                                  .onAccent)),
                                )
                              : Text(label),
                        ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  static String _actionLabel(String state, bool busy, bool connected,
      bool degraded, bool error) {
    if (busy) return ConnectionStates.label(state);
    if (degraded) return 'Reconnect';
    if (connected) return 'Disconnect';
    if (error) return 'Retry';
    return 'Connect';
  }

  static String _subLabel(String state, bool busy, bool connected,
      bool degraded, bool error) {
    if (busy) return 'Working — please wait…';
    if (degraded) return 'Connected, degraded path';
    if (connected) return 'Traffic routed through tunnel';
    if (error) return 'Last attempt failed';
    return 'Not connected';
  }
}
