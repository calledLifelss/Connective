import 'package:flutter/material.dart';

import '../components/connect_button.dart';
import '../components/server_dialogs.dart';
import '../components/server_row.dart';
import '../components/subscription_card.dart';
import '../components/update_banner.dart';
import '../models/connection_state.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import '../widgets/common.dart';

/// Home: the everyday connection experience.
///
/// One vertical scroll: status panel → Add Subscription → search →
/// subscriptions → servers. A persistent compact connection bar
/// fades in once the status panel scrolls away, so the Connect
/// action is never lost. Pressing Connect uses the manual selection
/// (or AUTO) through the real backend, then smoothly returns
/// attention to the status panel.
class DashboardPage extends StatefulWidget {
  final AppStore store;

  const DashboardPage({super.key, required this.store});

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  final ScrollController _scroll = ScrollController();
  final List<double> _downHist = [];
  final List<double> _upHist = [];
  bool _showCompact = false;
  String _query = '';

  static const double _compactThreshold = 260;

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scroll.removeListener(_onScroll);
    _scroll.dispose();
    super.dispose();
  }

  void _onScroll() {
    final show = _scroll.hasClients && _scroll.offset > _compactThreshold;
    if (show != _showCompact && mounted) {
      setState(() => _showCompact = show);
    }
  }

  /// Connect/disconnect through the real backend, then smoothly return
  /// attention to the status panel.
  Future<void> _connect() async {
    await widget.store.toggleConnection();
    if (!mounted || !_scroll.hasClients) return;
    await _scroll.animateTo(
      0,
      duration: const Duration(milliseconds: 350),
      curve: Curves.easeOutCubic,
    );
  }

  Future<void> _backToTop() async {
    if (!mounted || !_scroll.hasClients) return;
    await _scroll.animateTo(
      0,
      duration: const Duration(milliseconds: 350),
      curve: Curves.easeOutCubic,
    );
  }

  /// Content column: capped width, centered in wide windows so the
  /// page feels composed instead of stretched edge to edge.
  static const double _contentMax = 720;

  EdgeInsets _hPad(BuildContext context) {
    final total = MediaQuery.sizeOf(context).width;
    const rail = 200.0; // sidebar width in main.dart
    final avail = total - rail - _contentMax;
    if (avail <= 0) {
      return const EdgeInsets.symmetric(
          horizontal: ConnectiveTheme.pad);
    }
    final side = ConnectiveTheme.pad + avail / 2;
    return EdgeInsets.symmetric(horizontal: side);
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        final hPad = _hPad(context);
        _pushHistory(store);
        return Column(
          children: [
            AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              curve: Curves.easeOut,
              height: _showCompact ? 56 : 0,
              child: _showCompact
                  ? _CompactBar(
                      key: const Key('dash-compact-bar'),
                      store: store,
                      onConnect: _connect,
                      onTop: _backToTop,
                    )
                  : const SizedBox.shrink(),
            ),
            Expanded(
              child: CustomScrollView(
                key: const Key('dash-scroll'),
                controller: _scroll,
                slivers: [
                  const SliverToBoxAdapter(
                      child: SizedBox(height: 16)),
                  ..._banners(store, hPad),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad,
                      child: _ConnectionCard(
                          store: store,
                          onConnect: _connect,
                          downHist: _downHist,
                          upHist: _upHist),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad.copyWith(top: 12),
                      child: _TunCard(store: store),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: UpdateBanner(store: store),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad.copyWith(top: 12),
                      child: _ServerToolbar(
                        store: store,
                        onQuery: (q) =>
                            setState(() => _query = q),
                      ),
                    ),
                  ),
                  ..._body(store, hPad),
                  const SliverToBoxAdapter(
                      child: SizedBox(height: 24)),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  List<Widget> _banners(AppStore store, EdgeInsets hPad) {
    final out = <Widget>[];
    SliverToBoxAdapter wrap(Widget c) => SliverToBoxAdapter(
          child: Padding(
            padding: hPad.copyWith(bottom: 12),
            child: c,
          ),
        );
    if (store.foreignTun.isNotEmpty &&
        !ConnectionStates.isConnected(store.connectionState)) {
      out.add(wrap(Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 10),
          child: Row(
            children: [
              const Icon(Icons.warning_amber,
                  color: ConnectiveTheme.warning, size: 18),
              const SizedBox(width: 10),
              Expanded(
                  child: Text(
                      'Another VPN tunnel (${store.foreignTun.join(', ')}) is active. Connecting may conflict — the backend will fail safely rather than corrupt routes.',
                      style: const TextStyle(fontSize: 13, height: 1.4))),
            ],
          ),
        ),
      )));
    }
    if (!store.backendAlive) {
      out.add(wrap(Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 10),
          child: Row(
            children: [
              const Icon(Icons.cloud_off,
                  color: ConnectiveTheme.warning, size: 18),
              const SizedBox(width: 10),
              const Expanded(
                  child: Text(
                      'Backend unavailable — showing last known state.',
                      style: TextStyle(fontSize: 13))),
              TextButton(
                onPressed: () => store.reconnect(),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      )));
    }
    if (store.lastError != null) {
      out.add(wrap(ErrorBanner(
        message: store.lastError!,
        onDismiss: store.dismissError,
      )));
    }
    return out;
  }

  /// Subscriptions + servers (or flat search results), lazily built.
  List<Widget> _body(AppStore store, EdgeInsets hPad) {
    final pad = hPad;
    final q = _query.trim();
    if (q.isNotEmpty) {
      final results = store.search(q);
      return [
        SliverToBoxAdapter(
          child: Padding(
            padding: pad.copyWith(top: 12),
            child: Text('${results.length} result${results.length == 1 ? '' : 's'} for "$q"',
                style: const TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 12)),
          ),
        ),
        if (results.isEmpty)
          const SliverToBoxAdapter(
            child: SizedBox(
              height: 160,
              child: Center(
                  child: Text('No servers match.',
                      style: TextStyle(
                          color:
                              ConnectiveTheme.textSecondary))),
            ),
          )
        else
          SliverPadding(
            padding: pad.copyWith(top: 8),
            sliver: SliverList.builder(
              itemCount: results.length,
              addAutomaticKeepAlives: false,
              itemBuilder: (context, i) => ServerRow(
                key: ValueKey('dash-${results[i].id}'),
                store: store,
                server: results[i],
              ),
            ),
          ),
      ];
    }

    final slivers = <Widget>[];
    for (final sub in store.subscriptions) {
      final children = store.serversOf(sub.id);
      final expanded = store.isExpandedSub(sub.id);
      slivers.add(SliverToBoxAdapter(
        child: Padding(
          padding: pad.copyWith(top: 12),
          child: Card(
            margin: EdgeInsets.zero,
            child: SubscriptionHeader(store: store, sub: sub),
          ),
        ),
      ));
      slivers.add(SliverPadding(
        padding: pad,
        sliver: SliverList.builder(
          itemCount: expanded ? children.length : 0,
          addAutomaticKeepAlives: false,
          itemBuilder: (context, i) => ServerRow(
            key: ValueKey('dash-${children[i].id}'),
            store: store,
            server: children[i],
          ),
        ),
      ));
    }
    final locals = store.localServers;
    if (locals.isNotEmpty) {
      slivers.add(SliverToBoxAdapter(
        child: Padding(
          padding: pad.copyWith(top: 12),
          child:
              const SectionHeader(title: 'Local servers'),
        ),
      ));
      slivers.add(SliverPadding(
        padding: pad,
        sliver: SliverList.builder(
          itemCount: locals.length,
          addAutomaticKeepAlives: false,
          itemBuilder: (context, i) => ServerRow(
            key: ValueKey('dash-${locals[i].id}'),
            store: store,
            server: locals[i],
          ),
        ),
      ));
    }
    if (store.subscriptions.isEmpty && locals.isEmpty) {
      slivers.add(const SliverToBoxAdapter(
        child: SizedBox(
          height: 160,
          child: Center(
              child: Text(
                  'No servers yet. Add a subscription to load servers.',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                      color: ConnectiveTheme.textSecondary))),
        ),
      ));
    }
    return slivers;
  }

  void _pushHistory(AppStore store) {
    _downHist.add(store.stats.downRate.toDouble());
    _upHist.add(store.stats.upRate.toDouble());
    if (_downHist.length > 60) _downHist.removeAt(0);
    if (_upHist.length > 60) _upHist.removeAt(0);
  }
}

