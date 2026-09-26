import 'package:flutter/material.dart';

import '../models/running_app.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../widgets/common.dart';

/// Routing + TUN + DNS + kill-switch + per-app split tunneling
/// (§9, §10). Every control maps to a real backend setting; applied on
/// change. Split-tunnel app rules render into the sing-box config on the
/// next connect; the picker lists currently-running apps first.
class RoutingPage extends StatefulWidget {
  final AppStore store;

  const RoutingPage({super.key, required this.store});

  @override
  State<RoutingPage> createState() => _RoutingPageState();
}

class _RoutingPageState extends State<RoutingPage> {
  final _addCtrl = TextEditingController();
  String _filter = '';
  String? _addError;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (widget.store.runningApps.isEmpty &&
          !widget.store.runningAppsLoading) {
        widget.store.refreshRunningApps();
      }
    });
  }

  @override
  void dispose() {
    _addCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        final s = store.settings;
        return ListView(
          padding: const EdgeInsets.all(ConnectiveTheme.pad),
          children: [
            if (store.lastError != null)
              ErrorBanner(
                message: store.lastError!,
                onDismiss: store.dismissError,
              ),
            const SectionHeader(title: 'ROUTING MODE'),
            Card(
              child: RadioGroup<String>(
                groupValue: s.routingMode,
                onChanged: (v) => store.updateSettings(
                    s.copyWith(routingMode: v!)),
                child: const Column(
                  children: [
                    RadioListTile<String>(
                      title: Text('Global'),
                      subtitle: Text(
                          'Almost everything goes through the proxy'),
                      value: 'global',
                    ),
                    RadioListTile<String>(
                      title: Text('Rules'),
                      subtitle: Text(
                          'Bypass LAN/private, route the rest by rule'),
                      value: 'rules',
                    ),
                  ],
                ),
              ),
            ),
            const SectionHeader(title: 'TUN'),
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    title: const Text('Enable TUN'),
                    subtitle: const Text(
                        'Capture device traffic (needs elevation on connect)'),
                    value: s.tunEnabled,
                    onChanged: (v) => store.updateSettings(
                        s.copyWith(tunEnabled: v)),
                  ),
                  ListTile(
                    title: const Text('Interface MTU'),
                    trailing: SizedBox(
                      width: 90,
                      child: TextFormField(
                        key: ValueKey(s.mtu),
                        initialValue: '${s.mtu}',
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) {
                          final mtu =
                              int.tryParse(v) ?? s.mtu;
                          store.updateSettings(
                              s.copyWith(mtu: mtu));
                        },
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SectionHeader(title: 'DNS'),
            Card(
              child: RadioGroup<String>(
                groupValue: s.dnsMode,
                onChanged: (v) => store.updateSettings(
                    s.copyWith(dnsMode: v!)),
                child: const Column(
                  children: [
                    RadioListTile<String>(
                      title: Text('Proxy-aware'),
                      subtitle: Text(
                          'DNS through the proxy (anti-leak)'),
                      value: 'proxy-aware',
                    ),
                    RadioListTile<String>(
                      title: Text('System'),
                      subtitle:
                          Text('Leave the OS resolver alone'),
                      value: 'system',
                    ),
                  ],
                ),
              ),
            ),
            const SectionHeader(title: 'KILL SWITCH'),
            Card(
              child: SwitchListTile(
                title: const Text('Kill switch'),
                subtitle: const Text(
                    'Block non-tunnel egress (nftables, needs elevation)'),
                value: s.killSwitch,
                onChanged: (v) => store.updateSettings(
                    s.copyWith(killSwitch: v)),
              ),
            ),
            const SectionHeader(title: 'SPLIT TUNNELING'),
            Card(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  RadioGroup<String>(
                    groupValue: s.splitMode,
                    onChanged: (v) => store.setSplitMode(v!),
                    child: const Column(
                      children: [
                        RadioListTile<String>(
                          title: Text('Off'),
                          subtitle: Text(
                              'Everything follows the routing mode above'),
                          value: 'off',
                        ),
                        RadioListTile<String>(
                          title: Text('Bypass VPN for these apps'),
                          subtitle: Text(
                              'Listed apps connect directly, rest uses the VPN'),
                          value: 'bypass',
                        ),
                        RadioListTile<String>(
                          title: Text('Only these apps use VPN'),
                          subtitle: Text(
                              'Listed apps use the VPN, rest connects directly'),
                          value: 'only',
                        ),
                      ],
                    ),
                  ),
                  if (s.splitMode != 'off') ...[
                    const Divider(height: 1),
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
                      child: Row(
                        children: [
                          Expanded(
                            child: TextField(
                              key: const Key('split-app-add-field'),
                              controller: _addCtrl,
                              decoration: InputDecoration(
                                hintText:
                                    'Add app by exe name (e.g. firefox)',
                                isDense: true,
                                border: const OutlineInputBorder(),
                                errorText: _addError,
                              ),
                              onChanged: (_) {
                                if (_addError != null) {
                                  setState(() => _addError = null);
                                }
                              },
                              onSubmitted: (_) => _add(store),
                            ),
                          ),
                          const SizedBox(width: 8),
                          FilledButton(
                            key: const Key('split-app-add-button'),
                            onPressed: () => _add(store),
                            child: const Text('Add'),
                          ),
                        ],
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 4, 16, 0),
                      child: TextField(
                        decoration: const InputDecoration(
                          hintText: 'Filter apps…',
                          isDense: true,
                          prefixIcon: Icon(Icons.search, size: 18),
                        ),
                        onChanged: (v) =>
                            setState(() => _filter = v.trim().toLowerCase()),
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 4, 16, 0),
                      child: Row(
                        children: [
                          const Text('Running apps appear first',
                              style: TextStyle(
                                  color:
                                      ConnectiveTheme.textSecondary,
                                  fontSize: 12)),
                          const Spacer(),
                          if (store.runningAppsLoading)
                            const SizedBox(
                              width: 14,
                              height: 14,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2),
                            )
                          else
                            TextButton(
                              onPressed: store.refreshRunningApps,
                              child: const Text('Refresh'),
                            ),
                        ],
                      ),
                    ),
                    for (final row in _visibleRows(store))
                      CheckboxListTile(
                        key: ValueKey('split-app-${row.id}'),
                        dense: true,
                        controlAffinity:
                            ListTileControlAffinity.leading,
                        value: row.selected,
                        onChanged: (_) => store.setSplitAppSelected(
                            row.id, !row.selected),
                        title: Text(row.name,
                            overflow: TextOverflow.ellipsis),
                        subtitle: row.running
                            ? Text(
                                'Running${row.count > 1 ? ' ×${row.count}' : ''} · ${row.id}',
                                style: const TextStyle(fontSize: 12),
                              )
                            : Text('Not running · ${row.id}',
                                style: const TextStyle(fontSize: 12)),
                        secondary: Container(
                          width: 8,
                          height: 8,
                          decoration: BoxDecoration(
                            color: row.running
                                ? ConnectiveTheme.success
                                : ConnectiveTheme.textSecondary
                                    .withValues(alpha: 0.4),
                            shape: BoxShape.circle,
                          ),
                        ),
                      ),
                    if (_visibleRows(store).isEmpty)
                      const Padding(
                        padding: EdgeInsets.fromLTRB(16, 8, 16, 16),
                        child: Text(
                          'No apps match. Add one by exe name above.',
                          style: TextStyle(
                              color: ConnectiveTheme.textSecondary,
                              fontSize: 12),
                        ),
                      ),
                    const SizedBox(height: 8),
                  ],
                ],
              ),
            ),
            const Padding(
              padding: EdgeInsets.all(8),
              child: Text(
                'Routing, TUN, kill-switch and split-tunnel changes apply on the next connect. Per-app rules need TUN enabled.',
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

  void _add(AppStore store) {
    final raw = _addCtrl.text.trim();
    if (raw.isEmpty) return;
    // Validate client-side with the same rules as the daemon
    // (apps.NormalizeAppID) so bad input gets an inline error instead of
    // a bounced settings.update (B15).
    final id = normalizeSplitAppId(raw);
    if (id == null) {
      setState(() => _addError =
          'Enter an executable name like "firefox" '
          '(letters, digits, . _ - +).');
      return;
    }
    setState(() => _addError = null);
    store.addSplitApp(id);
    _addCtrl.clear();
  }

  List<SplitAppRow> _visibleRows(AppStore store) {
    final rows = store.splitAppRows();
    if (_filter.isEmpty) return rows;
    return rows
        .where((r) =>
            r.name.toLowerCase().contains(_filter) ||
            r.id.toLowerCase().contains(_filter))
        .toList();
  }
}
