import 'package:flutter/material.dart';

import '../components/server_dialogs.dart';
import '../components/server_row.dart';
import '../components/subscription_card.dart';
import '../components/update_banner.dart';
import '../models/connection_state.dart';
import '../models/server.dart';
import '../state/app_store.dart';
import '../theme/connective_theme.dart';
import '../utils/country.dart';
import '../widgets/common.dart';

/// Home: the everyday connection experience.
///
/// One vertical scroll: status hero (state → server → protection →
/// action → live stats) → VPN mode → server selection → servers.
/// A persistent compact connection bar fades in once the hero scrolls
/// away, so the Connect action is never lost. Pressing Connect uses
/// the manual selection (or AUTO) through the real backend, then
/// smoothly returns attention to the status hero.
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

  /// Sample times aligned with the rate histories: the graph's axis
  /// and hover readout label real moments, never a derived offset.
  final List<DateTime> _times = [];
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
  /// attention to the status hero.
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

  /// Content column: page padding only. The card stack stretches edge
  /// to edge so the dashboard fills the whole area right of the
  /// sidebar — same full-width scroll padding as the other pages.
  static const EdgeInsets _hPad =
      EdgeInsets.symmetric(horizontal: ConnectiveTheme.pad);

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.store,
      builder: (context, _) {
        final store = widget.store;
        const hPad = _hPad;
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
                // Generous cache so the toolbar controls stay mounted
                // alongside the hero on desktop viewports (avoids
                // lazy-build pop-in while scrolling; still lazy for
                // very long server lists).
                cacheExtent: 1000,
                slivers: [
                  const SliverToBoxAdapter(
                      child: SizedBox(height: 16)),
                  ..._banners(store, hPad),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad,
                      child: _ConnectionHero(
                          store: store,
                          onConnect: _connect,
                          downHist: _downHist,
                          upHist: _upHist,
                          times: _times),
                    ),
                  ),
                  // Update notice sits directly under the hero (zero
                  // footprint when quiet): important enough to see
                  // without scrolling, never buried under sections.
                  SliverToBoxAdapter(
                    child: UpdateBanner(store: store),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad.copyWith(top: 16),
                      child: _TunSection(store: store),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: hPad.copyWith(top: 8),
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
    if (store.routingPendingReconnect) {
      out.add(wrap(Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 10),
          child: Row(
            children: [
              const Icon(Icons.settings_backup_restore,
                  color: ConnectiveTheme.warning, size: 18),
              const SizedBox(width: 10),
              const Expanded(
                  child: Text(
                      'Routing changes are saved but not active yet — reconnect to apply them.',
                      style: TextStyle(fontSize: 13, height: 1.4))),
              TextButton(
                key: const Key('routing-reconnect-button'),
                onPressed: store.reconnectToApply,
                child: const Text('Reconnect'),
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

  /// Subscriptions + servers (or a flat filtered view), lazily built.
  List<Widget> _body(AppStore store, EdgeInsets hPad) {
    final pad = hPad;
    final q = _query.trim();
    final filtered = store.serverFilter != 'all' || q.isNotEmpty;
    if (filtered) {
      final results =
          store.applyServerView(store.servers, query: q);
      final filterLabel = store.serverFilter == 'all'
          ? ''
          : ' · ${store.serverFilter}';
      return [
        SliverToBoxAdapter(
          child: Padding(
            padding: pad.copyWith(top: 4),
            child: Text(
                '${results.length} result${results.length == 1 ? '' : 's'}${q.isNotEmpty ? ' for "$q"' : ''}$filterLabel',
                style: const TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 12)),
          ),
        ),
        if (results.isEmpty)
          SliverToBoxAdapter(
            child: SizedBox(
              height: 160,
              child: Center(
                  child: Text(
                      store.serverFilter == 'favorites'
                          ? 'No favorites yet. Star a server to pin it here.'
                          : 'No servers match.',
                      textAlign: TextAlign.center,
                      style: const TextStyle(
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
                subscriptionName:
                    store.subscriptionName(results[i].subscriptionId),
              ),
            ),
          ),
      ];
    }

    final slivers = <Widget>[];
    for (final sub in store.subscriptions) {
      final children = store.applyServerView(
          store.serversOf(sub.id));
      final expanded = store.isExpandedSub(sub.id);
      slivers.add(SliverToBoxAdapter(
        child: Padding(
          padding: pad.copyWith(top: 12),
          child: _SubscriptionSection(store: store, subId: sub.id),
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
    final locals =
        store.applyServerView(store.localServers);
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
    _times.add(DateTime.now());
    // Keep a few minutes of samples for hover inspection; the graph
    // renders the last windowSec samples.
    if (_downHist.length > 300) {
      _downHist.removeRange(0, _downHist.length - 300);
      _upHist.removeRange(0, _upHist.length - 300);
      _times.removeRange(0, _times.length - 300);
    }
  }
}

/// Compact persistent connection bar: pinned above the dashboard
/// scroll, visible once the status hero scrolls away. Same backend
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

/// Status hero: the single most prominent block on the page.
///
/// Hierarchy: state → server + latency → protection → action, with
/// technical details (protocol, selection mode) secondary. Live
/// traffic follows below a divider inside the same card so the page
/// stays flat instead of stacking nested panels.
class _ConnectionHero extends StatelessWidget {
  final AppStore store;
  final Future<void> Function() onConnect;
  final List<double> downHist;
  final List<double> upHist;
  final List<DateTime> times;

  const _ConnectionHero(
      {required this.store,
      required this.onConnect,
      required this.downHist,
      required this.upHist,
      required this.times});

  @override
  Widget build(BuildContext context) {
    final state = store.connectionState;
    final connected = ConnectionStates.isConnected(state);
    final busy = ConnectionStates.isBusy(state);
    final selected = store.selectedServer;
    final shown =
        connected ? store.activeServer : store.connectTarget;
    final lat = shown != null && shown.latencyMs >= 0
        ? '  ·  ${shown.latencyLabel}'
        : '';

    return Card(
      key: const Key('dash-connection-expanded'),
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Power affordance mirrors the connect action.
                _PowerCircle(
                    state: state,
                    busy: busy,
                    onTap: busy ? null : onConnect),
                const SizedBox(width: 14),
                Expanded(
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
                                  fontSize: 18,
                                  fontWeight: FontWeight.w700,
                                  height: 1.2,
                                  color:
                                      ConnectiveTheme.textPrimary),
                            ),
                          ),
                          Text(
                            connected ? 'PROTECTED' : 'NOT PROTECTED',
                            style: TextStyle(
                              fontSize: 10.5,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 0.6,
                              color: connected
                                  ? ConnectiveTheme.success
                                  : ConnectiveTheme.textMuted,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      if (shown != null)
                        _HeroServerLine(
                            server: shown, latencySuffix: lat)
                      else if (store.autoMode)
                        const Row(
                          children: [
                            Icon(Icons.autorenew,
                                size: 15,
                                color: ConnectiveTheme.textSecondary),
                            SizedBox(width: 6),
                            Expanded(
                              child: Text(
                                  'AUTO — best server is picked automatically',
                                  style: TextStyle(
                                      color: ConnectiveTheme
                                          .textSecondary,
                                      fontSize: 13)),
                            ),
                          ],
                        )
                      else
                        const Text('Manual — selection unavailable',
                            style: TextStyle(
                                color:
                                    ConnectiveTheme.textSecondary,
                                fontSize: 13)),
                      if (shown != null) ...[
                        const SizedBox(height: 3),
                        Text(
                          shown.compactInfo,
                          style: const TextStyle(
                              fontSize: 11.5,
                              letterSpacing: 0.3,
                              color: ConnectiveTheme.textMuted),
                        ),
                      ],
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          Icon(
                            connected
                                ? Icons.shield_outlined
                                : Icons.shield_outlined,
                            size: 14,
                            color: connected
                                ? ConnectiveTheme.success
                                : ConnectiveTheme.textMuted,
                          ),
                          const SizedBox(width: 6),
                          Expanded(
                            child: Text(
                              connected
                                  ? 'Traffic is protected through tunnel'
                                  : 'Traffic is not protected',
                              style: TextStyle(
                                  fontSize: 12.5,
                                  color: connected
                                      ? ConnectiveTheme.success
                                      : ConnectiveTheme.textSecondary),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      _ModeCaption(store: store),
                    ],
                  ),
                ),
                const SizedBox(width: 12),
                // The one primary action: always visible, top-aligned.
                SizedBox(
                  height: 40,
                  child: connected
                      ? OutlinedButton(
                          key: const Key('connect-button'),
                          onPressed: busy ? null : onConnect,
                          style: OutlinedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 20),
                          ),
                          child: const Text('Disconnect'),
                        )
                      : FilledButton(
                          key: const Key('connect-button'),
                          onPressed: busy ? null : onConnect,
                          style: FilledButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 24),
                          ),
                          child: busy
                              ? const SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      valueColor:
                                          AlwaysStoppedAnimation<
                                                  Color>(
                                              ConnectiveTheme
                                                  .onAccent)),
                                )
                              : Text(state ==
                                      ConnectionStates.error
                                  ? 'Retry'
                                  : 'Connect'),
                        ),
                ),
              ],
            ),
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
            if (!connected && busy) ...[
              const SizedBox(height: 8),
              const Text(
                'Working — please wait…',
                style: TextStyle(
                    color: ConnectiveTheme.textSecondary, fontSize: 12),
              ),
            ],
            const SizedBox(height: 12),
            const Divider(height: 1),
            const SizedBox(height: 12),
            _LiveStats(
                store: store,
                downHist: downHist,
                upHist: upHist,
                times: times),
          ],
        ),
      ),
    );
  }
}

