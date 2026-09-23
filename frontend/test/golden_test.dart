import 'dart:io' show Platform;

import 'package:connective/main.dart';
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'lab.dart';

/// Headless visual goldens: real widgets + real backend state, rendered
/// with test fonts. Review with: open build/.../goldens/*.png
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
    await store.addSubscription(
        'Germany Servers', 'http://127.0.0.1:${lab.subPort}/sub');
    await store.updateSubscriptions();
    await store.testServers();
    for (var i = 0;
        i < 40 && store.testingServers.isNotEmpty;
        i++) {
      await Future.delayed(const Duration(milliseconds: 250));
    }
  });

  tearDown(() async {
    store.dispose();
    await lab.stop();
  });

  Future<void> settle(WidgetTester t) async {
    for (var i = 0; i < 6; i++) {
      await t.pump(const Duration(milliseconds: 200));
    }
  }

  testWidgets('golden dashboard disconnected', (t) async {
    // Hermetic layout golden: the foreign-TUN banner reflects live host
    // state (another VPN up on the build machine must not change a
    // layout golden). Banner logic itself is covered by the dashboard
    // conditional + backend OtherTunInterfaces unit tests.
    store.foreignTun = [];
    await t.pumpWidget(SizedBox(
        width: 1280,
        height: 800,
        child: ConnectiveApp(store: store)));
    await settle(t);
    await expectLater(
        find.byType(ConnectiveApp),
        matchesGoldenFile(
            'goldens/dashboard-disconnected.png'));
  }, skip: Platform.isWindows); // Linux-rendered pixel references; see docs/WINDOWS.md

  testWidgets('golden servers expanded', (t) async {
    await t.pumpWidget(SizedBox(
        width: 1280,
        height: 800,
        child: ConnectiveApp(store: store)));
    await settle(t);
    await t.drag(find.byKey(const Key('dash-scroll')),
        const Offset(0, -400));
    await settle(t);
    final id = store.subscriptions.first.id;
    if (!store.isExpandedSub(id)) {
      await t.tap(find.text('Germany Servers'));
      await settle(t);
    }
    final srv = store.servers.first;
    if (!store.isExpandedServer(srv.id)) {
      await t.tap(find.text(srv.displayName));
      await settle(t);
    }
    await expectLater(
        find.byType(ConnectiveApp),
        matchesGoldenFile('goldens/servers-expanded.png'));
  }, skip: Platform.isWindows); // Linux-rendered pixel references; see docs/WINDOWS.md

  testWidgets('golden dashboard connected', (t) async {
    await t.runAsync(() async {
      await store.toggleConnection();
      for (var i = 0;
          i < 20 && store.connectionState != 'connected';
          i++) {
        await Future.delayed(const Duration(seconds: 1));
      }
    });
    await t.pumpWidget(SizedBox(
        width: 1280,
        height: 800,
        child: ConnectiveApp(store: store)));
    await settle(t);
    await expectLater(
        find.byType(ConnectiveApp),
        matchesGoldenFile('goldens/dashboard-connected.png'));
  }, skip: Platform.isWindows); // Linux-rendered pixel references; see docs/WINDOWS.md
}
