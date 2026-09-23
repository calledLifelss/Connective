import 'package:flutter/material.dart';

import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import 'update_dialog.dart';

/// Non-intrusive Dashboard banner: visible only when an
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
          child: Padding(
            padding: const EdgeInsets.symmetric(
                horizontal: 12, vertical: 10),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: ConnectiveTheme.surfaceElevated,
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                        color: ConnectiveTheme.border),
                  ),
                  child: const Icon(Icons.system_update,
                      size: 17,
                      color: ConnectiveTheme.textSecondary),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Connective ${info.version} is available',
                        style: const TextStyle(
                            fontWeight: FontWeight.w600,
                            fontSize: 13),
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
                  child: const Text('Update'),
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



/// Subtle update dot for navigation areas.
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
          width: 7,
          height: 7,
          decoration: const BoxDecoration(
              color: ConnectiveTheme.success,
              shape: BoxShape.circle),
        );
      },
    );
  }
}
