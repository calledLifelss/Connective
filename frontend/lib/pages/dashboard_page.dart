import 'package:flutter/material.dart';

import '../components/connect_button.dart';
import '../components/server_row.dart';
import '../components/subscription_card.dart';
import '../components/update_banner.dart';
import '../models/connection_state.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import '../widgets/common.dart';

/// Home: the everyday connection experience (§1–§13, §21–§23).
///
/// One vertical scroll: compact connection section → Add Subscription →
/// search → subscriptions → servers. A persistent compact connection bar
/// fades in once the expanded section scrolls away, so the Connect
/// action is never lost. Pressing Connect uses the manual selection (or
/// AUTO) through the real backend, then smoothly returns attention to
/// the connection section.
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
  /// attention to the connection section (§11).
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

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        _pushHistory(store);
        return Column(
          children: [
            AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              curve: Curves.easeOut,
              height: _showCompact ? 60 : 0,
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
                      child: SizedBox(height: 12)),
                  ..._banners(store),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                          horizontal: ConnectiveTheme.pad),
                      child: _ConnectionCard(
                          store: store,
                          onConnect: _connect,
                          downHist: _downHist,
                          upHist: _upHist),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: UpdateBanner(store: store),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(
                          ConnectiveTheme.pad, 12, ConnectiveTheme.pad, 0),
                      child: _AddAndSearch(
                        store: store,
                        onQuery: (q) =>
                            setState(() => _query = q),
                      ),
                    ),
                  ),
                  ..._body(store),
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

  List<Widget> _banners(AppStore store) {
    final out = <Widget>[];
    SliverToBoxAdapter wrap(Widget c) => SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
                ConnectiveTheme.pad, 0, ConnectiveTheme.pad, 12),
            child: c,
          ),
        );
    if (store.foreignTun.isNotEmpty &&
        !ConnectionStates.isConnected(store.connectionState)) {
      out.add(wrap(Card(
        color: ConnectiveTheme.warning.withValues(alpha: 0.12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              const Icon(Icons.warning_amber,
                  color: ConnectiveTheme.warning, size: 20),
              const SizedBox(width: 10),
              Expanded(
                  child: Text(
                      'Another VPN tunnel (${store.foreignTun.join(', ')}) is active. Connecting may conflict — the backend will fail safely rather than corrupt routes.',
                      style: const TextStyle(fontSize: 13))),
            ],
          ),
        ),
      )));
    }
    if (!store.backendAlive) {
      out.add(wrap(Card(
        color: ConnectiveTheme.warning.withValues(alpha: 0.12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              const Icon(Icons.cloud_off,
                  color: ConnectiveTheme.warning, size: 20),
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
  List<Widget> _body(AppStore store) {
    const pad =
        EdgeInsets.symmetric(horizontal: ConnectiveTheme.pad);
    final q = _query.trim();
    if (q.isNotEmpty) {
      final results = store.search(q);
      return [
        SliverToBoxAdapter(
          child: Padding(
            padding: pad.copyWith(top: 12),
            child: Text('${results.length} result${results.length == 1 ? '' : 's'} for "$q"',
                style: const TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 13)),
          ),
        ),
        if (results.isEmpty)
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.only(top: 40),
              child: Center(child: Text('No servers match.')),
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
              const SectionHeader(title: 'LOCAL SERVERS'),
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
        child: Padding(
          padding: EdgeInsets.only(top: 32),
          child: Center(
              child: Text(
                  'No servers yet. Add a subscription to load servers.',
                  textAlign: TextAlign.center)),
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

/// Compact persistent connection bar (§12): pinned above the dashboard
/// scroll, visible once the expanded section scrolls away. Same backend
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
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Row(
            children: [
              _StateDot(state: state, size: 10),
              const SizedBox(width: 8),
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
                            fontWeight: FontWeight.w700, fontSize: 14)),
                    Text(_targetLine(store),
                        style: const TextStyle(
                            color: ConnectiveTheme.textSecondary,
                            fontSize: 12),
                        overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              IconButton(
                icon: const Icon(Icons.arrow_upward, size: 18),
                tooltip: 'Back to connection',
                onPressed: onTop,
              ),
              const SizedBox(width: 4),
              SizedBox(
                height: 36,
                child: FilledButton(
                  key: const Key('dash-compact-connect'),
                  onPressed: busy ? null : onConnect,
                  style: FilledButton.styleFrom(
                    backgroundColor: connected
                        ? ConnectiveTheme.success
                            .withValues(alpha: 0.2)
                        : ConnectiveTheme.accent,
                    foregroundColor: Colors.white,
                    padding:
                        const EdgeInsets.symmetric(horizontal: 16),
                  ),
                  child: busy
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                              strokeWidth: 2))
                      : Text(connected ? 'Disconnect' : 'Connect'),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Expanded connection section (§3): visually important but compact —
/// state, AUTO/selected, connected-vs-selected, real latency/health,
/// one Connect/Disconnect control, secondary slim stats.
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
    final active = store.activeServer;
    final selected = store.selectedServer;
    final activeCode = active == null
        ? null
        : countryCodeOf(
            country: active.country, displayName: active.displayName);
    return Card(
      key: const Key('dash-connection-expanded'),
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _StateDot(state: state, size: 12),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    ConnectionStates.label(state),
                    style: const TextStyle(
                        fontSize: 22, fontWeight: FontWeight.w800),
                  ),
                ),
                StatusBadge(
                  label: connected
                      ? (state == ConnectionStates.degraded
                          ? 'Degraded'
                          : 'Protected')
                      : 'Unprotected',
                  color: connected
                      ? (state == ConnectionStates.degraded
                          ? ConnectiveTheme.warning
                          : ConnectiveTheme.success)
                      : ConnectiveTheme.textSecondary,
                ),
              ],
            ),
            const SizedBox(height: 6),
            _ModeLine(store: store),
            if (connected && active != null) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  CountryFlag(code: activeCode),
                  StatusBadge(
                      label: active.compactInfo,
                      color: ConnectiveTheme.accent),
                  StatusBadge(
                      label: active.latencyLabel,
                      color: ConnectiveTheme.textSecondary),
                  StatusBadge(
                      label: active.health,
                      color: _healthColor(active.health)),
                ],
              ),
            ],
            if (connected &&
                selected != null &&
                selected.id != store.activeServerId) ...[
              const SizedBox(height: 6),
              Text(
                'Selected: ${selected.displayName} — press Connect to switch.',
                style: const TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 12),
              ),
            ],
            const SizedBox(height: 12),
            ConnectButton(store: store, onPressed: onConnect),
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

  Color _healthColor(String h) {
    switch (h) {
      case 'healthy':
        return ConnectiveTheme.success;
      case 'degraded':
        return ConnectiveTheme.warning;
      case 'unhealthy':
        return ConnectiveTheme.danger;
      default:
        return ConnectiveTheme.textSecondary;
    }
  }
}

/// AUTO vs SELECTED visual language (§19): unambiguous, one line.
class _ModeLine extends StatelessWidget {
  final AppStore store;

  const _ModeLine({required this.store});

  @override
  Widget build(BuildContext context) {
    if (store.autoMode) {
      return const Row(
        children: [
          Icon(Icons.auto_awesome,
              size: 16, color: ConnectiveTheme.accent),
          SizedBox(width: 6),
          Expanded(
            child: Text('AUTO — Connective picks the best server',
                style: TextStyle(
                    color: ConnectiveTheme.textSecondary)),
          ),
        ],
      );
    }
    final sel = store.selectedServer;
    final code = sel == null
        ? null
        : countryCodeOf(
            country: sel.country, displayName: sel.displayName);
    return Row(
      children: [
        const StatusDot(color: ConnectiveTheme.accent, label: 'SELECTED'),
        const SizedBox(width: 8),
        if (sel != null) CountryFlag(code: code, width: 18, height: 13),
        if (sel != null) const SizedBox(width: 6),
        Expanded(
          child: Text(
            sel == null
                ? 'Manual — selection unavailable'
                : '${sel.displayName} · ${sel.latencyLabel}',
            style: const TextStyle(fontWeight: FontWeight.w600),
            overflow: TextOverflow.ellipsis,
          ),
        ),
        TextButton(
          onPressed: () => store.selectServer('auto'),
          child: const Text('Use Auto'),
        ),
      ],
    );
  }
}

/// Secondary traffic info (§4): numbers first, one slim live graph.
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
              Text(label,
                  style: Theme.of(context)
                      .textTheme
                      .bodySmall
                      ?.copyWith(
                          color:
                              ConnectiveTheme.textSecondary)),
              const SizedBox(height: 1),
              Text(value,
                  style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700)),
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
        const SizedBox(height: 6),
        SizedBox(
          height: 28,
          child: Stack(
            children: [
              Sparkline(
                  values: downHist,
                  color: ConnectiveTheme.success),
              Sparkline(
                  values: upHist, color: ConnectiveTheme.accent),
            ],
          ),
        ),
      ],
    );
  }
}

/// Add Subscription entry point (§5) + server search (§15).
class _AddAndSearch extends StatelessWidget {
  final AppStore store;
  final ValueChanged<String> onQuery;

  const _AddAndSearch({required this.store, required this.onQuery});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SizedBox(
          width: double.infinity,
          child: FilledButton.tonalIcon(
            key: const Key('dash-add-subscription'),
            onPressed: () =>
                showSubscriptionDialog(context, store, null),
            icon: const Icon(Icons.add),
            label: const Text('Add Subscription'),
          ),
        ),
        const SizedBox(height: 8),
        SearchBar(
          key: const Key('dash-search'),
          hintText: 'Search servers…',
          leading: const Icon(Icons.search),
          onChanged: onQuery,
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
  return 'AUTO';
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
      c = ConnectiveTheme.accent;
    } else if (state == ConnectionStates.error) {
      c = ConnectiveTheme.danger;
    } else {
      c = ConnectiveTheme.textSecondary;
    }
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(color: c, shape: BoxShape.circle),
    );
  }
}
