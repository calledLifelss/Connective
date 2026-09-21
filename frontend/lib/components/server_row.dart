import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../models/server.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import 'server_dialogs.dart';

/// One server row (§4, §16–18): collapsed = flag + name + compact info +
/// real latency + arrow; expanded = protocol-dependent details + actions.
/// Expansion is inline (§17), animated, never a modal.
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
          margin: const EdgeInsets.symmetric(vertical: 4),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(ConnectiveTheme.radius),
            side: BorderSide(
              color: isSelected
                  ? ConnectiveTheme.accent
                  : ConnectiveTheme.border,
              width: isSelected ? 1.5 : 1,
            ),
          ),
          color: isSelected
              ? ConnectiveTheme.accent.withValues(alpha: 0.08)
              : null,
          child: InkWell(
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radius),
            onTap: () => store.toggleServer(server.id),
            child: Padding(
              padding:
                  const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      CountryFlag(code: code),
                      const SizedBox(width: 4),
                      // Manual selection control: obvious, keyboard
                      // accessible, never starts a connection by itself.
                      IconButton(
                        icon: Icon(
                          isSelected
                              ? Icons.radio_button_checked
                              : Icons.radio_button_unchecked,
                          size: 20,
                          color: isSelected
                              ? ConnectiveTheme.accent
                              : ConnectiveTheme.textSecondary,
                        ),
                        tooltip: isSelected
                            ? 'Selected for connection'
                            : 'Select for connection',
                        onPressed: () => isSelected
                            ? store.selectServer('auto')
                            : store.selectServer(server.id),
                      ),
                      const SizedBox(width: 2),
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
                                          fontSize: 15)),
                                ),
                                if (isActive) ...[
                                  const SizedBox(width: 6),
                                  Container(
                                    width: 8,
                                    height: 8,
                                    decoration: const BoxDecoration(
                                        color: ConnectiveTheme.success,
                                        shape: BoxShape.circle),
                                  ),
                                ],
                                if (isSelected) ...[
                                  const SizedBox(width: 6),
                                  const Icon(Icons.check_circle,
                                      size: 16,
                                      color: ConnectiveTheme.accent),
                                ],
                              ],
                            ),
                            Text(server.compactInfo,
                                style: Theme.of(context)
                                    .textTheme
                                    .bodySmall
                                    ?.copyWith(
                                        color: ConnectiveTheme
                                            .textSecondary)),
                          ],
                        ),
                      ),
                      if (testing)
                        const SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                                strokeWidth: 2))
                      else
                        Text(server.latencyLabel,
                            style: Theme.of(context).textTheme.bodyMedium),
                      if (isSelected)
                        const Padding(
                          padding: EdgeInsets.only(left: 6),
                          child: StatusDot(
                              color: ConnectiveTheme.accent,
                              label: 'SELECTED'),
                        ),
                      Icon(expanded
                          ? Icons.expand_less
                          : Icons.expand_more),
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
          const Divider(),
          for (final row in server.detailRows())
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                      width: 110,
                      child: Text(row.key,
                          style: Theme.of(context)
                              .textTheme
                              .bodySmall
                              ?.copyWith(
                                  color:
                                      ConnectiveTheme.textSecondary))),
                  Expanded(
                      child: SelectableText(row.value,
                          style: const TextStyle(fontSize: 13))),
                ],
              ),
            ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton.icon(
                onPressed: () async {
                  await store.selectServer(server.id);
                  await store.toggleConnection();
                },
                icon: const Icon(Icons.bolt, size: 16),
                label: const Text('Connect'),
              ),
              OutlinedButton.icon(
                onPressed: isSelected
                    ? null
                    : () => store.selectServer(server.id),
                icon: const Icon(Icons.check, size: 16),
                label: const Text('Select'),
              ),
              OutlinedButton.icon(
                onPressed: () => store.testServers([server.id]),
                icon: const Icon(Icons.speed, size: 16),
                label: const Text('Test'),
              ),
              OutlinedButton.icon(
                onPressed: () =>
                    showServerEditor(context, store, server),
                icon: const Icon(Icons.edit, size: 16),
                label: const Text('Edit'),
              ),
              PopupMenuButton<String>(
                icon: const Icon(Icons.more_vert, size: 18),
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

/// Tiny uppercase badge used next to latency (selected/connected).
class StatusDot extends StatelessWidget {
  final Color color;
  final String label;

  const StatusDot({super.key, required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: color.withValues(alpha: 0.5)),
      ),
      child: Text(label,
          style: TextStyle(
              color: color, fontSize: 10, fontWeight: FontWeight.w700)),
    );
  }
}
