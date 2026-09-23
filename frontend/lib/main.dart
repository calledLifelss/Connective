import 'dart:io';

import 'package:flutter/material.dart';
import 'package:launch_at_startup/launch_at_startup.dart';
import 'package:tray_manager/legacy.dart';
import 'package:window_manager/window_manager.dart';

import 'models/connection_state.dart';
import 'pages/dashboard_page.dart';
import 'pages/logs_page.dart';
import 'pages/routing_page.dart';
import 'pages/settings_page.dart';
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

/// Desktop integration: tray, minimize-to-tray, window behavior.
/// Runs only in the real app, never in widget tests.
Future<void> _initDesktop(AppStore store) async {
  try {
    await windowManager.ensureInitialized();
    await windowManager.setTitle('Connective');
    await windowManager.setMinimumSize(const Size(900, 620));
    // tray_manager needs .ico on Windows, .png elsewhere.
    await trayManager.setIcon(
        Platform.isWindows ? 'assets/tray.ico' : 'assets/tray.png');
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
    widget.store.quitApp = _quit;
    trayManager.addListener(this);
    windowManager.addListener(this);
  }

  @override
  void dispose() {
    widget.store.quitApp = null;
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
    // Minimize to tray instead of quitting.
    await windowManager.hide();
  }

  @override
  Widget build(BuildContext context) {
    final pages = [
      DashboardPage(store: widget.store),
      RoutingPage(store: widget.store),
      SettingsPage(store: widget.store),
      LogsPage(store: widget.store),
    ];
    return MaterialApp(
      title: 'Connective',
      theme: ConnectiveTheme.dark(),
      debugShowCheckedModeBanner: false,
      home: Scaffold(
        body: Row(
          children: [
            _Sidebar(
              store: widget.store,
              index: _index,
              onSelect: (i) => setState(() => _index = i),
            ),
            Container(
                width: 1, color: ConnectiveTheme.border),
            Expanded(child: pages[_index]),
          ],
        ),
      ),
    );
  }
}

/// Native-feeling left navigation: fixed 200px rail, product header,
/// plain list rows, subtle selected wash with a thin green edge.
class _Sidebar extends StatelessWidget {
  final AppStore store;
  final int index;
  final ValueChanged<int> onSelect;

  const _Sidebar(
      {required this.store, required this.index, required this.onSelect});

  static const _labels = [
    'Dashboard',
    'Routing',
    'Settings',
    'Logs',
  ];

  static const _icons = [
    Icons.home_outlined,
    Icons.route_outlined,
    Icons.settings_outlined,
    Icons.terminal_outlined,
  ];

  static const _iconsActive = [
    Icons.home,
    Icons.route,
    Icons.settings,
    Icons.terminal,
  ];

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 200,
      color: ConnectiveTheme.surface,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 14, 14, 10),
            child: Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: BoxDecoration(
                    color: ConnectiveTheme.surfaceElevated,
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                        color: ConnectiveTheme.border),
                  ),
                  child: const Icon(
                      Icons.shield_outlined,
                      size: 16,
                      color: ConnectiveTheme.textPrimary),
                ),
                const SizedBox(width: 10),
                const Expanded(
                  child: Column(
                    crossAxisAlignment:
                        CrossAxisAlignment.start,
                    children: [
                      Text('Connective',
                          style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600)),
                      Text('VPN client',
                          style: TextStyle(
                              fontSize: 11,
                              color: ConnectiveTheme
                                  .textSecondary)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const Divider(height: 1),
          const SizedBox(height: 8),
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(
                  horizontal: 8, vertical: 2),
              itemCount: _labels.length,
              itemBuilder: (context, i) {
                final selected = i == index;
                return Padding(
                  padding:
                      const EdgeInsets.symmetric(vertical: 2),
                  child: Material(
                    color: Colors.transparent,
                    borderRadius: BorderRadius.circular(6),
                    child: InkWell(
                      borderRadius: BorderRadius.circular(6),
                      onTap: () => onSelect(i),
                      child: Container(
                        decoration: BoxDecoration(
                          color: selected
                              ? ConnectiveTheme.surfaceElevated
                              : Colors.transparent,
                          borderRadius:
                              BorderRadius.circular(6),
                          border: Border(
                            left: BorderSide(
                              color: selected
                                  ? ConnectiveTheme.accent
                                  : Colors.transparent,
                              width: 2,
                            ),
                          ),
                        ),
                        padding: const EdgeInsets.symmetric(
                            horizontal: 10, vertical: 8),
                        child: Row(
                          children: [
                            Icon(
                              selected
                                  ? _iconsActive[i]
                                  : _icons[i],
                              size: 18,
                              color: selected
                                  ? ConnectiveTheme.accent
                                  : ConnectiveTheme.textSecondary,
                            ),
                            const SizedBox(width: 10),
                            Expanded(
                              child: Text(
                                _labels[i],
                                style: TextStyle(
                                  fontSize: 13,
                                  fontWeight: selected
                                      ? FontWeight.w600
                                      : FontWeight.w400,
                                  color: selected
                                      ? ConnectiveTheme.textPrimary
                                      : ConnectiveTheme.textSecondary,
                                ),
                              ),
                            ),
                            if (i == 2 &&
                                store.updateAvailable)
                              Container(
                                width: 7,
                                height: 7,
                                decoration: const BoxDecoration(
                                  color: ConnectiveTheme.accent,
                                  shape: BoxShape.circle,
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
          const Divider(height: 1),
          ListenableBuilder(
            listenable: store,
            builder: (context, _) {
              final state = store.connectionState;
              final connected =
                  ConnectionStates.isConnected(state);
              final Color dot;
              if (connected) {
                dot = state == ConnectionStates.degraded
                    ? ConnectiveTheme.warning
                    : ConnectiveTheme.success;
              } else if (ConnectionStates.isBusy(state)) {
                dot = ConnectiveTheme.info;
              } else if (state == ConnectionStates.error) {
                dot = ConnectiveTheme.danger;
              } else {
                dot = ConnectiveTheme.textMuted;
              }
              return Padding(
                padding: const EdgeInsets.symmetric(
                    horizontal: 14, vertical: 10),
                child: Row(
                  children: [
                    Container(
                        width: 8,
                        height: 8,
                        decoration: BoxDecoration(
                            color: dot,
                            shape: BoxShape.circle)),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        ConnectionStates.label(state),
                        style: const TextStyle(
                            fontSize: 12,
                            color: ConnectiveTheme.textSecondary),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (!store.backendAlive)
                      const Icon(Icons.cloud_off,
                          size: 14,
                          color: ConnectiveTheme.warning),
                  ],
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
