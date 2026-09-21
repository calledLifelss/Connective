# Third-party notices

Reference projects were inspected for behavior only. **No third-party
source code is included in this repository.** All Connective code is
original. The projects below constrain what may be reused in the future
and are credited here as required for study-derived work.

## Studied (behavior only, nothing copied)

- **Hiddify App 4.1.1** — `~/Desktop/Sources/hiddify-app-4.1.1`
  License: **Hiddify Extended GPLv3** (GPLv3 §7 additional conditions:
  fork-of-upstream publishing, attribution, naming/UI restrictions,
  non-commercial-only without consent, share-alike). Because of these
  conditions, Hiddify code must NOT be copied into Connective; only
  independently implemented, behavior-compatible code is used.
  Upstream: https://github.com/hiddify/hiddify-app
- **v2rayN 7.24.9** — `~/Desktop/Sources/v2rayN-7.24.9`
  License: **GNU GPLv3** (Copyright © 2019–Present 2dust). Studied for
  behavior; nothing copied (verbatim reuse would impose GPLv3 on the
  combined work and is avoided by independent implementation).
  Upstream: https://github.com/2dust/v2rayN

## Runtime dependencies (external, fetched at install — not vendored)

- **sing-box** (Sagernet) — GPLv3. Connective drives it as an external
  `sing-box run -c` child process. Packaging must ship its license notice
  and source offer alongside the binary (phase 2 packaging task).
  Upstream: https://github.com/SagerNet/sing-box
- **Flutter SDK / Dart** — BSD-3-Clause (Google). UI framework only.
- **Go toolchain** — BSD-3-Clause (golang.org). Build tool only.

## Phase-1 dependency footprint

The Go backend is **stdlib-only** (`go.mod` has zero requirements), so a
Phase-1 build introduces no transitive license obligations. Flutter
`pubspec` dependencies (phase 2, resolved at build time) will be recorded
here with their licenses when locked.

## Flutter dependencies (locked, Phase 3–5)

Direct (all used — tray, window management, login startup,
notifications; verified by import audit):

- `tray_manager 0.7.0` — MIT (leanflutter). Used via the 0.5.x legacy
  bridge (`package:tray_manager/legacy.dart`) pending native-API
  migration; deprecation notices accepted.
- `window_manager 0.5.2` — MIT (leanflutter). Window title, min size,
  minimize-to-tray, close behavior.
- `launch_at_startup 0.5.1` — MIT. Optional login autostart toggle.
- `flutter_local_notifications 22.3.1` — BSD-3-Clause (Michael Bui).
  Desktop notifications for connect/failover/update events only.

Transitives come from pub.dev under their own OSI-approved licenses
(BSD/MIT/Apache-2.0 family; no copyleft in the Flutter tree).

## Bundled flag assets (connective UI)

- **flag-icons 4×3 artwork** (`frontend/assets/flags/*.png`, 257
  ISO-3166-1 alpha-2 files) — **MIT License**, Copyright © 2013
  Panayiotis Lipiridis. The upstream `flags/4x3/[a-z][a-z].svg` files
  were rasterized locally (librsvg via ImageMagick, 66×48, transparent
  background preserved) and bundled as PNGs so server country flags
  render identically on Linux/KDE/Wayland and Windows without OS emoji,
  network loads, or extra Flutter dependencies. No artwork was modified
  beyond rasterization.
  Upstream: https://github.com/lipis/flag-icons