class _PowerCircle extends StatelessWidget {
  final String state;
  final bool busy;
  final Future<void> Function()? onTap;

  const _PowerCircle(
      {required this.state, required this.busy, this.onTap});

  @override
  Widget build(BuildContext context) {
    final connected = ConnectionStates.isConnected(state);
    final degraded = state == ConnectionStates.degraded;
    final error = state == ConnectionStates.error;
    final Color ring;
    final Color icon;
    if (connected && !degraded) {
      ring = ConnectiveTheme.success;
      icon = ConnectiveTheme.success;
    } else if (degraded) {
      ring = ConnectiveTheme.warning;
      icon = ConnectiveTheme.warning;
    } else if (error) {
      ring = ConnectiveTheme.danger;
      icon = ConnectiveTheme.danger;
    } else if (busy) {
      ring = ConnectiveTheme.info;
      icon = ConnectiveTheme.textSecondary;
    } else {
      ring = ConnectiveTheme.border;
      icon = ConnectiveTheme.textSecondary;
    }
    return Material(
      color: Colors.transparent,
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: Container(
          width: 54,
          height: 54,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: connected && !degraded
                ? ConnectiveTheme.accentDim.withValues(alpha: 0.35)
                : ConnectiveTheme.surfaceElevated,
            border: Border.all(color: ring, width: 1.6),
          ),
          child: Center(
            child: busy
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child:
                        CircularProgressIndicator(strokeWidth: 2))
                : Icon(Icons.power_settings_new,
                    size: 24, color: icon),
          ),
        ),
      ),
    );
  }
}

