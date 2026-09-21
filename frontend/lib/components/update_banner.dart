import 'package:flutter/material.dart';

import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import 'update_dialog.dart';

/// Non-intrusive Dashboard banner (§11, §35): visible only when an
/// update is actually available. Dismissing hides it until a NEWER
/// version appears; checks continue in the background.
class UpdateBanner extends StatelessWidget {
  final AppStore store;

  const UpdateBanner({super.key, required this.store});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        // Zero footprint when quiet: the Dashboard goldens and layout
        // must be pixel-identical with no update available.
        if (!store.updateAvailable) return const SizedBox.shrink();
        final info = store.updateInfo!;
        return Padding(
          padding: const EdgeInsets.fromLTRB(
              ConnectiveTheme.pad, 12, ConnectiveTheme.pad, 0),
          child: Card(
            key: const Key('update-banner'),
            margin: EdgeInsets.zero,
            color:
                ConnectiveTheme.accentSoft.withValues(alpha: 0.25),
          child: Padding(
            padding: const EdgeInsets.symmetric(
                horizontal: 14, vertical: 10),
            child: Row(
              children: [
                const Icon(Icons.auto_awesome,
                    size: 20, color: ConnectiveTheme.accent),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Connective ${info.version} is available',
                        style: const TextStyle(
                            fontWeight: FontWeight.w700),
                      ),
                      Text(
                        info.sizeLabel,
                        style: const TextStyle(
                            color:
                                ConnectiveTheme.textSecondary,
                            fontSize: 12),
                      ),
                    ],
                  ),
                ),
                TextButton(
                  onPressed: () => store.dismissUpdate(),
                  child: const Text('Later'),
                ),
                const SizedBox(width: 4),
                FilledButton(
                  key: const Key('update-banner-now'),
                  onPressed: () =>
                      showUpdateDialog(context, store),
                  child: const Text('Update now'),
                ),
              ],
            ),
          ),
          ),
        );
      },
    );
  }
}



/// Subtle update dot for navigation areas (§35).
class UpdateDot extends StatelessWidget {
  final AppStore store;

  const UpdateDot({super.key, required this.store});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        if (!store.updateAvailable) {
          return const SizedBox.shrink();
        }
        return Container(
          width: 8,
          height: 8,
          decoration: const BoxDecoration(
              color: ConnectiveTheme.accent,
              shape: BoxShape.circle),
        );
      },
    );
  }
}
