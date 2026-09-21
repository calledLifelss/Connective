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

## Bundled flag assets (connective UI)

- **flag-icons 4×3 artwork** (`assets/flags/*.png`, 257 ISO-3166-1
  alpha-2 files) — **MIT License**, Copyright © 2013 Panayiotis
  Lipiridis. Upstream SVGs rasterized locally (66×48 PNG) and bundled
  so country flags render identically on Linux and Windows without OS
  emoji or network loads.
  Upstream: https://github.com/lipis/flag-icons
