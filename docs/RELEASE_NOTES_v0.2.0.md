# Connective v0.2.0 — first stable release

First stable release of Connective, the Flutter + Go + sing-box
desktop VPN client for Linux (Fedora / KDE Plasma / Wayland).

## What's new

- **Dashboard home** — connect control, subscriptions, and servers on
  one page with a sticky connection bar; server selection flows
  straight into Connect.
- **Bundled country flags** — consistent local flag assets across
  Dashboard, servers, and status (no OS emoji).
- **Self-update engine** — in-app updates from GitHub Releases with
  signed manifests, delta/full artifacts, staging, atomic activation,
  and rollback (Settings → UPDATES).
- **Connection core** — sing-box supervision, TUN, global/rules
  routing, proxy-aware DNS, kill switch.
- **Auto selection + failover** — hysteresis-scored server choice with
  health monitoring and automatic recovery.
- **Subscriptions & servers** — traffic/expiry metadata,
  local-preserving refresh, share-link import/export, latency testing.

## Security / reliability

- Update manifests are Ed25519-signed; every artifact is SHA-256
  verified before staging. A forged or corrupted update can never
  replace the working install.
- Privileged TUN/firewall operations stay in the polkit helper; the UI
  and daemon run unprivileged.

## Installation

```bash
sudo rpm -Uvh connective-0.2.0-2.fc44.x86_64.rpm
```

Or update from inside the app once installed (Settings → UPDATES).

## Verify

```bash
sha256sum -c SHA256SUMS
```

## Known limitations

- Linux x86_64 only; no Windows release yet.
- `/opt` layout migration to the versioned updater layout lands in a
  later release (see docs/UPDATE_ARCHITECTURE.md).
