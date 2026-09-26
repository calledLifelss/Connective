import 'package:flutter/material.dart';

import '../models/subscription.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import 'server_row.dart';

/// Header row shared by the management cards and the Dashboard sliver
/// sections: name + traffic/expiry always visible, per-subscription
/// update + menu, error indicator, expand chevron.
class SubscriptionHeader extends StatelessWidget {
  final AppStore store;
  final Subscription sub;

  const SubscriptionHeader(
      {super.key, required this.store, required this.sub});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final expanded = store.isExpandedSub(sub.id);
        final updating = store.updatingSubs[sub.id] ?? false;
        final count = store.serversOf(sub.id).length;
        return InkWell(
          borderRadius: BorderRadius.circular(
              ConnectiveTheme.radiusSmall),
          onTap: () => store.toggleSubscription(sub.id),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 8, 12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                // Account affordance: marks this as subscription /
                // account info, not another server row.
                Container(
                  width: 34,
                  height: 34,
                  decoration: BoxDecoration(
                    color: ConnectiveTheme.surfaceElevated,
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                        color: ConnectiveTheme.border),
                  ),
                  child: const Icon(Icons.person_outline,
                      size: 18,
                      color: ConnectiveTheme.textSecondary),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment:
                        CrossAxisAlignment.start,
                    children: [
                      Row(
                        textBaseline: TextBaseline.alphabetic,
                        crossAxisAlignment:
                            CrossAxisAlignment.baseline,
                        children: [
                          Flexible(
                            child: Text(sub.name,
                                style: const TextStyle(
                                    fontWeight: FontWeight.w600,
                                    fontSize: 14,
                                    height: 1.25,
                                    color: ConnectiveTheme
                                        .textPrimary)),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            '$count servers',
                            style: const TextStyle(
                                fontSize: 12,
                                color: ConnectiveTheme.textMuted),
                          ),
                          if (sub.lastError.isNotEmpty) ...[
                            const SizedBox(width: 6),
                            const Tooltip(
                              message:
                                  'Update error — see Logs',
                              child: Icon(Icons.error_outline,
                                  color:
                                      ConnectiveTheme.warning,
                                  size: 15),
                            ),
                          ],
                        ],
                      ),
                      const SizedBox(height: 8),
                      TrafficBar(traffic: sub.traffic),
                    ],
                  ),
                ),
                const SizedBox(width: 4),
                if (updating)
                  const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                          strokeWidth: 2))
                else ...[
                  IconButton(
                    icon: const Icon(Icons.refresh, size: 18),
                    tooltip: 'Refresh subscription',
                    onPressed: () =>
                        store.updateSubscriptions(sub.id),
                  ),
                  PopupMenuButton<String>(
                    icon: const Icon(Icons.more_horiz, size: 18),
                    tooltip: 'Manage subscription',
                    onSelected: (v) =>
                        subscriptionMenu(context, store, sub, v),
                    itemBuilder: (context) => const [
                      PopupMenuItem(
                          value: 'test',
                          child: Text('Test all servers')),
                      PopupMenuItem(
                          value: 'edit',
                          child: Text('Edit subscription')),
                      PopupMenuItem(
                          value: 'delete',
                          child: Text('Delete subscription')),
                    ],
                  ),
                ],
                Icon(
                    expanded
                        ? Icons.expand_less
                        : Icons.chevron_right,
                    size: 18,
                    color: ConnectiveTheme.textMuted),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Traffic quota: plain usage label above a thin 5px meter.
/// Fill is green while healthy, amber past 80%, red past 95%.
/// Falls back to the plain text summary when the subscription
/// reports no quota, and renders nothing when there is no
/// traffic data at all.
class TrafficBar extends StatelessWidget {
  final TrafficInfo traffic;

  const TrafficBar({super.key, required this.traffic});

