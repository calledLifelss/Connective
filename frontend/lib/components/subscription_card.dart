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
        return InkWell(
          borderRadius: BorderRadius.circular(12),
          onTap: () => store.toggleSubscription(sub.id),
          child: Padding(
            padding: const EdgeInsets.symmetric(
                horizontal: 8, vertical: 10),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment:
                        CrossAxisAlignment.start,
                    children: [
                      Text(sub.name,
                          style: const TextStyle(
                              fontWeight: FontWeight.w700,
                              fontSize: 16)),
                      const SizedBox(height: 4),
                      TrafficBar(traffic: sub.traffic),
                    ],
                  ),
                ),
                if (updating)
                  const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                          strokeWidth: 2))
                else ...[
                  IconButton(
                    icon: const Icon(Icons.refresh, size: 20),
                    tooltip: 'Update now',
                    onPressed: () =>
                        store.updateSubscriptions(sub.id),
                  ),
                  PopupMenuButton<String>(
                    icon: const Icon(Icons.more_vert, size: 20),
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
                if (sub.lastError.isNotEmpty)
                  const Tooltip(
                    message: 'Update error — see Logs',
                    child: Icon(Icons.error_outline,
                        color: Colors.orange, size: 20),
                  ),
                Icon(expanded
                    ? Icons.expand_less
                    : Icons.chevron_right),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Traffic quota bar: used/total progress with the label drawn
/// centered on the bar and the expiry date at the end — the familiar
/// subscription-remaining glance. The fill shifts accent → amber →
/// red as the quota runs out. Falls back to the plain text summary
/// when the subscription reports no quota, and renders nothing when
/// there is no traffic data at all.
class TrafficBar extends StatelessWidget {
  final TrafficInfo traffic;

  const TrafficBar({super.key, required this.traffic});

  @override
  Widget build(BuildContext context) {
    if (traffic.hasLimit && traffic.total > 0) {
      final fraction = traffic.usedFraction;
      final fill = fraction >= 0.95
          ? ConnectiveTheme.danger
          : fraction >= 0.8
              ? ConnectiveTheme.warning
              : ConnectiveTheme.accent;
      final expiry = traffic.expiryLabel();
      return Row(
        children: [
          Expanded(
            child: SizedBox(
              height: 20,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(6),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    LinearProgressIndicator(
                      value: fraction,
                      backgroundColor: ConnectiveTheme.surface2,
                      valueColor:
                          AlwaysStoppedAnimation<Color>(fill),
                    ),
                    Center(
                      child: Text(
                        traffic.usageLabel(),
                        style: const TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: Colors.white,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          if (expiry.isNotEmpty) ...[
            const SizedBox(width: 8),
            Text(
              expiry,
              style: Theme.of(context)
                  .textTheme
                  .bodySmall
                  ?.copyWith(
                      color: ConnectiveTheme.textSecondary),
            ),
          ],
        ],
      );
    }
    final summary = traffic.summary();
    if (summary.isEmpty) return const SizedBox.shrink();
    return Text(
      summary,
      style: Theme.of(context)
          .textTheme
          .bodySmall
          ?.copyWith(color: Colors.grey),
    );
  }
}

/// Independently collapsible subscription card (§4, §15): header with
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
                const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            child: Column(
              children: [
                SubscriptionHeader(store: store, sub: sub),
                AnimatedSize(
                  duration: const Duration(milliseconds: 200),
                  alignment: Alignment.topCenter,
                  child: expanded
                      ? Column(
                          children: [
                            for (final s in children)
                              Padding(
                                padding: const EdgeInsets.only(
                                    left: 8, right: 8),
                                child: ServerRow(
                                    key: ValueKey(s.id),
                                    store: store,
                                    server: s),
                              ),
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
                    child: const Text('Delete')),
              ],
            ),
          ) ??
          false;
      if (ok) await store.removeSubscription(sub.id);
      break;
  }
}

/// Add/edit subscription dialog (§7).
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
