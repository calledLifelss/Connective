import 'package:flutter/material.dart';

import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../widgets/common.dart';

/// Routing + TUN + DNS + kill-switch settings (§9, §10). Every control
/// maps to a real backend setting; applied on change.
class RoutingPage extends StatelessWidget {
  final AppStore store;

  const RoutingPage({super.key, required this.store});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
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
            const Padding(
              padding: EdgeInsets.all(8),
              child: Text(
                'Routing, TUN and kill-switch changes apply on the next connect.',
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
}
