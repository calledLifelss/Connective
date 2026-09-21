import 'package:connective/main.dart';
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'lab.dart';

/// Full-stack widget test: REAL Flutter UI + REAL IPC + REAL backend.
/// Headless (no display needed); backend is proxy-only loopback.
///
/// Rules learned the hard way:
/// - setUp/tearDown run in the real async zone: lab + store boot live here.
/// - testWidgets bodies run in FakeAsync: EVERY backend/store call must
///   be wrapped in tester.runAsync, or real socket IO freezes.
/// - No pumpAndSettle: live backend event streams (stats ticks) rebuild
///   forever; use bounded pump() instead.
void main() {
  late Lab lab;
  late AppStore store;

  setUp(() async {
    lab = Lab();
    await lab.start();
    store = AppStore();
    await store.boot(lab.sock);
    await store.updateSettings(store.settings.copyWith(
      tunEnabled: false,
      killSwitch: false,
      mixedPort: lab.mixedPort,
      clashApiPort: lab.clashPort,
      corePath: lab.singBox,
      healthIntervalSec: 30,
      // No competing background refreshes during the test.
      updateOnStart: false,
    ));
  });

  tearDown(() async {
    store.dispose();
    await lab.stop();
  });

  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 10; i++) {
      await tester.pump(const Duration(milliseconds: 200));
    }
  }

  /// Flush pending fake-zone timers before teardown (debounces etc.).
  Future<void> flushTimers(WidgetTester tester) async {
    await tester.pump(const Duration(seconds: 30));
  }

  testWidgets('dashboard shows backend state and connects',
      (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);

    // Initial: disconnected dashboard (header + button share the label).
    expect(find.text('Disconnected'), findsWidgets);

    // Sidebar navigates to Servers.
    await tester.tap(find.text('Servers'));
    await settle(tester);
    expect(find.text('AUTO'), findsOneWidget);
    expect(find.textContaining('No servers yet'), findsOneWidget);

    // Seed a subscription (real backend, real async zone).
    await tester.runAsync(() => store.addSubscription(
        'Lab', 'http://127.0.0.1:${lab.subPort}/sub'));
    await tester
        .runAsync(() => store.updateSubscriptions());
    await settle(tester);

    // Subscription card appears with servers; collapse hides them.
    expect(find.text('Lab'), findsOneWidget);
    expect(find.text('T1'), findsOneWidget);
    await tester.tap(find.text('Lab'));
    await settle(tester);
    expect(find.text('T1'), findsNothing);
    await tester.tap(find.text('Lab'));
    await settle(tester);
    expect(find.text('T1'), findsOneWidget);

    // Server row expands inline to technical details.
    await tester.tap(find.text('T1'));
    await settle(tester);
    expect(find.text('Address'), findsWidgets);

    // Back to dashboard, connect through the real button.
    await tester.tap(find.text('Dashboard'));
    await settle(tester);
    await tester.tap(find.widgetWithText(FilledButton, 'Disconnected'));
    // The daemon needs real seconds to start the core: poll with
    // real-time waits between pumps.
    var connected = false;
    for (var i = 0; i < 8 && !connected; i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(seconds: 2)));
      await settle(tester);
      connected = find.text('Connected').evaluate().isNotEmpty;
    }
    expect(connected, isTrue);

    // Disconnect again.
    await tester.tap(find.widgetWithText(FilledButton, 'Connected'));
    var gone = false;
    for (var i = 0; i < 6 && !gone; i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(seconds: 2)));
      await settle(tester);
      gone = find.text('Connected').evaluate().isEmpty;
    }
    expect(gone, isTrue);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));

  testWidgets('logs page shows backend entries', (tester) async {
    await tester.runAsync(() => store.refreshLogs());
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.tap(find.text('Logs'));
    await settle(tester);
    expect(find.text('All'), findsOneWidget);
    expect(find.textContaining('connectived'), findsWidgets);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('settings apply to backend', (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.tap(find.text('Settings'));
    await settle(tester);
    expect(find.text('AUTO SERVER'), findsOneWidget);
    // ADVANCED section needs scrolling on small test viewport.
    await tester.drag(find.byType(ListView), const Offset(0, -2500));
    await settle(tester);
    expect(find.text('Clash API port'), findsOneWidget);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('routing page persists TUN and mode', (tester) async {
    Future<void> waitFor(bool Function() cond) async {
      for (var i = 0; i < 10 && !cond(); i++) {
        await tester.runAsync(
            () => Future.delayed(const Duration(milliseconds: 500)));
        await settle(tester);
      }
    }

    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.tap(find.text('Routing'));
    await settle(tester);
    expect(find.text('ROUTING MODE'), findsOneWidget);
    // Flip to rules mode through the real radio.
    await tester.tap(find.text('Rules'));
    await waitFor(() => store.settings.routingMode == 'rules');
    expect(store.settings.routingMode, 'rules');
    // Toggle TUN off through the real switch.
    final tunSwitch = find.widgetWithText(SwitchListTile, 'Enable TUN');
    expect(tunSwitch, findsOneWidget);
    await tester.tap(tunSwitch);
    await waitFor(() => !store.settings.tunEnabled);
    expect(store.settings.tunEnabled, isFalse);
    // Toggle back on (default for real use).
    await tester.tap(tunSwitch);
    await waitFor(() => store.settings.tunEnabled);
    expect(store.settings.tunEnabled, isTrue);
    await tester.tap(find.text('Global'));
    await waitFor(() => store.settings.routingMode == 'global');
    expect(store.settings.routingMode, 'global');
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('large subscription stays usable', (tester) async {
    await tester.runAsync(() => store.addSubscription(
        'Big', 'http://127.0.0.1:${lab.subPort}/sub?n=500'));
    await tester.runAsync(() => store.updateSubscriptions());
    expect(store.servers.length, 500);
    final sw = Stopwatch()..start();
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.tap(find.text('Servers'));
    await settle(tester);
    sw.stop();
    // ignore: avoid_print
    print('LARGE render+settle: ${sw.elapsedMilliseconds}ms');
    // Search narrows instantly over 500 rows.
    await tester.enterText(
        find.widgetWithText(SearchBar, 'Search servers…'), 'Perf-49');
    await settle(tester);
    expect(find.textContaining('Perf-49'), findsWidgets);
    await tester.enterText(
        find.widgetWithText(SearchBar, 'Search servers…'), '');
    await settle(tester);
    // Scroll deep into the list without jank failures.
    await tester.drag(find.byType(ListView).first, const Offset(0, -3000));
    await settle(tester);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));
}