/// Compact persistent connection bar: pinned above the dashboard
/// scroll, visible once the status panel scrolls away. Same backend
/// action, same state — never a second competing control.
class _CompactBar extends StatelessWidget {
  final AppStore store;
  final Future<void> Function() onConnect;
  final Future<void> Function() onTop;

  const _CompactBar(
      {super.key,
      required this.store,
      required this.onConnect,
      required this.onTop});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (context, _) {
        final state = store.connectionState;
        final connected = ConnectionStates.isConnected(state);
        final busy = ConnectionStates.isBusy(state);
        final flagServer =
            connected ? store.activeServer : store.selectedServer;
        final flagCode = flagServer == null
            ? null
            : countryCodeOf(
                country: flagServer.country,
                displayName: flagServer.displayName);
        return Container(
          decoration: const BoxDecoration(
            color: ConnectiveTheme.surface,
            border: Border(
                bottom: BorderSide(color: ConnectiveTheme.border)),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            children: [
              _StateDot(state: state, size: 8),
              const SizedBox(width: 10),
              if (flagCode != null || flagServer != null)
                CountryFlag(
                    code: flagCode, width: 20, height: 15),
              if (flagCode != null || flagServer != null)
                const SizedBox(width: 8),
              Expanded(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(ConnectionStates.label(state),
                        style: const TextStyle(
                            fontWeight: FontWeight.w600, fontSize: 13)),
                    Text(_targetLine(store),
                        style: const TextStyle(
                            color: ConnectiveTheme.textSecondary,
                            fontSize: 12),
                        overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              IconButton(
                icon: const Icon(Icons.arrow_upward, size: 16),
                tooltip: 'Back to status',
                onPressed: onTop,
              ),
              const SizedBox(width: 4),
              SizedBox(
                height: 34,
                child: connected
                    ? OutlinedButton(
                        key: const Key('dash-compact-connect'),
                        onPressed: busy ? null : onConnect,
                        child: const Text('Disconnect'),
                      )
                    : FilledButton(
                        key: const Key('dash-compact-connect'),
                        onPressed: busy ? null : onConnect,
                        child: busy
                            ? const SizedBox(
                                width: 14,
                                height: 14,
                                child: CircularProgressIndicator(
                                    strokeWidth: 2))
                            : const Text('Connect'),
                      ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Status panel: state, one server summary, connect control,
/// live stats. Flat surfaces, typography-led hierarchy — no hero art.
class _ConnectionCard extends StatelessWidget {
  final AppStore store;
  final Future<void> Function() onConnect;
  final List<double> downHist;
  final List<double> upHist;

  const _ConnectionCard(
      {required this.store,
      required this.onConnect,
      required this.downHist,
      required this.upHist});

  @override
  Widget build(BuildContext context) {
    final state = store.connectionState;
    final connected = ConnectionStates.isConnected(state);
    final selected = store.selectedServer;
    return Card(
      key: const Key('dash-connection-expanded'),
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _StateDot(state: state, size: 8),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    ConnectionStates.label(state),
                    style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: ConnectiveTheme.textPrimary),
                  ),
                ),
                Text(
                  connected
                      ? (state == ConnectionStates.degraded
                          ? 'DEGRADED'
                          : 'PROTECTED')
                      : 'NOT PROTECTED',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.6,
                    color: connected
                        ? (state == ConnectionStates.degraded
                            ? ConnectiveTheme.warning
                            : ConnectiveTheme.success)
                        : ConnectiveTheme.textMuted,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            _ServerContext(store: store),
            if (connected &&
                selected != null &&
                selected.id != store.activeServerId) ...[
              const SizedBox(height: 8),
              Text(
                'Selected: ${selected.displayName} — press Connect to switch.',
                style: const TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 12),
              ),
            ],
            const SizedBox(height: 12),
            ConnectButton(store: store, onPressed: onConnect),
            const SizedBox(height: 12),
            const Divider(height: 1),
            const SizedBox(height: 10),
            _CompactStats(
                store: store,
                downHist: downHist,
                upHist: upHist),
          ],
        ),
      ),
    );
  }
}

/// The single server summary: flag + name + protocol/latency on one
/// block, health at the trailing edge, and the AUTO/manual mode as a
/// quiet caption underneath — never the same facts twice.
class _ServerContext extends StatelessWidget {
  final AppStore store;

  const _ServerContext({required this.store});

  @override
  Widget build(BuildContext context) {
    final connected =
        ConnectionStates.isConnected(store.connectionState);
    final server =
        connected ? store.activeServer : store.selectedServer;
    if (server == null && store.autoMode) {
      return const Row(
        children: [
          Icon(Icons.autorenew,
              size: 15, color: ConnectiveTheme.textSecondary),
          SizedBox(width: 6),
          Expanded(
            child: Text('AUTO — best server is picked automatically',
                style: TextStyle(
                    color: ConnectiveTheme.textSecondary,
                    fontSize: 13)),
          ),
        ],
      );
    }
    if (server == null) {
      return Row(
        children: [
          const Expanded(
            child: Text('Manual — selection unavailable',
                style: TextStyle(
                    color: ConnectiveTheme.textSecondary,
                    fontSize: 13)),
          ),
          TextButton(
            onPressed: () => store.selectServer('auto'),
            child: const Text('Use Auto'),
          ),
        ],
      );
    }
    final code = countryCodeOf(
        country: server.country,
        displayName: server.displayName);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            CountryFlag(code: code),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    server.displayName,
                    style: const TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: ConnectiveTheme.textPrimary),
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${server.compactInfo}  ·  ${server.latencyLabel}',
                    style: const TextStyle(
                        fontSize: 12,
                        color: ConnectiveTheme.textSecondary),
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            _HealthDot(health: server.health),
          ],
        ),
        const SizedBox(height: 6),
        Row(
          children: [
            if (store.autoMode) ...[
              const Icon(Icons.autorenew,
                  size: 14,
                  color: ConnectiveTheme.textMuted),
              const SizedBox(width: 6),
              const Expanded(
                child: Text('via AUTO selection',
                    style: TextStyle(
                        color: ConnectiveTheme.textMuted,
                        fontSize: 12)),
              ),
            ] else ...[
              Container(
                  width: 6,
                  height: 6,
                  decoration: const BoxDecoration(
                      color: ConnectiveTheme.success,
                      shape: BoxShape.circle)),
              const SizedBox(width: 6),
              const Text('SELECTED',
                  style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 0.5,
                      color: ConnectiveTheme.success)),
              const SizedBox(width: 6),
              const Expanded(
                child: Text('Manual selection',
                    style: TextStyle(
                        color: ConnectiveTheme.textMuted,
                        fontSize: 12),
                    overflow: TextOverflow.ellipsis),
              ),
              TextButton(
                onPressed: () => store.selectServer('auto'),
                style: TextButton.styleFrom(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 8, vertical: 4),
                  minimumSize: Size.zero,
                  tapTargetSize:
                      MaterialTapTargetSize.shrinkWrap,
                ),
                child: const Text('Use Auto'),
              ),
            ],
          ],
        ),
      ],
    );
  }
}

