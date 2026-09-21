import 'package:flutter/material.dart';

import '../components/server_row.dart';
import '../components/server_dialogs.dart';
import '../components/subscription_card.dart';
import '../state/app_store.dart';
import '../widgets/common.dart';

/// Server browser (§4, §32): AUTO selector card, search, per-subscription
/// collapsible cards, local servers, add/import actions.
class ServersPage extends StatefulWidget {
  final AppStore store;

  const ServersPage({super.key, required this.store});

  @override
  State<ServersPage> createState() => _ServersPageState();
}

class _ServersPageState extends State<ServersPage> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        final subs = store.subscriptions;
        final locals = store.localServers;
        final results = store.search(_query.trim());
        return Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              Row(
                children: [
                  Expanded(
                    child: SearchBar(
                      hintText: 'Search servers…',
                      leading: const Icon(Icons.search),
                      onChanged: (q) =>
                          setState(() => _query = q),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton(
                    icon: const Icon(Icons.add),
                    tooltip: 'Add server',
                    onPressed: () =>
                        showServerEditor(context, store, null),
                  ),
                  IconButton(
                    icon: const Icon(Icons.link),
                    tooltip: 'Import share link',
                    onPressed: () =>
                        showImportDialog(context, store),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              _autoCard(context, store),
              const SizedBox(height: 4),
              Expanded(
                child: _query.trim().isEmpty
                    ? ListView(
                        children: [
                          for (final s in subs)
                            SubscriptionCard(
                                store: store, sub: s),
                          if (locals.isNotEmpty) ...[
                            const SectionHeader(
                                title: 'LOCAL SERVERS'),
                            for (final s in locals)
                              ServerRow(store: store, server: s),
                          ],
                          if (subs.isEmpty && locals.isEmpty)
                            const Padding(
                              padding:
                                  EdgeInsets.only(top: 40),
                              child: Center(
                                  child: Text(
                                      'No servers yet. Add a subscription or import a link.')),
                            ),
                        ],
                      )
                    : results.isEmpty
                        ? Center(
                            child: Padding(
                              padding:
                                  const EdgeInsets.only(top: 40),
                              child: Text(
                                  'No servers match "${_query.trim()}".',
                                  textAlign: TextAlign.center),
                            ),
                          )
                        : ListView(
                            children: [
                              for (final s in results)
                                ServerRow(
                                    store: store, server: s),
                            ],
                          ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _autoCard(BuildContext context, AppStore store) {
    final auto = store.autoMode;
    return Card(
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => store.selectServer('auto'),
        child: Padding(
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 10),
          child: Row(
            children: [
              Icon(Icons.auto_awesome,
                  color: auto ? Colors.white : Colors.grey),
              const SizedBox(width: 10),
              const Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('AUTO',
                        style: TextStyle(
                            fontWeight: FontWeight.w700)),
                    Text('Let Connective pick and recover',
                        style: TextStyle(
                            fontSize: 12, color: Colors.grey)),
                  ],
                ),
              ),
              Switch(
                value: auto,
                onChanged: (v) {
                  if (v) {
                    store.selectServer('auto');
                  } else {
                    // Leave AUTO only when there is a concrete server to
                    // pin: prefer the existing manual selection, else the
                    // connected server.
                    final pin = store.selectedServerId.isNotEmpty
                        ? store.selectedServerId
                        : store.activeServerId;
                    if (pin.isNotEmpty) store.selectServer(pin);
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}
