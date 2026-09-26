import 'package:flutter/material.dart';

import '../models/connection_state.dart';
import '../models/server.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import '../widgets/common.dart';
import 'server_dialogs.dart';

/// One server row: flag + name + protocol line + latency, with an
/// inline expandable detail section. Hierarchy is carried by
/// typography, spacing and separators — not nested cards.
///
/// Priority (left → right): favorite star, flag, selection radio,
/// name + protocol, latency, health, expand chevron. Secondary info
/// (protocol, subscription, diagnostics) stays muted.
///
/// The leading radio selects the server for the next Connect press
/// (backend `servers.select`, no connection started). The expanded
/// "Connect" action keeps the old select+connect behavior.
class ServerRow extends StatelessWidget {
  final AppStore store;
  final Server server;
  final String? subscriptionName;

  const ServerRow(
      {super.key,
      required this.store,
      required this.server,
      this.subscriptionName});

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
        final highlighted = isActive || isSelected;

        // Rounded wash + separate accent bar + hairline: a
        // two-tone Border with borderRadius is rejected by the
        // framework at paint time ("borders with uniform colors"),
        // so each element is drawn independently.
        return Container(
          margin: const EdgeInsets.symmetric(vertical: 2),
          decoration: BoxDecoration(
            color: isActive
                ? ConnectiveTheme.success.withValues(alpha: 0.07)
                : isSelected
                    ? ConnectiveTheme.success.withValues(alpha: 0.04)
                    : Colors.transparent,
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radiusSmall),
          ),
          child: InkWell(
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radiusSmall),
            onTap: () => store.toggleServer(server.id),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Stack(
                  children: [
                    if (highlighted)
                      Positioned(
                        left: 0,
                        top: 8,
                        bottom: 8,
                        width: 2,
                        child: Container(
                          decoration: BoxDecoration(
                            color: ConnectiveTheme.success,
                            borderRadius: BorderRadius.circular(1),
                          ),
                        ),
                      ),
                    Padding(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 9),
                      child: Column(
                        crossAxisAlignment:
                            CrossAxisAlignment.start,
                        children: [
                          Row(
                            crossAxisAlignment:
                                CrossAxisAlignment.center,
                            children: [
                      // Favorite: subtle star, never dominates the row.
                      SizedBox(
                        width: 30,
                        height: 32,
                        child: IconButton(
                          padding: EdgeInsets.zero,
                          constraints: const BoxConstraints(
                              minWidth: 30, minHeight: 32),
                          icon: Icon(
                            server.favorite
                                ? Icons.star
                                : Icons.star_border,
                            size: 16,
                            color: server.favorite
                                ? ConnectiveTheme.warning
                                : ConnectiveTheme.textMuted,
                          ),
                          tooltip: server.favorite
                              ? 'Remove from favorites'
                              : 'Mark as favorite',
                          onPressed: () =>
                              store.toggleFavorite(server.id),
                        ),
                      ),
                      CountryFlag(code: code, width: 24, height: 17),
                      // Manual selection control: obvious, keyboard
                      // accessible, never starts a connection by itself.
                      SizedBox(
                        width: 32,
                        height: 32,
                        child: IconButton(
                          padding: EdgeInsets.zero,
                          icon: Icon(
                            isSelected
                                ? Icons.radio_button_checked
                                : Icons.radio_button_unchecked,
                            size: 17,
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
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Row(
                              children: [
                                Flexible(
                                  child: Text(server.displayName,
                                      style: const TextStyle(
                                          fontWeight: FontWeight.w600,
                                          fontSize: 13.5,
                                          height: 1.25)),
                                ),
                                if (isActive) ...[
                                  const SizedBox(width: 8),
                                  Container(
                                    padding: const EdgeInsets.symmetric(
                                        horizontal: 7, vertical: 2),
                                    decoration: BoxDecoration(
                                      color: ConnectiveTheme.success
                                          .withValues(alpha: 0.14),
                                      borderRadius:
                                          BorderRadius.circular(4),
                                      border: Border.all(
                                          color: ConnectiveTheme.success
                                              .withValues(alpha: 0.45)),
                                    ),
                                    child: const Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Icon(Icons.check,
                                            size: 11,
                                            color: ConnectiveTheme
                                                .success),
                                        SizedBox(width: 3),
                                        Text('CONNECTED',
                                            style: TextStyle(
                                                fontSize: 10,
                                                fontWeight:
                                                    FontWeight.w700,
                                                letterSpacing: 0.4,
                                                color: ConnectiveTheme
                                                    .success)),
                                      ],
                                    ),
                                  ),
                                ] else if (isSelected) ...[
                                  const SizedBox(width: 8),
                                  const Text('SELECTED',
                                      style: TextStyle(
                                          fontSize: 10,
                                          fontWeight: FontWeight.w700,
                                          letterSpacing: 0.5,
                                          color: ConnectiveTheme
                                              .success)),
                                ],
                              ],
                            ),
                            const SizedBox(height: 2),
                            Text(
                                _subline(server, subscriptionName),
                                style: const TextStyle(
                                    fontSize: 12,
                                    height: 1.3,
                                    color: ConnectiveTheme
                                        .textSecondary)),
                          ],
                        ),
                      ),
                      const SizedBox(width: 8),
                      // Latency: bright + tabular so it scans fast.
                      if (testing)
                        const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                                strokeWidth: 2))
                      else
                        Tooltip(
                          message: server.latencyMs < 0
                              ? 'Not tested yet'
                              : 'Latency: ${server.latencyMs} ms',
                          child: SizedBox(
                            width: 76,
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.end,
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(Icons.signal_cellular_alt,
                                    size: 13,
                                    color: _latencyColor(server)),
                                const SizedBox(width: 4),
                                Flexible(
                                  child: Text(server.latencyLabel,
                                      textAlign: TextAlign.end,
                                      overflow: TextOverflow.ellipsis,
                                      style: TextStyle(
                                          fontSize: 13,
                                          fontWeight: FontWeight.w600,
                                          color: server.latencyMs < 0
                                              ? ConnectiveTheme
                                                  .textMuted
                                              : ConnectiveTheme
                                                  .textPrimary,
                                          fontFeatures: const [
                                            FontFeature
                                                .tabularFigures()
                                          ])),
                                ),
                              ],
                            ),
                          ),
                        ),
                      const SizedBox(width: 10),
                      // Health: color AND icon AND text.
                      Tooltip(
                        message: 'Health: ${server.health}',
                        child: HealthBadge(
                            health: server.health, compact: true),
                      ),
                      const SizedBox(width: 6),
                      Icon(
                          expanded
                              ? Icons.expand_less
                              : Icons.expand_more,
                          size: 18,
                          color: ConnectiveTheme.textMuted),
                            ],
                          ),
                          AnimatedSize(
                            duration:
                                const Duration(milliseconds: 200),
                            alignment: Alignment.topCenter,
                            child: expanded
                                ? _detail(context,
                                    isSelected: isSelected,
                                    isActive: isActive)
                                : const SizedBox.shrink(),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                  Container(
                    height: 1,
                    margin: const EdgeInsets.only(
                        left: 10, right: 8),
                    color: ConnectiveTheme.borderSubtle,
                  ),
                ],
              ),
            ),
        );
      },
    );
  }

  static String _subline(Server s, String? subName) {
    final parts = <String>[s.compactInfo];
    if (subName != null && subName.isNotEmpty) parts.add(subName);
    return parts.join('  ·  ');
  }

  static Color _latencyColor(Server s) {
    if (s.latencyMs < 0) return ConnectiveTheme.textMuted;
    if (s.latencyMs < 120) return ConnectiveTheme.success;
    if (s.latencyMs < 300) return ConnectiveTheme.warning;
    return ConnectiveTheme.textSecondary;
  }

  Widget _detail(BuildContext context,
          {required bool isSelected, required bool isActive}) =>
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
                onPressed: () => store.connectToServer(server.id),
                icon: const Icon(Icons.bolt, size: 15),
                label: Text(isActive
                    ? 'Reconnect'
                    : ConnectionStates.isConnected(
                            store.connectionState)
                        ? 'Switch'
                        : 'Connect'),
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
