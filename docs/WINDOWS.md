# Windows support

Same Connective product, second target: Windows x64 (Windows 10 1809+
/ 11). No fork — one Flutter UI, one Go backend, one update engine.
Linux behavior is untouched; every Windows difference lives behind
`GOOS` build tags or the `platform` package.

## Architecture

```
Flutter UI (shared, incl. Dashboard + flags + update UI)
    ↓  unix socket in %LOCALAPPDATA%\Connective (Win10 1803+)
Go daemon (connectived.exe): shared state machine, selector, updater
    ↓  platform/windows + connective-helper.exe (UAC per action)
sing-box.exe + wintun.dll (bundled beside the core)
    ↓
wintun TUN / route table / adapter DNS / Windows Firewall
```

The division mirrors Linux exactly: sing-box owns the data plane
(wintun adapter, auto_route, DNS servers); Go verifies and cleans up;
privileged steps run in the helper. Generic connection logic contains
no Windows branches except path/exe names via `platform`.

## Build requirements (Windows)

- Go ≥ 1.26, Flutter stable, Visual Studio Build Tools (MSVC),
  Inno Setup 6 (CI installs it if absent), `zip` semantics via
  PowerShell `Compress-Archive`.
- Cross-checks from Linux: `GOOS=windows go build ./...`
  (full compile proof, run in CI too).

## Build process

```powershell
cd backend
go test ./...
go build -o C:\dist\connectived.exe .\cmd\connectived
go build -o C:\dist\connective-helper.exe .\cmd\connective-helper
go build -o C:\dist\connective-updater.exe .\cmd\connective-updater
# sing-box.exe (SagerNet release) + wintun.dll (wintun.net) beside them
cd ..\frontend
flutter pub get
flutter build windows --release --build-name 0.3.0-beta.1 --build-number 1
```

The Flutter bundle plus the four binaries assemble into
`stage\versions\<ver>\`, with `stage\updater\` and `stage\current.txt`
alongside; Inno Setup (`packaging/windows/connective.iss`) compiles
`Connective-<ver>-windows-x64-setup.exe`. See `.github/workflows/windows.yml`
for the exact automated sequence.

## TUN

sing-box creates the `connective0` wintun adapter from the generated
config (`interface_name` + `auto_route`, same as Linux).
`tun.WindowsManager` verifies appearance (bounded, loud on failure)
and absence on teardown; `wintun.dll` must sit next to `sing-box.exe`
or connect fails fast with an actionable error
(`platform.RequireWintun`). Stale adapters are disabled by
`helper tun-cleanup` (wintun adapters die with the core process, so
unlike Linux there is usually nothing to delete).

## Routing

No `ip route` on Windows: `route_windows.go` parses `route print -4`
and performs longest-prefix match with metric tie-break in-process,
mirroring stack selection including sing-box auto_route additions.
`VerifyTUNDefault("connective0", …)` keeps the daemon call shape —
the name resolves to its interface address first.

## DNS

`ResolverSnapshot()` hashes normalized `netsh interface ip show
dnsservers` output (same role as resolv.conf hashing on Linux), so
connect/disconnect prove the system resolver untouched. `sysproxy`
Apply/Restore programs HKCU proxy keys via `reg.exe` (per-user, no
elevation) with snapshot restore.

## Privilege / helper

The app never runs elevated (`asInvoker` manifest). Privileged steps
(TUN-dependent core launch, firewall, stale cleanup) go through
`connective-helper.exe`, elevated per-invocation by a UAC prompt via
PowerShell `Start-Process -Verb RunAs -Wait` on the exact validated
argv (never a shell string). `platform.TrustedBinary` enforces
absolute `.exe` paths outside temp/download locations. Known
simplification vs Linux: no Pdeathsig equivalent in stdlib (job
objects would need x/sys) — orphan protection is Stop +
ensureCoreGone + `stop-core` with image-name verification
(`tasklist`/`wmic`/CIM before `taskkill`), same guarantees,
documented here instead of pretended.

## Kill switch

Real egress lockdown through Windows Firewall (helper-executed):
previous policy snapshotted to `--state`, owned `Connective *` allow
rules (loopback, RFC1918, DHCP, VPN endpoints TCP+UDP), default
flipped to `blockinbound,blockoutbound`, exact restore on off.
Cleanup deletes by owned prefix only — foreign rules are never
touched. A future WFP-callout refinement is possible; netsh is the
honest stdlib-only mechanism today.

## Foreign VPN safety

`OtherTunInterfaces()` enumerates `netsh interface show interface`
(Connected + tunnel-like names, own device excluded, Hyper-V/WSL/
container noise excluded) with a wintun-binding second layer when the
tool is present. Active foreign tunnels fail the connect before core
startup with the same user-facing error contract as Linux.

## Installed layout

```
%ProgramFiles%\Connective\
  versions\0.3.0-beta.1\connective.exe + data\ + *.dll
  versions\0.3.0-beta.1\connectived.exe, connective-helper.exe,
    sing-box.exe, wintun.dll
  updater\connective-updater.exe
  current.txt                     ("0.3.0-beta.1", atomically replaced)
%LOCALAPPDATA%\Connective\        user data (never touched by install)
```

Activation rewrites `current.txt` (atomic file replace); a future
launcher stub will resolve it (shortcuts currently target the
versioned exe directly). The updater runs elevated (Program Files
writes), so symlink-based activation remains valid there too.

## Updater

Same engine: per-platform manifest lookup
(`update-manifest-windows.json`, falling back to
`update-manifest.json`), signed + hashed artifacts, delta/full
selection, staging, health-checked activation, rollback. First
Windows prerelease ships full-only (no same-platform base exists
yet); deltas follow automatically once a base release exists.

## Troubleshooting

- `TUN needs "...wintun.dll"`: reinstall (the installer bundles it).
- UAC prompt declined: connect aborts safely, nothing half-applied.
- `Another VPN/TUN interface is active`: disconnect the other VPN or
  use proxy-only mode (TUN off), same as Linux.
- Backend log: `%LOCALAPPDATA%\Connective\` + in-app Logs page.

## CI testing notes

Pixel goldens (`golden_test.dart`) are Linux-rendered references and
skip on Windows (font rasterization differs per OS); layout coverage
on Windows comes from the widget tests themselves, which run fully.

## Limitations (honest)

- Bare-metal validation (real adapter traffic, sleep/resume,
  multi-VPN coexistence beyond detection) needs a physical Windows
  host; CI covers build, unit/widget tests, installer compile, and an
  install/launch/uninstall smoke run. See the prerelease notes.
- DACL-granular binary trust and WFP callouts are future refinements;
  location + identity policy + UAC scoping is what ships.
- ARM64: architecture is plumbed (`x86_64`/`aarch64` in provider and
  manifest) but no ARM64 CI build yet.
