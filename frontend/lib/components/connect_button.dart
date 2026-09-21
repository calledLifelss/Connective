import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';

/// The dominant main control (§3, §30). Renders backend state verbatim:
/// never claims connected unless the daemon says so; disabled while a
/// transition is in flight to prevent duplicate connects.
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
        return SizedBox(
          width: double.infinity,
          height: 56,
          child: FilledButton.icon(
            onPressed: busy ? null : onPressed,
            icon: busy
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(connected ? Icons.power_off : Icons.power),
            label: Text(
              ConnectionStates.label(state),
              style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w600),
            ),
            style: FilledButton.styleFrom(
              backgroundColor: connected
                  ? ConnectiveTheme.success.withValues(alpha: 0.2)
                  : ConnectiveTheme.accent,
              foregroundColor: Colors.white,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16),
              ),
            ),
          ),
        );
      },
    );
  }
}