class _HeroServerLine extends StatelessWidget {
  final Server server;
  final String latencySuffix;

  const _HeroServerLine(
      {required this.server, required this.latencySuffix});

  @override
  Widget build(BuildContext context) {
    final code = countryCodeOf(
        country: server.country, displayName: server.displayName);
    return Row(
      children: [
        CountryFlag(code: code, width: 24, height: 17),
        const SizedBox(width: 8),
        Expanded(
          child: RichText(
            overflow: TextOverflow.ellipsis,
            text: TextSpan(
              style: const TextStyle(
                  fontSize: 15.5,
                  fontWeight: FontWeight.w700,
                  color: ConnectiveTheme.textPrimary),
              children: [
                TextSpan(text: server.displayName),
                TextSpan(
                  text: latencySuffix,
                  style: const TextStyle(
                      fontSize: 12.5,
                      fontWeight: FontWeight.w500,
                      color: ConnectiveTheme.textSecondary,
                      fontFeatures: [
                        FontFeature.tabularFigures()
                      ]),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

/// Quiet selection caption: never restates the server facts, only the
/// mode. Keeps the legacy "SELECTED" / "Use Auto" / "AUTO" strings
/// the tests and users rely on.
class _ModeCaption extends StatelessWidget {
  final AppStore store;
  const _ModeCaption({required this.store});

  @override
  Widget build(BuildContext context) {
    if (store.autoMode) {
      return const Row(
        children: [
          Icon(Icons.autorenew,
              size: 13, color: ConnectiveTheme.textMuted),
          SizedBox(width: 6),
          Expanded(
            child: Text('via AUTO selection',
                style: TextStyle(
                    color: ConnectiveTheme.textMuted,
                    fontSize: 12)),
          ),
        ],
      );
    }
    return const Row(
      children: [
        SizedBox(
          width: 6,
          height: 6,
          child: DecoratedBox(
            decoration: BoxDecoration(
                color: ConnectiveTheme.success,
                shape: BoxShape.circle),
          ),
        ),
        SizedBox(width: 6),
        Text('SELECTED',
            style: TextStyle(
                fontSize: 10.5,
                fontWeight: FontWeight.w700,
                letterSpacing: 0.5,
                color: ConnectiveTheme.success)),
        SizedBox(width: 6),
        Expanded(
          child: Text('Manual selection',
              style: TextStyle(
                  color: ConnectiveTheme.textMuted,
                  fontSize: 12),
              overflow: TextOverflow.ellipsis),
        ),
      ],
    );
  }
}

/// Live traffic: values first (large + tabular), labels quiet above.
/// Functional graph below with axes, hover readout and legend.
class _LiveStats extends StatelessWidget {
  final AppStore store;
  final List<double> downHist;
  final List<double> upHist;
  final List<DateTime> times;

  const _LiveStats(
      {required this.store,
      required this.downHist,
      required this.upHist,
      required this.times});

  @override
  Widget build(BuildContext context) {
    final s = store.stats;
    Widget stat(
        {required String label,
        required IconData icon,
        required String value,
        required String semantic}) {
      return Expanded(
        child: Semantics(
          label: '$label $semantic',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(icon,
                      size: 12,
                      color: ConnectiveTheme.textMuted),
                  const SizedBox(width: 5),
                  Text(label.toUpperCase(),
                      style: const TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                          letterSpacing: 0.7,
                          color: ConnectiveTheme.textMuted)),
                ],
              ),
              const SizedBox(height: 4),
              Text(value,
                  style: const TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.w700,
                      height: 1.15,
                      fontFeatures: [
                        FontFeature.tabularFigures()
                      ])),
            ],
          ),
        ),
      );
    }

    final session = ConnectiveTheme.total(s.downTotal + s.upTotal);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.show_chart,
                size: 13, color: ConnectiveTheme.textMuted),
            SizedBox(width: 6),
            Expanded(
              child: Text('LIVE TRAFFIC',
                  style: TextStyle(
                      fontSize: 10.5,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 0.8,
                      color: ConnectiveTheme.textMuted)),
            ),
            _LiveDot(),
          ],
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            stat(
                label: 'Down',
                icon: Icons.arrow_downward,
                value: ConnectiveTheme.rate(s.downRate),
                semantic: 'download rate'),
            stat(
                label: 'Up',
                icon: Icons.arrow_upward,
                value: ConnectiveTheme.rate(s.upRate),
                semantic: 'upload rate'),
            stat(
                label: 'Session',
                icon: Icons.storage_outlined,
                value: session,
                semantic: 'session total'),
            stat(
                label: 'Time',
                icon: Icons.schedule,
                value: ConnectiveTheme.duration(s.durationMs),
                semantic: 'connection duration'),
          ],
        ),
        const SizedBox(height: 10),
        TrafficGraph(
          down: downHist,
          up: upHist,
          times: times,
          downLabel: ConnectiveTheme.rate(s.downRate),
          upLabel: ConnectiveTheme.rate(s.upRate),
        ),
      ],
    );
  }
}