class _HealthDot extends StatelessWidget {
  final String health;
  const _HealthDot({required this.health});

  @override
  Widget build(BuildContext context) {
    final Color c;
    switch (health) {
      case 'healthy':
        c = ConnectiveTheme.success;
        break;
      case 'degraded':
        c = ConnectiveTheme.warning;
        break;
      case 'unhealthy':
        c = ConnectiveTheme.danger;
        break;
      default:
        c = ConnectiveTheme.textMuted;
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
            width: 7,
            height: 7,
            decoration:
                BoxDecoration(color: c, shape: BoxShape.circle)),
        const SizedBox(width: 6),
        Text(health,
            style: const TextStyle(
                fontSize: 12,
                color: ConnectiveTheme.textSecondary)),
      ],
    );
  }
}

/// Secondary traffic info: numbers first, one slim live graph.
class _CompactStats extends StatelessWidget {
  final AppStore store;
  final List<double> downHist;
  final List<double> upHist;

  const _CompactStats(
      {required this.store,
      required this.downHist,
      required this.upHist});

  @override
  Widget build(BuildContext context) {
    final s = store.stats;
    Widget stat(String label, String value) => Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label.toUpperCase(),
                  style: const TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 0.6,
                      color: ConnectiveTheme.textMuted)),
              const SizedBox(height: 2),
              Text(value,
                  style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      fontFeatures: [
                        FontFeature.tabularFigures()
                      ])),
            ],
          ),
        );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            stat('Down', ConnectiveTheme.rate(s.downRate)),
            stat('Up', ConnectiveTheme.rate(s.upRate)),
            stat('Session',
                ConnectiveTheme.total(s.downTotal + s.upTotal)),
            stat('Time', ConnectiveTheme.duration(s.durationMs)),
          ],
        ),
        const SizedBox(height: 8),
        SizedBox(
          height: 32,
          child: Stack(
            children: [
              Sparkline(
                  values: downHist,
                  color: ConnectiveTheme.success),
              Sparkline(
                  values: upHist,
                  color: ConnectiveTheme.info),
            ],
          ),
        ),
        const SizedBox(height: 4),
        const Row(
          children: [
            _LegendDot(
                color: ConnectiveTheme.success, label: 'Down'),
            SizedBox(width: 12),
            _LegendDot(
                color: ConnectiveTheme.info, label: 'Up'),
          ],
        ),
      ],
    );
  }
}