  @override
  Widget build(BuildContext context) {
    if (traffic.hasLimit && traffic.total > 0) {
      final fraction = traffic.usedFraction.clamp(0.0, 1.0);
      final fill = fraction >= 0.95
          ? ConnectiveTheme.danger
          : fraction >= 0.8
              ? ConnectiveTheme.warning
              : ConnectiveTheme.success;
      final expiry = traffic.expiryLong();
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  traffic.usageShort(),
                  style: const TextStyle(
                    fontSize: 12,
                    color: ConnectiveTheme.textSecondary,
                    fontFeatures: [
                      FontFeature.tabularFigures()
                    ],
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (expiry.isNotEmpty)
                Text(
                  expiry,
                  style: const TextStyle(
                      fontSize: 12,
                      color: ConnectiveTheme.textMuted),
                ),
            ],
          ),
          const SizedBox(height: 6),
          ClipRRect(
            borderRadius: BorderRadius.circular(3),
            child: LinearProgressIndicator(
              value: fraction,
              backgroundColor: const Color(0xFF2A313B),
              valueColor:
                  AlwaysStoppedAnimation<Color>(fill),
              minHeight: 6,
            ),
          ),
        ],
      );
    }
    final summary = traffic.summary();
    if (summary.isEmpty) return const SizedBox.shrink();
    return Text(
      summary,
      style: const TextStyle(
          fontSize: 12, color: ConnectiveTheme.textSecondary),
    );
  }
}

/// Independently collapsible subscription card: header with
/// name + traffic/expiry always visible; servers hide on collapse;
/// per-subscription update + menu. Used by the Servers and
/// Subscriptions management pages; the Dashboard uses [SubscriptionHeader]
/// with lazy sliver lists instead.
class SubscriptionCard extends StatelessWidget {
  final AppStore store;
  final Subscription sub;

  const SubscriptionCard(
      {super.key, required this.store, required this.sub});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final expanded = store.isExpandedSub(sub.id);
        final children = store.serversOf(sub.id);
        return Card(
          child: Padding(
            padding:
                const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
            child: Column(
              children: [
                SubscriptionHeader(store: store, sub: sub),
                AnimatedSize(
                  duration: const Duration(milliseconds: 200),
                  alignment: Alignment.topCenter,
                  child: expanded
                      ? Column(
                          children: [
                            const Divider(height: 1),
                            const SizedBox(height: 6),
                            for (final s in children)
                              Padding(
                                padding: const EdgeInsets.only(
                                    left: 8,
                                    right: 8,
                                    bottom: 2),
                                child: ServerRow(
                                    key: ValueKey(s.id),
                                    store: store,
                                    server: s),
                              ),
                            const SizedBox(height: 4),
                          ],
                        )
                      : const SizedBox.shrink(),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

Future<void> subscriptionMenu(
    BuildContext context, AppStore store, Subscription sub, String v) async {
  switch (v) {
    case 'test':
      await store.testServers(
          store.serversOf(sub.id).map((s) => s.id).toList());
      break;
    case 'edit':
      if (context.mounted) {
        await showSubscriptionDialog(context, store, sub);
      }
      break;
    case 'delete':
      final ok = await showDialog<bool>(
            context: context,
            builder: (c) => AlertDialog(
              title: const Text('Delete subscription?'),
              content: Text(
                  '"${sub.name}" and its servers will be removed.'),
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
      if (ok) await store.removeSubscription(sub.id);
      break;
  }
}

/// Add/edit subscription dialog.
Future<void> showSubscriptionDialog(
    BuildContext context, AppStore store, Subscription? existing) async {
  final isNew = existing == null;
  final name =
      TextEditingController(text: existing?.name ?? '');
  final url = TextEditingController(text: existing?.url ?? '');
  var enabled = existing?.enabled ?? true;

  await showDialog(
    context: context,
    builder: (c) => StatefulBuilder(
      builder: (c, setState) => AlertDialog(
        title:
            Text(isNew ? 'Add subscription' : 'Edit subscription'),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                  controller: name,
                  decoration:
                      const InputDecoration(labelText: 'Name')),
              const SizedBox(height: 8),
              TextField(
                  controller: url,
                  decoration: const InputDecoration(
                      labelText: 'Subscription URL (https://…)')),
              const SizedBox(height: 8),
              SwitchListTile(
                title: const Text('Enabled'),
                subtitle: const Text('Disabled subscriptions are skipped on update'),
                value: enabled,
                onChanged: (v) => setState(() => enabled = v),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(c),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              Navigator.pop(c);
              if (isNew) {
                store.addSubscription(
                    name.text.trim(), url.text.trim());
              } else {
                store.editSubscription(existing.id,
                    name: name.text.trim(),
                    url: url.text.trim(),
                    enabled: enabled);
              }
            },
            child: const Text('Save'),
          ),
        ],
      ),
    ),
  );
}