class _LiveDot extends StatefulWidget {
  const _LiveDot();
  @override
  State<_LiveDot> createState() => _LiveDotState();
}

class _LiveDotState extends State<_LiveDot>
    with SingleTickerProviderStateMixin {
  late final AnimationController _c;

  @override
  void initState() {
    super.initState();
    _c = AnimationController(
        vsync: this,
        duration: const Duration(milliseconds: 1600))
      ..repeat(reverse: true);
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: Tween(begin: 1.0, end: 0.35).animate(_c),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
              width: 7,
              height: 7,
              decoration: const BoxDecoration(
                  color: ConnectiveTheme.success,
                  shape: BoxShape.circle)),
          const SizedBox(width: 6),
          const Text('LIVE',
              style: TextStyle(
                  fontSize: 10.5,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 0.7,
                  color: ConnectiveTheme.success)),
        ],
      ),
    );
  }
}

/// VPN mode: a single flat control row — no nested card. The purpose
/// is stated plainly; the technical term TUN is kept and explained
/// via the info tooltip for technical users.
class _TunSection extends StatelessWidget {
  final AppStore store;

  const _TunSection({required this.store});

  @override
  Widget build(BuildContext context) {
    final tun = store.settings.tunEnabled;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'VPN mode'),
        Container(
          decoration: BoxDecoration(
            color: ConnectiveTheme.surface,
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radius),
            border: Border.all(color: ConnectiveTheme.border),
          ),
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 10),
          child: Row(
            children: [
              Container(
                width: 34,
                height: 34,
                decoration: BoxDecoration(
                  color: tun
                      ? ConnectiveTheme.success
                          .withValues(alpha: 0.12)
                      : ConnectiveTheme.surfaceElevated,
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(
                      color: ConnectiveTheme.border),
                ),
                child: Icon(Icons.shield_outlined,
                    size: 18,
                    color: tun
                        ? ConnectiveTheme.success
                        : ConnectiveTheme.textMuted),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(
                          tun ? 'TUN enabled' : 'TUN disabled',
                          style: const TextStyle(
                              fontWeight: FontWeight.w600,
                              fontSize: 13.5),
                        ),
                        const SizedBox(width: 4),
                        Tooltip(
                          message:
                              'TUN mode captures all device traffic into the VPN tunnel (full-tunnel). When off, only proxied app traffic uses the VPN. Applies on the next connect.',
                          preferBelow: false,
                          child: const Icon(
                              Icons.info_outline,
                              size: 14,
                              color:
                                  ConnectiveTheme.textMuted),
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      tun
                          ? 'All device traffic routed through VPN'
                          : 'Only proxied traffic uses the VPN',
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
      ],
    );
  }
}