class _LegendDot extends StatelessWidget {
  final Color color;
  final String label;
  const _LegendDot({required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
            width: 6,
            height: 6,
            decoration:
                BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 5),
        Text(label,
            style: const TextStyle(
                fontSize: 11,
                color: ConnectiveTheme.textMuted)),
      ],
    );
  }
}

/// Tunnel capture switch on the dashboard: the most common routing
/// control, without a page hop. Same backend setting as
/// Routing → TUN; applies on the next connect.
class _TunCard extends StatelessWidget {
  final AppStore store;

  const _TunCard({required this.store});

  @override
  Widget build(BuildContext context) {
    final tun = store.settings.tunEnabled;
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.symmetric(
            horizontal: 12, vertical: 8),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: ConnectiveTheme.surfaceElevated,
                borderRadius: BorderRadius.circular(6),
                border: Border.all(
                    color: ConnectiveTheme.border),
              ),
              child: Icon(Icons.vpn_lock_outlined,
                  size: 17,
                  color: tun
                      ? ConnectiveTheme.success
                      : ConnectiveTheme.textMuted),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('TUN mode',
                      style: TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 13)),
                  Text(
                    tun
                        ? 'On — captures device traffic'
                        : 'Off',
                    style: const TextStyle(
                        fontSize: 12,
                        color:
                            ConnectiveTheme.textSecondary),
                  ),
                ],
              ),
            ),
            Switch(
              key: const Key('dash-tun-toggle'),
              value: tun,
              onChanged: (v) => store.updateSettings(
                  store.settings.copyWith(tunEnabled: v)),
            ),
          ],
        ),
      ),
    );
  }
}

