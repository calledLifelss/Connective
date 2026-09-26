import 'package:flutter/material.dart';

import '../models/update.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import 'update_progress.dart';

/// Update details + action dialog (§13, §36): current → new version,
/// release notes, size, then state-driven actions. Everything reflects
/// backend state; the dialog never simulates progress.
Future<void> showUpdateDialog(
    BuildContext context, AppStore store) {
  return showDialog(
    context: context,
    barrierDismissible: false,
    builder: (c) => ListenableBuilder(
      listenable: store,
      builder: (context, _) => AlertDialog(
        key: const Key('update-dialog'),
        title: Text(store.updateInfo == null
            ? 'Software update'
            : 'Connective ${store.updateInfo!.version}'),
        content: SizedBox(
          width: 420,
          child: _Body(store: store),
        ),
        actions: _actions(context, store),
      ),
    ),
  );
}

class _Body extends StatelessWidget {
  final AppStore store;

  const _Body({required this.store});

  @override
  Widget build(BuildContext context) {
    final info = store.updateInfo;
    final state = store.updateState;
    return SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (info != null) ...[
            Text(
              'Current: ${store.updateCurrent.isEmpty ? '?' : store.updateCurrent} → New: ${info.version}',
              style: const TextStyle(
                  color: ConnectiveTheme.textSecondary),
            ),
            const SizedBox(height: 8),
            if (info.releaseNotes.isNotEmpty) ...[
              const Text("What's new:",
                  style:
                      TextStyle(fontWeight: FontWeight.w600)),
              const SizedBox(height: 4),
              for (final n in info.releaseNotes)
                Padding(
                  padding:
                      const EdgeInsets.only(bottom: 2),
                  child: Text('• $n'),
                ),
              const SizedBox(height: 8),
            ],
            Text(
              'Download: ${info.sizeLabel} · ${info.artifactType}',
              style: const TextStyle(
                  color: ConnectiveTheme.textSecondary),
            ),
            const SizedBox(height: 12),
          ],
          if (store.updateBusy)
            UpdateProgressBar(
                state: state,
                progress: store.updateProgress),
          if (state == UpdateStates.updated)
            Row(
              children: [
                const Icon(Icons.check_circle,
                    color: ConnectiveTheme.success),
                const SizedBox(width: 8),
                Expanded(
                    child: Text(
                        'Connective is now running ${info?.version ?? 'the new version'}.')),
              ],
            ),
          if (state == UpdateStates.restarting) ...[
            const Row(
              children: [
                Icon(Icons.restart_alt,
                    size: 18,
                    color: ConnectiveTheme.textSecondary),
                SizedBox(width: 8),
                Expanded(
                    child: Text(
                        'The new version is ready. Close Connective to finish installing — your VPN disconnects — then start it again from the menu.')),
              ],
            ),
          ],
          if (state == UpdateStates.failed &&
              store.updateError != null) ...[
            const SizedBox(height: 4),
            Text(store.updateError!,
                style: const TextStyle(
                    color: ConnectiveTheme.danger)),
          ],
          if (state == UpdateStates.rolledBack)
            const Text(
                'The new version failed its health check and the previous version was restored.',
                style:
                    TextStyle(color: ConnectiveTheme.warning)),
        ],
      ),
    );
  }
}

List<Widget> _actions(BuildContext context, AppStore store) {
  final state = store.updateState;
  void pop() => Navigator.of(context).maybePop();
  Widget later(String label) => TextButton(
        key: const Key('update-dialog-later'),
        onPressed: pop,
        child: Text(label),
      );
  if (state == UpdateStates.restarting) {
    // Restarting is "busy" but Cancel cannot recall a spawned
    // installer — the only honest actions are restart or wait.
    return [
      later('Not now'),
      FilledButton(
        key: const Key('update-dialog-restart'),
        onPressed: () {
          pop();
          store.restartForUpdate();
        },
        child: const Text('Restart now'),
      ),
    ];
  }
  if (store.updateBusy) {
    return [
      TextButton(
        key: const Key('update-dialog-cancel'),
        onPressed: () => store.cancelUpdate(),
        child: const Text('Cancel'),
      ),
    ];
  }
  switch (state) {
    case UpdateStates.available:
      if (store.updateStaged) {
        return [
          later('Later'),
          FilledButton(
            key: const Key('update-dialog-install'),
            onPressed: () => store.installUpdate(),
            child: const Text('Install now'),
          ),
        ];
      }
      return [
        later('Later'),
        FilledButton(
          key: const Key('update-dialog-download'),
          onPressed: () => store.downloadUpdate(),
          child: const Text('Update now'),
        ),
      ];
    case UpdateStates.updated:
      return [
        TextButton(
          key: const Key('update-dialog-close'),
          onPressed: pop,
          child: const Text('Close'),
        ),
      ];
    case UpdateStates.failed:
    case UpdateStates.rolledBack:
    case UpdateStates.cancelled:
      // Retry re-checks (the only legal transition out of these
      // states) and resumes the download automatically.
      return [
        TextButton(
          key: const Key('update-dialog-close'),
          onPressed: pop,
          child: const Text('Close'),
        ),
        FilledButton(
          key: const Key('update-dialog-retry'),
          onPressed: () => store.retryUpdate(),
          child: const Text('Try again'),
        ),
      ];
    default:
      return [later('Close')];
  }
}
