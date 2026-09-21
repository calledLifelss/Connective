import 'package:connective/main.dart';
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'lab.dart';

/// Dashboard redesign coverage: the Dashboard is the primary connection
/// experience (connect + subscriptions + servers on ONE page), against
/// the REAL backend. Same harness rules as app_test.dart: boot in
/// setUp, every backend/store call in tester.runAsync, bounded pump().
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

  Future<void> flushTimers(WidgetTester tester) async {
    await tester.pump(const Duration(seconds: 30));
  }

  Future<void> seedLab(WidgetTester tester) async {
    await tester.runAsync(() => store.addSubscription(
        'Lab', 'http://127.0.0.1:${lab.subPort}/sub'));
    await tester.runAsync(() => store.updateSubscriptions());
  }

  testWidgets('dashboard loads connection, subs and servers on one page',
      (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);

    // Connection section: state + AUTO + connect control.
    expect(find.text('Disconnected'), findsWidgets);
    expect(find.textContaining('AUTO'), findsWidgets);
    expect(
        find.byKey(const Key('dash-connection-expanded')),
        findsOneWidget);
    // Add Subscription entry point lives under the connection section.
    expect(
        find.byKey(const Key('dash-add-subscription')),
        findsOneWidget);
    // Empty state guides the user.
    expect(find.textContaining('No servers yet'), findsOneWidget);

    await seedLab(tester);
    await settle(tester);

    // Subscription + servers appear inline, no navigation needed.
    expect(find.text('Lab'), findsOneWidget);
    expect(find.text('T1'), findsOneWidget);
    expect(find.text('T2'), findsOneWidget);

    // Collapse hides servers, keeps the subscription visible.
    await tester.tap(find.text('Lab'));
    await settle(tester);
    expect(find.text('T1'), findsNothing);
    expect(find.text('Lab'), findsOneWidget);
    await tester.tap(find.text('Lab'));
    await settle(tester);
    expect(find.text('T1'), findsOneWidget);

    // Server row expands inline to technical details and collapses.
    await tester.tap(find.text('T1'));
    await settle(tester);
    expect(find.text('Address'), findsWidgets);
    await tester.tap(find.text('T1'));
    await settle(tester);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));

  testWidgets('add subscription flow from the dashboard', (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);

    // The entry point opens the existing add dialog in place.
    await tester.tap(find.byKey(const Key('dash-add-subscription')));
    await settle(tester);
    expect(find.text('Add subscription'), findsOneWidget);
    await tester.enterText(
        find.widgetWithText(TextField, 'Name'), 'Lab');
    await tester.enterText(
        find.widgetWithText(
            TextField, 'Subscription URL (https://…)'),
        'http://127.0.0.1:${lab.subPort}/sub');
    await tester.tap(find.widgetWithText(FilledButton, 'Save'));
    await tester.runAsync(
        () => Future.delayed(const Duration(seconds: 1)));
    await settle(tester);

    // Still on the dashboard: fetch servers, sub appears with T1/T2.
    await tester.runAsync(() => store.updateSubscriptions());
    await settle(tester);
    expect(find.text('Lab'), findsOneWidget);
    expect(find.text('T1'), findsOneWidget);
    expect(find.text('T2'), findsOneWidget);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));

  testWidgets('server selection reflects in connection section',
      (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await seedLab(tester);
    await settle(tester);

    // Select T2 (selection only — must NOT connect by itself).
    await tester.tap(
        find.byTooltip('Select for connection').first);
    await tester.runAsync(
        () => Future.delayed(const Duration(seconds: 1)));
    await settle(tester);

    expect(store.autoMode, isFalse);
    expect(store.selectedServerId, isNotEmpty);
    final selected = store.selectedServer;
    expect(selected, isNotNull);
    // Connection section shows the SELECTED state unambiguously.
    expect(find.text('SELECTED'), findsWidgets);
    expect(find.textContaining(selected!.displayName),
        findsWidgets);
    // AUTO is gone as the mode, but the way back is obvious.
    expect(find.text('Use Auto'), findsOneWidget);
    expect(store.connectionState, 'disconnected');

    // Back to AUTO.
    await tester.tap(find.text('Use Auto'));
    await tester.runAsync(
        () => Future.delayed(const Duration(seconds: 1)));
    await settle(tester);
    expect(store.autoMode, isTrue);
    expect(find.text('SELECTED'), findsNothing);
    expect(find.textContaining('AUTO'), findsWidgets);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));

  testWidgets('sticky control stays available and connect returns to top',
      (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await seedLab(tester);
    // A long list so the page actually scrolls (compact bar only
    // matters once the expanded section can scroll away). 500 nodes:
    // the dashboard must stay as usable as the management pages.
    final sw = Stopwatch()..start();
    await tester.runAsync(() => store.addSubscription(
        'Big', 'http://127.0.0.1:${lab.subPort}/sub?n=500'));
    await tester.runAsync(() => store.updateSubscriptions());
    expect(store.servers.length, 502);
    await settle(tester);
    sw.stop();
    // ignore: avoid_print
    print('DASHBOARD 500-node render+settle: ${sw.elapsedMilliseconds}ms');

    // Scroll deep: the compact bar appears with the same action.
    await tester.drag(find.byKey(const Key('dash-scroll')),
        const Offset(0, -1500));
    await settle(tester);
    expect(
        find.byKey(const Key('dash-compact-bar')), findsOneWidget);

    // Select T2 first so Connect has a manual target.
    final t2 = store.servers.firstWhere((s) => s.name == 'T2');
    await tester.runAsync(() => store.selectServer(t2.id));
    await settle(tester);

    // Connect from the persistent control.
    await tester
        .tap(find.byKey(const Key('dash-compact-connect')));
    var connected = false;
    for (var i = 0; i < 8 && !connected; i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(seconds: 2)));
      await settle(tester);
      connected = find.text('Connected').evaluate().isNotEmpty;
    }
    expect(connected, isTrue);
    // The real backend connected to the SELECTED server, not AUTO.
    expect(store.activeServerId, t2.id);
    // Attention returned to the connection section (compact bar gone).
    expect(
        find.byKey(const Key('dash-compact-bar')), findsNothing);
    expect(
        find.byKey(const Key('dash-connection-expanded')),
        findsOneWidget);

    // Disconnect from the connection section.
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

  testWidgets('dashboard search filters, management pages intact',
      (tester) async {
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await seedLab(tester);
    await settle(tester);

    // Dashboard search narrows to matches.
    await tester.enterText(
        find.byKey(const Key('dash-search')), 'T1');
    await settle(tester);
    expect(find.text('T1'), findsWidgets);
    expect(find.text('T2'), findsNothing);
    await tester.enterText(
        find.byKey(const Key('dash-search')), '');
    await settle(tester);
    expect(find.text('T2'), findsOneWidget);

    // Servers management page still works.
    await tester.tap(find.text('Servers'));
    await settle(tester);
    expect(find.text('AUTO'), findsOneWidget);
    expect(find.text('Lab'), findsOneWidget);

    // Subscriptions management page still works.
    await tester.tap(find.text('Subscriptions'));
    await settle(tester);
    expect(find.text('Update all'), findsOneWidget);
    expect(find.text('Lab'), findsOneWidget);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));
}
