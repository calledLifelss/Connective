# Connective v6.0 — full-width dashboard, real traffic graph, updater fixes

Sixth stable release of Connective, the Flutter + Go + sing-box
desktop VPN client for Linux (Fedora / KDE Plasma / Wayland), with a
Windows x64 sidecar.

## What's new

- Dashboard rebuilt: the status hero and every panel now use the full window width instead of a centered narrow column.
- Live traffic is a real graph: rate axis, wall-clock timeline, hover readout, legend, LIVE indicator, five minutes of history.
- Server list tools: filter chips (All, Healthy, Favorites, Fastest, Recently used), sorting, favorites, latency and health badges, and a Connect button on every row.
- Auto vs manual server selection is one explicit control with a plain-language caption, and picking a row now switches the live tunnel instead of only disconnecting.
- Routing edits (TUN, DNS, kill switch, split tunneling) ask for a reconnect instead of silently staying inactive, and a kill switch conflicting with split tunneling is refused with a clear message.
- DNS mode reaches the generated config: system, proxy-aware, or custom, with per-app DNS in split "only" mode, and Global routing now carries LAN traffic through the tunnel.
- Windows updates apply again: installs under "Program Files" no longer fail silently, and a declined UAC prompt surfaces as an error instead of a spinner stuck on "restarting".
- Updates are more reliable: cancelling really aborts, large downloads are no longer cut off by a whole-body timeout, and one broken GitHub release can no longer block every update check.
- Crash safety: an interrupted session no longer leaves a blocking firewall rule or a stale TUN device behind.

<!-- manifest-notes: the block above (single-line bullets, no
     semicolons) is what release.yml feeds to `update-tool manifest
     --notes` and therefore what the in-app update dialog shows. -->

## Security / reliability

- Delta updates carry the full target file list and refuse to overlay
  a base tree of the wrong version; file modes are preserved instead
  of turning data files executable.
- Firewalls rules validate IP/port input before it reaches elevated
  nft/netsh commands, and IPv6 endpoints get allow rules so a
  kill switch can no longer block the server it is connected to.
- Applied kill switch and TUN state is persisted and reconciled at
  the next boot, so a crash cannot leave traffic silently blocked.

## Installation

```bash
sudo rpm -Uvh connective-6.0-1.fc44.x86_64.rpm
```

Or update from inside the app once installed (Settings → UPDATES).
Windows: download `Connective-6.0-setup.exe` from the release.

## Verify

The in-app updater refuses anything that is not Ed25519-signed
(`update-manifest.json.sig`, key `connective-release-1`) and checks the
SHA-256 of every artifact before it installs. By hand:

```bash
sha256sum full-6.0.zip   # must equal artifacts[0].sha256 in update-manifest.json
```

## Known limitations

- Split-tunnel app lists are capped at 200 entries.
- Routing changes take effect on the next reconnect (the dashboard
  offers the button).