/// Server management toolbar: selection mode, section label +
/// Update all, compact search, filter chips + sort, and the entry
/// points (subscription / manual server / share-link import).
class _ServerToolbar extends StatelessWidget {
  final AppStore store;
  final ValueChanged<String> onQuery;

  const _ServerToolbar({required this.store, required this.onQuery});

  static const _filters = [
    ('all', 'All'),
    ('healthy', 'Healthy'),
    ('favorites', 'Favorites'),
    ('fastest', 'Fastest'),
    ('recent', 'Recently used'),
  ];

  @override
  Widget build(BuildContext context) {
    final updating =
        store.updatingSubs.values.any((v) => v);
    // NOTE (layout contract): this toolbar shares one sliver whose
    // top sits above the fold, so the block paints (and its early
    // controls are tappable) without scrolling; the mode switch
    // sits last so it stays next to the server list — and
    // reachable — deep in the page.
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SectionHeader(
          title: 'Servers',
          action: TextButton.icon(
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
        ),
        LayoutBuilder(          builder: (context, constraints) {
            final narrow = constraints.maxWidth < 520;
            final search = SearchBar(
              key: const Key('dash-search'),
              hintText: 'Search servers…',
              leading: const Icon(Icons.search, size: 18),
              onChanged: onQuery,
            );
            final sort = _SortMenu(store: store);
            if (narrow) {
              return Column(
                children: [
                  SizedBox(
                      width: double.infinity, child: search),
                  const SizedBox(height: 8),
                  Align(
                      alignment: Alignment.centerRight,
                      child: sort),
                ],
              );
            }
            return Row(
              children: [
                Expanded(child: search),
                const SizedBox(width: 8),
                sort,
              ],
            );
          },
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 6,
          runSpacing: 6,
          children: [
            for (final (value, label) in _filters)
              _FilterChip(
                label: label,
                selected: store.serverFilter == value,
                onTap: () => store.setServerFilter(value),
              ),
          ],
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            OutlinedButton.icon(
              key: const Key('dash-add-subscription'),
              onPressed: () => showSubscriptionDialog(
                  context, store, null),
              icon: const Icon(Icons.add, size: 16),
              label: const Text('Add Subscription'),
            ),
            OutlinedButton.icon(
              key: const Key('dash-add-server'),
              onPressed: () =>
                  showServerEditor(context, store, null),
              icon: const Icon(Icons.dns_outlined, size: 16),
              label: const Text('Server'),
            ),
            OutlinedButton.icon(
              key: const Key('dash-import-server'),
              onPressed: () =>
                  showImportDialog(context, store),
              icon: const Icon(Icons.link, size: 16),
              label: const Text('Import'),
            ),
          ],
        ),
        const SizedBox(height: 8),
        const SectionHeader(title: 'Server selection'),
        // Clear Manual/Auto concept with the active mode indicated.
        // Sits last, right above the list it governs.
        Container(
          decoration: BoxDecoration(
            color: ConnectiveTheme.surface,
            borderRadius:
                BorderRadius.circular(ConnectiveTheme.radiusSmall),
            border: Border.all(color: ConnectiveTheme.border),
          ),
          padding: const EdgeInsets.all(4),
          child: Row(
            children: [
              Expanded(
                child: _ModeButton(
                  label: 'Manual',
                  icon: Icons.touch_app_outlined,
                  selected: !store.autoMode,
                  onTap: () {
                    final s = store.selectedServer ??
                        store.activeServer;
                    if (s != null) {
                      store.selectServer(s.id);
                    }
                  },
                  tooltip:
                      'Use the manually selected server',
                ),
              ),
              const SizedBox(width: 4),
              Expanded(
                child: _ModeButton(
                  label: 'Auto',
                  icon: Icons.autorenew,
                  selected: store.autoMode,
                  onTap: () => store.selectServer('auto'),
                  tooltip:
                      'Connective picks the best server automatically',
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 4),
        if (store.autoMode)
          const Text(
            'AUTO — Connective will choose an appropriate server.',
            style: TextStyle(
                fontSize: 12,
                color: ConnectiveTheme.textSecondary),
          )
        else
          Row(
            children: [
              Expanded(
                child: Text(
                  store.selectedServer != null
                      ? 'Manual — ${store.selectedServer!.displayName} will be used.'
                      : 'Manual — select a server below.',
                  style: const TextStyle(
                      fontSize: 12,
                      color: ConnectiveTheme.textSecondary),
                ),
              ),
              // The single "back to Auto" escape hatch: it lives
              // here next to the servers (not duplicated in the
              // hero), so it stays reachable at any scroll offset.
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
          ),
      ],
    );
  }
}

class _ModeButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final bool selected;
  final VoidCallback onTap;
  final String tooltip;

  const _ModeButton(
      {required this.label,
      required this.icon,
      required this.selected,
      required this.onTap,
      required this.tooltip});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: selected
            ? ConnectiveTheme.success.withValues(alpha: 0.14)
            : Colors.transparent,
        borderRadius: BorderRadius.circular(5),
        child: InkWell(
          borderRadius: BorderRadius.circular(5),
          onTap: onTap,
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 9),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(5),
              border: Border.all(
                  color: selected
                      ? ConnectiveTheme.success
                          .withValues(alpha: 0.5)
                      : Colors.transparent),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(icon,
                    size: 15,
                    color: selected
                        ? ConnectiveTheme.success
                        : ConnectiveTheme.textSecondary),
                const SizedBox(width: 6),
                Text(label,
                    style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: selected
                            ? ConnectiveTheme.textPrimary
                            : ConnectiveTheme.textSecondary)),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _FilterChip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _FilterChip(
      {required this.label,
      required this.selected,
      required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected
          ? ConnectiveTheme.success.withValues(alpha: 0.14)
          : ConnectiveTheme.surfaceElevated,
      borderRadius: BorderRadius.circular(16),
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(
              horizontal: 12, vertical: 7),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
                color: selected
                    ? ConnectiveTheme.success
                        .withValues(alpha: 0.55)
                    : ConnectiveTheme.border),
          ),
          child: Text(label,
              style: TextStyle(
                  fontSize: 12,
                  fontWeight:
                      selected ? FontWeight.w600 : FontWeight.w500,
                  color: selected
                      ? ConnectiveTheme.success
                      : ConnectiveTheme.textSecondary)),
        ),
      ),
    );
  }
}