/// Server management toolbar: section label + Update all, compact
/// search, and the entry points (subscription / manual server /
/// share-link import) absorbed from the removed Servers and
/// Subscriptions tabs.
class _ServerToolbar extends StatelessWidget {
  final AppStore store;
  final ValueChanged<String> onQuery;

  const _ServerToolbar({required this.store, required this.onQuery});

  @override
  Widget build(BuildContext context) {
    final updating =
        store.updatingSubs.values.any((v) => v);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Expanded(
              child: Text('SERVERS',
                  style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 0.8,
                      color: ConnectiveTheme.textMuted)),
            ),
            TextButton.icon(
              key: const Key('dash-update-all'),
              onPressed: store.subscriptions.isEmpty || updating
                  ? null
                  : () => store.updateSubscriptions(),
              icon: updating
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(
                          strokeWidth: 2))
                  : const Icon(Icons.sync, size: 15),
              label: const Text('Update all'),
            ),
          ],
        ),
        SearchBar(
          key: const Key('dash-search'),
          hintText: 'Search servers…',
          leading: const Icon(Icons.search, size: 18),
          onChanged: onQuery,
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                key: const Key('dash-add-subscription'),
                onPressed: () => showSubscriptionDialog(
                    context, store, null),
                icon: const Icon(Icons.add, size: 16),
                label: const Text('Add Subscription',
                    overflow: TextOverflow.ellipsis),
              ),
            ),
            const SizedBox(width: 8),
            OutlinedButton.icon(
              key: const Key('dash-add-server'),
              onPressed: () =>
                  showServerEditor(context, store, null),
              icon: const Icon(Icons.dns_outlined, size: 16),
              label: const Text('Server'),
            ),
            const SizedBox(width: 8),
            OutlinedButton.icon(
              key: const Key('dash-import-server'),
              onPressed: () =>
                  showImportDialog(context, store),
              icon: const Icon(Icons.link, size: 16),
              label: const Text('Import'),
            ),
          ],
        ),
      ],
    );
  }
}

String _targetLine(AppStore store) {
  final connected =
      ConnectionStates.isConnected(store.connectionState);
  if (connected) {
    final a = store.activeServer;
    if (a == null) return 'Connected';
    final lat =
        a.latencyMs >= 0 ? ' · ${a.latencyLabel}' : '';
    return '${a.displayName}$lat';
  }
  if (!store.autoMode) {
    final s = store.selectedServer;
    if (s != null) return 'Selected: ${s.displayName}';
    return 'Manual';
  }
  return 'Auto';
}

class _StateDot extends StatelessWidget {
  final String state;
  final double size;

  const _StateDot({required this.state, required this.size});

  @override
  Widget build(BuildContext context) {
    final Color c;
    if (ConnectionStates.isConnected(state)) {
      c = state == ConnectionStates.degraded
          ? ConnectiveTheme.warning
          : ConnectiveTheme.success;
    } else if (ConnectionStates.isBusy(state)) {
      c = ConnectiveTheme.info;
    } else if (state == ConnectionStates.error) {
      c = ConnectiveTheme.danger;
    } else {
      c = ConnectiveTheme.textMuted;
    }
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(color: c, shape: BoxShape.circle),
    );
  }
}
