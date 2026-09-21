import 'package:flutter/material.dart';

import '../components/subscription_card.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';

/// Subscription management (§7): add/edit/delete/update-now per
/// subscription, traffic/expiry/server counts, update errors.
class SubscriptionsPage extends StatelessWidget {
  final AppStore store;

  const SubscriptionsPage({super.key, required this.store});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        return Scaffold(
          backgroundColor: Colors.transparent,
          floatingActionButton: FloatingActionButton.extended(
            onPressed: () =>
                showSubscriptionDialog(context, store, null),
            icon: const Icon(Icons.add),
            label: const Text('Add'),
          ),
          body: Padding(
            padding: const EdgeInsets.all(ConnectiveTheme.pad),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                          '${store.subscriptions.length} subscriptions · ${store.servers.length} servers',
                          style: const TextStyle(
                              color:
                                  ConnectiveTheme.textSecondary)),
                    ),
                    TextButton.icon(
                      onPressed:
                          store.subscriptions.isEmpty
                              ? null
                              : () => store.updateSubscriptions(),
                      icon: const Icon(Icons.sync, size: 18),
                      label: const Text('Update all'),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Expanded(
                  child: store.subscriptions.isEmpty
                      ? const Center(
                          child: Text(
                              'No subscriptions yet.\nAdd one to load servers.',
                              textAlign: TextAlign.center))
                      : ListView(
                          children: [
                            for (final s
                                in store.subscriptions)
                              SubscriptionCard(
                                  store: store, sub: s),
                          ],
                        ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