class _SortMenu extends StatelessWidget {
  final AppStore store;
  const _SortMenu({required this.store});

  @override
  Widget build(BuildContext context) {
    final current = switch (store.serverSort) {
      'name' => 'Name',
      'load' => 'Load',
      _ => 'Latency',
    };
    return PopupMenuButton<String>(
      key: const Key('dash-sort'),
      tooltip: 'Sort servers',
      onSelected: (v) => store.setServerSort(v),
      itemBuilder: (context) => const [
        PopupMenuItem(value: 'latency', child: Text('Latency')),
        PopupMenuItem(value: 'name', child: Text('Name')),
        PopupMenuItem(value: 'load', child: Text('Load / status')),
      ],
      child: Container(
        padding: const EdgeInsets.symmetric(
            horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: ConnectiveTheme.surfaceElevated,
          borderRadius: BorderRadius.circular(
              ConnectiveTheme.radiusSmall),
          border: Border.all(color: ConnectiveTheme.border),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('Sort by:',
                style: TextStyle(
                    fontSize: 12,
                    color: ConnectiveTheme.textMuted)),
            const SizedBox(width: 6),
            Text(current,
                style: const TextStyle(
                    fontSize: 12.5,
                    fontWeight: FontWeight.w600)),
            const SizedBox(width: 4),
            const Icon(Icons.arrow_drop_down,
                size: 16,
                color: ConnectiveTheme.textSecondary),
          ],
        ),
      ),
    );
  }
}

/// Subscription grouping header: account-style summary (name,
/// server count, usage, expiry, quota bar) with refresh + manage.
/// Tapping the name row collapses the server list underneath.
class _SubscriptionSection extends StatelessWidget {
  final AppStore store;
  final String subId;

  const _SubscriptionSection(
      {required this.store, required this.subId});

  @override
  Widget build(BuildContext context) {
    final sub = store.subscriptions.firstWhere((s) => s.id == subId);
    return Card(
      margin: EdgeInsets.zero,
      child: SubscriptionHeader(store: store, sub: sub),
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
    return Semantics(
      label: 'State: ${ConnectionStates.label(state)}',
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(color: c, shape: BoxShape.circle),
      ),
    );
  }
}
