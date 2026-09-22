import 'package:connective/components/update_dialog.dart';
import 'package:connective/models/update.dart';
import 'package:connective/state/app_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Restart dialog without a backend: pure UI contract — restarting
/// offers Restart-now, which quits the app (no lab, no daemon, so none
/// of the shared harness applies here).
void main() {
  testWidgets('restart dialog offers restart-now that quits the app',
      (tester) async {
    final s = AppStore();
    s.updateInfo = const UpdateInfo(
        version: '0.3.9',
        channel: 'stable',
        sizeBytes: 10,
        artifactType: 'full');
    s.updateState = UpdateStates.restarting;
    var quit = false;
    s.quitApp = () async {
      quit = true;
    };
    await tester.pumpWidget(MaterialApp(
        home: Scaffold(
            body: Builder(
                builder: (c) => TextButton(
                    onPressed: () => showUpdateDialog(c, s),
                    child: const Text('open'))))));
    await tester.tap(find.text('open'));
    for (var i = 0; i < 5; i++) {
      await tester.pump(const Duration(milliseconds: 200));
    }
    expect(find.byKey(const Key('update-dialog')), findsOneWidget);
    expect(find.byKey(const Key('update-dialog-restart')),
        findsOneWidget);
    await tester.tap(find.byKey(const Key('update-dialog-restart')));
    for (var i = 0; i < 5; i++) {
      await tester.pump(const Duration(milliseconds: 200));
    }
    expect(quit, isTrue);
    s.dispose();
  });
}
