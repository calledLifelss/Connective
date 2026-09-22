import 'dart:io';

import 'package:connective/main.dart';
import 'package:connective/pages/settings_page.dart';
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'lab.dart';

/// Update-system widget tests: REAL Flutter UI + REAL IPC + REAL Go
/// UpdateManager, fed by deterministic signed local fixtures (never
/// production data, never network). The daemon runs with
/// CONNECTIVE_UPDATE_PROVIDER=dir:<fixture> and TEST_APPLY=1 so the
/// install path is exercised against a temp root — never the live app.
///
/// Same harness rules as app_test.dart: boot in setUp, backend calls in
/// tester.runAsync, bounded pump().
void main() {
  String fixture(String name) {
    // test/testdata/updates/<name> relative to the package root.
    // Fixtures carry a manifest platform, so Windows uses the -win
    // variants (same bytes, windows platform field).
    final root = Directory.current.path;
    if (Platform.isWindows) name = '$name-win';
    return '$root/test/testdata/updates/$name';
  }

  String keys() {
    final root = Directory.current.path;
    return '$root/test/testdata/updates/keys.json';
  }

  late Lab lab;
  late AppStore store;

  Future<void> boot(String fixtureName) async {
    lab = Lab();
    await lab.start(extraEnv: {
      'CONNECTIVE_UPDATE_PROVIDER': 'dir:${fixture(fixtureName)}',
      'CONNECTIVE_UPDATE_TRUSTED_KEYS': keys(),
      'CONNECTIVE_UPDATE_TEST_APPLY': '1',
    });
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
      updateAutoCheck: false,
    ));
  }

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

  Future<void> waitState(
      WidgetTester tester, bool Function() cond) async {
    for (var i = 0; i < 20 && !cond(); i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(milliseconds: 500)));
      await settle(tester);
    }
    if (!cond()) {
      // ignore: avoid_print
      print(
          'UPDATE-WAIT-TIMEOUT state=${store.updateState} err=${store.updateError} staged=${store.updateStaged}');
    }
  }

  testWidgets('no update: dashboard stays quiet', (tester) async {
    await tester.runAsync(() => boot('no-update'));
    await tester.runAsync(() => store.checkForUpdates());
    await waitState(
        tester, () => store.updateState == 'no-update');
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    expect(store.updateState, 'no-update');
    expect(find.byKey(const Key('update-banner')), findsNothing);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('banner, dialog, download, install flow', (tester) async {
    await tester.runAsync(() => boot('has-update'));
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);

    // Manual check surfaces the fixture release.
    await tester.runAsync(() => store.checkForUpdates());
    await waitState(tester, () => store.updateAvailable);
    expect(store.updateInfo?.version, '0.3.1');
    await settle(tester);

    // Dashboard banner: version + size + actions, non-dominant.
    expect(find.byKey(const Key('update-banner')), findsOneWidget);
    expect(find.textContaining('0.3.1'), findsWidgets);

    // Dialog shows details from backend state.
    await tester.tap(find.byKey(const Key('update-banner-now')));
    await settle(tester);
    expect(find.byKey(const Key('update-dialog')), findsOneWidget);
    expect(find.textContaining('Fixture feature one'), findsOneWidget);
    expect(find.textContaining('Current:'), findsOneWidget);

    // Download → verified → staged (Install replaces Download).
    // NOTE: update.* handlers run async with event-driven progress;
    // widget FakeAsync does not deliver socket events during pumps,
    // so poll the real status in the real zone after each window.
    await tester.tap(find.byKey(const Key('update-dialog-download')));
    for (var i = 0;
        i < 10 && !store.updateStaged;
        i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(seconds: 1)));
      await tester
          .runAsync(() => store.refreshUpdateStatus());
      await settle(tester);
    }
    expect(store.updateStaged, isTrue);
    await settle(tester);
    expect(find.byKey(const Key('update-dialog-install')),
        findsOneWidget);

    // Install (test-applied into a temp root) → updated.
    await tester.tap(find.byKey(const Key('update-dialog-install')));
    for (var i = 0;
        i < 15 && store.updateState != 'updated';
        i++) {
      await tester.runAsync(
          () => Future.delayed(const Duration(seconds: 1)));
      await tester
          .runAsync(() => store.refreshUpdateStatus());
      await settle(tester);
    }
    expect(store.updateState, 'updated');
    await settle(tester);
    expect(find.text('Close'), findsOneWidget);
    await tester.tap(find.byKey(const Key('update-dialog-close')));
    await settle(tester);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));

  testWidgets('bad signature fails closed', (tester) async {
    await tester.runAsync(() => boot('bad-sig'));
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.runAsync(() => store.checkForUpdates());
    await waitState(tester, () => store.updateState == 'failed');
    expect(store.updateState, 'failed');
    expect(store.updateAvailable, isFalse);
    expect(find.byKey(const Key('update-banner')), findsNothing);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('hash mismatch fails the download', (tester) async {
    await tester.runAsync(() => boot('hash-mismatch'));
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.runAsync(() => store.checkForUpdates());
    await waitState(tester, () => store.updateAvailable);
    await tester.runAsync(() => store.downloadUpdate());
    await waitState(tester, () => store.updateState == 'failed');
    expect(store.updateState, 'failed');
    expect(store.updateStaged, isFalse);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 2)));

  testWidgets('dismiss hides banner, settings exposes updates',
      (tester) async {
    await tester.runAsync(() => boot('has-update'));
    await tester.pumpWidget(ConnectiveApp(store: store));
    await settle(tester);
    await tester.runAsync(() => store.checkForUpdates());
    await waitState(tester, () => store.updateAvailable);
    await settle(tester);
    expect(find.byKey(const Key('update-banner')), findsOneWidget);

    // Later dismisses until a newer version appears.
    await tester.tap(find.text('Later'));
    await waitState(
        tester, () => store.updateState == 'idle');
    await settle(tester);
    expect(find.byKey(const Key('update-banner')), findsNothing);

    // Settings UPDATES section reflects backend state.
    await tester.tap(find.text('Settings'));
    await settle(tester);
    final list = find.descendant(
      of: find.byType(SettingsPage),
      matching: find.byType(ListView),
    );
    for (var i = 0;
        i < 6 && find.text('UPDATES').evaluate().isEmpty;
        i++) {
      await tester.drag(list, const Offset(0, -500));
      await settle(tester);
    }
    expect(find.text('UPDATES'), findsOneWidget);
    expect(find.text('Current version'), findsOneWidget);
    expect(find.text('Update channel'), findsOneWidget);
    expect(find.text('Check automatically'), findsOneWidget);
    expect(find.byKey(const Key('settings-check-updates')),
        findsOneWidget);
    await flushTimers(tester);
  }, timeout: const Timeout(Duration(minutes: 3)));
}
