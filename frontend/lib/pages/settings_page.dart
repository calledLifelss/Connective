import 'package:flutter/material.dart';

import '../models/update.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../widgets/common.dart';

/// Organized settings (§13): Connection / Auto Server / Subscriptions /
/// Appearance / Advanced (core paths) — progressive disclosure, no giant
/// dump. Everything persists through the backend.
class SettingsPage extends StatelessWidget {
  final AppStore store;

  const SettingsPage({super.key, required this.store});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final s = store.settings;
        return ListView(
          key: const Key('settings-scroll'),
          padding: const EdgeInsets.all(ConnectiveTheme.pad),
          children: [
            if (store.lastError != null)
              ErrorBanner(
                message: store.lastError!,
                onDismiss: store.dismissError,
              ),
            const SectionHeader(title: 'CONNECTION'),
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    title: const Text('Auto-connect on startup'),
                    value: s.autoConnect,
                    onChanged: (v) => store.updateSettings(
                        s.copyWith(autoConnect: v)),
                  ),
                  SwitchListTile(
                    title: const Text('Update subscriptions on startup'),
                    value: s.updateOnStart,
                    onChanged: (v) => store.updateSettings(
                        s.copyWith(updateOnStart: v)),
                  ),
                  ListTile(
                    title: const Text('Local proxy port'),
                    subtitle: const Text('Mixed SOCKS5+HTTP inbound'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.mixedPort),
                        initialValue: '${s.mixedPort}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) => store
                            .updateSettings(s.copyWith(
                                mixedPort:
                                    int.tryParse(v) ??
                                        s.mixedPort)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SectionHeader(title: 'AUTO SERVER'),
            Card(
              child: Column(
                children: [
                  ListTile(
                    title: const Text('Health check interval'),
                    subtitle:
                        const Text('Seconds between path probes'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.healthIntervalSec),
                        initialValue: '${s.healthIntervalSec}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) => store
                            .updateSettings(s.copyWith(
                                healthIntervalSec:
                                    int.tryParse(v) ??
                                        s.healthIntervalSec)),
                      ),
                    ),
                  ),
                  ListTile(
                    title: const Text('Test timeout'),
                    subtitle:
                        const Text('Milliseconds per server test'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.testTimeoutMs),
                        initialValue: '${s.testTimeoutMs}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) => store
                            .updateSettings(s.copyWith(
                                testTimeoutMs:
                                    int.tryParse(v) ??
                                        s.testTimeoutMs)),
                      ),
                    ),
                  ),
                  ListTile(
                    title: const Text('Test concurrency'),
                    subtitle:
                        const Text('Parallel server tests'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.testConcurrency),
                        initialValue: '${s.testConcurrency}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) => store
                            .updateSettings(s.copyWith(
                                testConcurrency:
                                    int.tryParse(v) ??
                                        s.testConcurrency)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SectionHeader(title: 'APPEARANCE'),
            Card(
              child: RadioGroup<String>(
                groupValue: s.theme,
                onChanged: (v) => store.updateSettings(
                    s.copyWith(theme: v!)),
                child: const Column(
                  children: [
                    RadioListTile<String>(
                      title: Text('Dark theme'),
                      value: 'dark',
                    ),
                  ],
                ),
              ),
            ),
            const SectionHeader(title: 'UPDATES'),
            Card(
              child: Column(
                children: [
                  ListTile(
                    title: const Text('Current version'),
                    trailing: Text(
                      store.updateCurrent.isEmpty
                          ? '—'
                          : store.updateCurrent,
                      style: const TextStyle(
                          fontWeight: FontWeight.w700),
                    ),
                  ),
                  ListTile(
                    title: const Text('Update channel'),
                    trailing: DropdownButton<String>(
                      value: const [
                        'stable',
                        'beta',
                        'dev'
                      ].contains(s.updateChannel)
                          ? s.updateChannel
                          : 'stable',
                      items: const [
                        DropdownMenuItem(
                            value: 'stable',
                            child: Text('Stable')),
                        DropdownMenuItem(
                            value: 'beta',
                            child: Text('Beta')),
                        DropdownMenuItem(
                            value: 'dev',
                            child: Text('Dev')),
                      ],
                      onChanged: (v) {
                        if (v != null) {
                          store.updateSettings(
                              s.copyWith(updateChannel: v));
                        }
                      },
                    ),
                  ),
                  SwitchListTile(
                    title: const Text('Check automatically'),
                    subtitle: const Text(
                        'Background check at most every few hours'),
                    value: s.updateAutoCheck,
                    onChanged: (v) => store.updateSettings(
                        s.copyWith(updateAutoCheck: v)),
                  ),
                  ListTile(
                    title: const Text('Last checked'),
                    trailing:
                        Text(store.updateLastCheckLabel()),
                  ),
                  Padding(
                    padding: const EdgeInsets.fromLTRB(
                        16, 0, 16, 12),
                    child: SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        key: const Key(
                            'settings-check-updates'),
                        onPressed: store.updateBusy
                            ? null
                            : () =>
                                store.checkForUpdates(),
                        icon: store.updateState ==
                                UpdateStates.checking
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child:
                                    CircularProgressIndicator(
                                        strokeWidth: 2))
                            : const Icon(Icons.sync,
                                size: 18),
                        label: Text(
                            'Check for updates — ${UpdateStates.label(store.updateState)}'),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SectionHeader(title: 'NOTIFICATIONS'),
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    title: const Text('Desktop notifications'),
                    subtitle: const Text(
                        'Connect, failover, updates (never spam)'),
                    value: store.notificationsEnabled,
                    onChanged: (v) =>
                        store.setNotifications(v),
                  ),
                  SwitchListTile(
                    title: const Text('Launch on login'),
                    subtitle: const Text(
                        'Start Connective with the desktop session'),
                    value: store.launchOnLogin,
                    onChanged: (v) =>
                        store.setLaunchOnLogin(v),
                  ),
                ],
              ),
            ),
            const SectionHeader(title: 'ADVANCED'),
            Card(
              child: Column(
                children: [
                  _pathTile(context, 'sing-box binary',
                      s.corePath, 'Core', (v) {
                    store.updateSettings(
                        s.copyWith(corePath: v));
                  }),
                  _pathTile(context, 'Privileged helper',
                      s.helperPath, 'Helper', (v) {
                    store.updateSettings(
                        s.copyWith(helperPath: v));
                  }),
                  ListTile(
                    title: const Text('Clash API port'),
                    subtitle: const Text(
                        'Local stats/control (127.0.0.1 only)'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.clashApiPort),
                        initialValue: '${s.clashApiPort}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) => store
                            .updateSettings(s.copyWith(
                                clashApiPort:
                                    int.tryParse(v) ??
                                        s.clashApiPort)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const Padding(
              padding: EdgeInsets.all(8),
              child: Text(
                'Port and path changes apply on the next connect.',
                style: TextStyle(
                    color: ConnectiveTheme.textSecondary,
                    fontSize: 12),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _pathTile(BuildContext context, String title,
      String value, String label, void Function(String) save) {
    final c = TextEditingController(text: value);
    return ListTile(
      title: Text(title),
      subtitle: Text(
          value.isEmpty ? 'Auto-detect' : value,
          overflow: TextOverflow.ellipsis),
      trailing: const Icon(Icons.edit, size: 18),
      onTap: () async {
        await showDialog(
          context: context,
          builder: (d) => AlertDialog(
            title: Text(label),
            content: TextField(
                controller: c,
                decoration: const InputDecoration(
                    labelText: 'Absolute path (empty = auto)')),
            actions: [
              TextButton(
                  onPressed: () => Navigator.pop(d),
                  child: const Text('Cancel')),
              FilledButton(
                  onPressed: () {
                    Navigator.pop(d);
                    save(c.text.trim());
                  },
                  child: const Text('Save')),
            ],
          ),
        );
      },
    );
  }
}
