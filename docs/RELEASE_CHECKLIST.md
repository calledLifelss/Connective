# RELEASE CHECKLIST — verified 2026-09-21 on Fedora 44 / KDE / Wayland

- [x] `cd backend && go test ./...` green (all pkgs, incl. new tests)
- [x] `go vet ./...` + `gofmt -l .` clean
- [x] `flutter analyze` clean (only pre-existing tray infos)
- [x] `flutter test` green (17/17; 1 env-caused golden fixed hermetically)
- [x] `flutter test integration_test -d linux` PASS (real window)
- [x] `tests/e2e-30s.sh` PASS (traffic=200)
- [x] `tests/e2e-failover-90s.sh` PASS (verified switch, traffic=200)
- [x] Foreign-TUN live: foreign `singbox_tun` up → TUN connect blocked
      with exact safe message, foreign untouched, no route/fw change;
      proxy-only coexistence works; host intact
- [x] Elevation-failure path: 2s friendly error, no hang (< 20s guard),
      machine safe, retry identical, no orphans/stale TUN
- [ ] Interactive polkit APPROVE via real dialog (human + root needed)
- [ ] Interactive polkit CANCEL/REJECT via real dialog (same)
- [ ] Live TUN-takeover + kill-switch enforcement vs real VPN
      (deferred: user's live VPN must not be disrupted)
- [x] Auto/failover via e2e + GUI drive test (selection, health,
      switch, traffic restore, correct display)
- [x] Subscriptions + server CRUD/expand/collapse/persistence via
      automated suites; failed-update preservation covered
- [x] No fake values (grep audit)
- [x] RPM rebuilt (`packaging/connective-0.2.0-1.fc44.x86_64.rpm`);
      payload verified (layout, perms, icons, desktop entry,
      sing-box 1.14.1, new safety strings present)
- [ ] System `dnf install → launch → connect → disconnect → remove →
      reinstall` (needs interactive root; spec unchanged since the
      PHASE5-verified install)
- [x] Host cleanliness after every suite (no orphans/routes/TUN/
      sockets; Internet 200)
- [ ] Live elevated connect (approve) → disconnect (approve) with
      the fixed RPM: completes in seconds, no stuck Disconnecting,
      no surviving root core, TUN/routes/DNS restored (closes §12)
- [ ] Manual desktop pass: tray minimize/restore, autostart,
      notification bubbles (known gap: desktop notify API unwired)

## 2026-09-26 — v6.0 release pass

- [x] `go test ./...`, `go vet ./...`, `gofmt -l .` clean
- [x] `flutter analyze --no-fatal-infos` (22 pre-existing style infos,
      no errors or warnings)
- [x] `flutter test` 38/38 — run inside a user+network namespace
      (`unshare -Urn`, loopback up, dummy default route) because the
      host's own live Connective TUN (`connective0`) trips the
      daemon's egress check; the lab binary was rebuilt from source
      first so local runs match CI
- [x] `flutter build linux --release` locally
- [x] `release`, `windows` and `ci` green on `v6.0`: RPM, both full
      zips, `Connective-6.0-windows-x64-setup.exe` and both signed
      manifests published
- [x] Signed update fixtures re-keyed to 99.0.0 after they outran the
      app version at 6.0 (`testdata/updates/README.md` documents
      re-signing)
- [ ] Interactive polkit APPROVE/CANCEL, live TUN-takeover + kill
      switch vs a real VPN, `dnf install → replace → remove`, and the
      manual desktop pass — still deferred: the operator's live
      session must not be disrupted
