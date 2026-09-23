import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../models/server.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import 'server_dialogs.dart';

/// One server row: flag + name + protocol line + latency, with an
/// inline expandable detail section. Typography and spacing carry
/// the hierarchy — no colorful pills.
///
/// The leading radio selects the server for the next Connect press
/// (backend `servers.select`, no connection started). The expanded
/// "Connect" action keeps the old select+connect behavior.
class ServerRow extends StatelessWidget {
  final AppStore store;
  final Server server;

  const ServerRow({super.key, required this.store, required this.server});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final expanded = store.isExpandedServer(server.id);
        final testing = store.testingServers.contains(server.id);
        final isActive = store.activeServerId == server.id &&
            ConnectionStates.isConnected(store.connectionState);
        final isSelected = !store.autoMode &&
            store.selectedServerId == server.id;
        final code = countryCodeOf(
            country: server.country, displayName: server.displayName);
        return Card(
          margin: const EdgeInsets.symmetric(vertical: 3),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(
                ConnectiveTheme.radius),
            side: BorderSide(
              color: isSelected
                  ? ConnectiveTheme.success
                      .withValues(alpha: 0.5)
                  : ConnectiveTheme.border,
              width: 1,
            ),
          ),
          color: null,
          child: InkWell(
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radius),
            onTap: () => store.toggleServer(server.id),
            child: Padding(
              padding:
                  const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      CountryFlag(code: code, width: 22, height: 16),
                      const SizedBox(width: 2),
                      // Manual selection control: obvious, keyboard
                      // accessible, never starts a connection by itself.
                      IconButton(
                        icon: Icon(
                          isSelected
                              ? Icons.radio_button_checked
                              : Icons.radio_button_unchecked,
                          size: 18,
                          color: isSelected
                              ? ConnectiveTheme.success
                              : ConnectiveTheme.textMuted,
                        ),
                        tooltip: isSelected
                            ? 'Selected for connection'
                            : 'Select for connection',
                        onPressed: () => isSelected
                            ? store.selectServer('auto')
                            : store.selectServer(server.id),
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Flexible(
                                  child: Text(server.displayName,
                                      style: const TextStyle(
                                          fontWeight: FontWeight.w600,
                                          fontSize: 13)),
                                ),
                                if (isActive) ...[
                                  const SizedBox(width: 6),
                                  Container(
                                    width: 7,
                                    height: 7,
                                    decoration: const BoxDecoration(
                                        color: ConnectiveTheme.success,
                                        shape: BoxShape.circle),
                                  ),
                                ],
                                if (isSelected) ...[
                                  const SizedBox(width: 8),
                                  const Text('SELECTED',
                                      style: TextStyle(
                                          fontSize: 10,
                                          fontWeight:
                                              FontWeight.w600,
                                          letterSpacing: 0.5,
                                          color: ConnectiveTheme
                                              .success)),
                                ],
                              ],
                            ),
                            const SizedBox(height: 1),
                            Text(
                                '${server.compactInfo}  ·  ${server.health}',
                                style: const TextStyle(
                                    fontSize: 12,
                                    color: ConnectiveTheme
                                        .textSecondary)),
                          ],
                        ),
                      ),
                      const SizedBox(width: 8),
                      if (testing)
                        const SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                                strokeWidth: 2))
                      else
                        SizedBox(
                          width: 64,
                          child: Text(server.latencyLabel,
                              textAlign: TextAlign.end,
                              style: const TextStyle(
                                  fontSize: 12,
                                  color:
                                      ConnectiveTheme.textSecondary,
                                  fontFeatures: [
                                    FontFeature.tabularFigures()
                                  ])),
                        ),
                      Icon(
                          expanded
                              ? Icons.expand_less
                              : Icons.expand_more,
                          size: 18,
                          color: ConnectiveTheme.textMuted),
                    ],
                  ),
                  AnimatedSize(
                    duration: const Duration(milliseconds: 200),
                    alignment: Alignment.topCenter,
                    child: expanded
                        ? _detail(context,
                            isSelected: isSelected)
                        : const SizedBox.shrink(),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _detail(BuildContext context, {required bool isSelected}) =>
      Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Divider(height: 16),
          for (final row in server.detailRows())
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                      width: 110,
                      child: Text(row.key,
                          style: const TextStyle(
                              fontSize: 12,
                              color:
                                  ConnectiveTheme.textMuted))),
                  Expanded(
                      child: SelectableText(row.value,
                          style: const TextStyle(fontSize: 12))),
                ],
              ),
            ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              FilledButton.icon(
                onPressed: () async {
                  await store.selectServer(server.id);
                  await store.toggleConnection();
                },
                icon: const Icon(Icons.bolt, size: 15),
                label: const Text('Connect'),
              ),
              OutlinedButton.icon(
                onPressed: isSelected
                    ? null
                    : () => store.selectServer(server.id),
                icon: const Icon(Icons.check, size: 15),
                label: const Text('Select'),
              ),
              OutlinedButton.icon(
                onPressed: () => store.testServers([server.id]),
                icon: const Icon(Icons.speed, size: 15),
                label: const Text('Test'),
              ),
              OutlinedButton.icon(
                onPressed: () =>
                    showServerEditor(context, store, server),
                icon: const Icon(Icons.edit, size: 15),
                label: const Text('Edit'),
              ),
              PopupMenuButton<String>(
                icon: const Icon(Icons.more_horiz, size: 18),
                tooltip: 'More actions',
                onSelected: (v) =>
                    _action(context, v),
                itemBuilder: (context) => const [
                  PopupMenuItem(
                      value: 'duplicate',
                      child: Text('Duplicate')),
                  PopupMenuItem(
                      value: 'export', child: Text('Copy share link')),
                  PopupMenuItem(
                      value: 'delete', child: Text('Delete')),
                ],
              ),
            ],
          ),
        ],
      );

  Future<void> _action(BuildContext context, String v) async {
    switch (v) {
      case 'duplicate':
        await store.duplicateServer(server.id);
        break;
      case 'export':
        final link = await store.exportServer(server.id);
        if (link != null && context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Share link ready (${link.length} chars)')),
          );
        }
        break;
      case 'delete':
        final ok = await showDialog<bool>(
              context: context,
              builder: (c) => AlertDialog(
                title: const Text('Delete server?'),
                content: Text(
                    '"${server.displayName}" will be removed.'),
                actions: [
                  TextButton(
                      onPressed: () => Navigator.pop(c, false),
                      child: const Text('Cancel')),
                  FilledButton(
                      onPressed: () => Navigator.pop(c, true),
                      style: FilledButton.styleFrom(
                        backgroundColor: ConnectiveTheme.danger,
                        foregroundColor: Colors.white,
                      ),
                      child: const Text('Delete')),
                ],
              ),
            ) ??
            false;
        if (ok) await store.removeServer(server.id);
        break;
    }
  }
}

/// Tiny uppercase label used next to latency (selected marker).
/// Kept for API compatibility; renders as flat text, not a pill.
class StatusDot extends StatelessWidget {
  final Color color;
  final String label;

  const StatusDot({super.key, required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Text(label.toUpperCase(),
        style: const TextStyle(
            color: ConnectiveTheme.textSecondary,
            fontSize: 10,
            fontWeight: FontWeight.w600,
            letterSpacing: 0.5));
  }
}
