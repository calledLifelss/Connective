import 'dart:io';

import 'package:flutter/material.dart';
import 'package:launch_at_startup/launch_at_startup.dart';
import 'package:tray_manager/legacy.dart';
import 'package:window_manager/window_manager.dart';

import 'pages/dashboard_page.dart';
import 'pages/logs_page.dart';
import 'pages/routing_page.dart';
import 'pages/servers_page.dart';
import 'pages/settings_page.dart';
import 'pages/subscriptions_page.dart';
import 'services/backend_launcher.dart';
import 'services/notifier.dart';
import 'state/app_store.dart';
import 'theme/connective_theme.dart';

/// Connective entrypoint. The UI owns presentation only; all networking
/// truth comes from connectived over IPC (docs/IPC_CONTRACT.md).
final BackendLauncher _launcher = BackendLauncher();

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final store = AppStore();
  await Notifier.init();
  await _initDesktop(store);
  await _bootBackend(store);
  runApp(ConnectiveApp(store: store, launcher: _launcher));
}

Future<void> _bootBackend(AppStore store) async {
  // Start (or attach to) the backend first; the UI cannot work without it.
  final ok = await _launcher.ensure(null);
  if (!ok) {
    store.lastError =
        'Backend not found: place connectived next to the app or on PATH.';
    return;
  }
  final sock = BackendLauncher.socketPath();
  for (var i = 0; i < 10; i++) {
    try {
      await store.boot(sock);
      return;
    } catch (_) {
      await Future.delayed(const Duration(seconds: 1));
    }
  }
  // Boot visible without backend; pages show reconnect guidance.
  store.lastError =
      'Could not reach connectived. Start the backend and restart.';
}

/// Desktop integration (§14): tray, minimize-to-tray, window behavior.
/// Runs only in the real app, never in widget tests.
Future<void> _initDesktop(AppStore store) async {
  try {
    await windowManager.ensureInitialized();
    await windowManager.setTitle('Connective');
    await windowManager.setMinimumSize(const Size(900, 620));
    await trayManager.setIcon('assets/tray.png');
    await trayManager.setToolTip('Connective');
    await trayManager.setContextMenu(Menu(items: [
      MenuItem(key: 'show', label: 'Show'),
      MenuItem(key: 'quit', label: 'Quit'),
    ]));
    launchAtStartup.setup(
      appName: 'connective',
      appPath: Platform.resolvedExecutable,
    );
  } catch (_) {
    // Headless/test environments: desktop integration is optional.
  }
}

class ConnectiveApp extends StatefulWidget {
  final AppStore store;
  final BackendLauncher? launcher;

  const ConnectiveApp({super.key, required this.store, this.launcher});

  @override
  State<ConnectiveApp> createState() => _ConnectiveAppState();
}

class _ConnectiveAppState extends State<ConnectiveApp>
    with TrayListener, WindowListener {
  int _index = 0;

  @override
  void initState() {
    super.initState;
    trayManager.addListener(this);
    windowManager.addListener(this);
  }

  @override
  void dispose() {
    trayManager.removeListener(this);
    windowManager.removeListener(this);
    // NOTE: the store is injected (owned by main/test), never disposed here.
    super.dispose();
  }

  @override
  void onTrayIconMouseDown() => windowManager.show();

  @override
  void onTrayMenuItemClick(MenuItem menuItem) {
    if (menuItem.key == 'show') windowManager.show();
    if (menuItem.key == 'quit') _quit();
  }

  Future<void> _quit() async {
    await widget.launcher?.stopOwned();
    exit(0);
  }

  @override
  void onWindowClose() async {
    // Minimize to tray instead of quitting (§14).
    await windowManager.hide();
  }

  @override
  Widget build(BuildContext context) {
    final pages = [
      DashboardPage(store: widget.store),
      ServersPage(store: widget.store),
      SubscriptionsPage(store: widget.store),
      RoutingPage(store: widget.store),
      SettingsPage(store: widget.store),
      LogsPage(store: widget.store),
    ];
    const labels = [
      'Dashboard',
      'Servers',
      'Subscriptions',
      'Routing',
      'Settings',
      'Logs'
    ];
    const icons = [
      Icons.dashboard_outlined,
      Icons.dns_outlined,
      Icons.subscriptions_outlined,
      Icons.route_outlined,
      Icons.settings_outlined,
      Icons.terminal_outlined,
    ];
    return MaterialApp(
      title: 'Connective',
      theme: ConnectiveTheme.dark(),
      home: Scaffold(
        body: Row(
          children: [
            NavigationRail(
              selectedIndex: _index,
              onDestinationSelected: (i) =>
                  setState(() => _index = i),
              labelType: NavigationRailLabelType.all,
              destinations: [
                for (var i = 0; i < labels.length; i++)
                  NavigationRailDestination(
                      icon: Icon(icons[i]),
                      label: Text(labels[i])),
              ],
            ),
            const VerticalDivider(width: 1),
            Expanded(child: pages[_index]),
          ],
        ),
      ),
    );
  }
}
