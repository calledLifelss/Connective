Name:           connective
Version:        0.4.0
Release:        1%{?dist}
Summary:        Modern desktop VPN client (Flutter + Go + sing-box)
License:        Apache-2.0 AND GPL-3.0-only
URL:            https://github.com/connective/connective
Source0:        %{name}-%{version}.tar.gz

BuildArch:      x86_64
# Prebuilt binaries (flutter AOT, Go, upstream sing-box): no debuginfo.
%global debug_package %{nil}
Requires:       gtk3
Requires:       systemd-libs
# Tray support (ayatana indicator on modern Plasma/GNOME).
Requires:       libayatana-appindicator-gtk3

%description
Connective is a production-grade desktop VPN/proxy client: a Flutter UI
driving a Go backend over local IPC, supervising sing-box for proxying,
TUN, routing and DNS. Linux-first (Fedora/KDE/Wayland); the UI never
runs as root — privileged TUN/firewall operations go through a small
polkit-authorized helper.

Connective's own code is original. The bundled sing-box binary is
GPLv3 (Sagernet) with an additional naming condition; see
/usr/share/licenses/connective/ for notices and source pointers.

%prep
%autosetup -n %{name}-%{version}

%build
# Binaries arrive prebuilt (flutter build linux --release; go build).
# Nothing to compile here; layout is assembled in %%install.

%install
rm -rf %{buildroot}
# Versioned layout for the in-app updater: immutable per-version trees
# under versions/<ver>, `current` flipped atomically on update. The
# top-level names stay as symlinks so /usr/bin/connective and existing
# launchers keep working across updates.
install -d %{buildroot}/opt/connective/versions/%{version}
cp -a bundle/* %{buildroot}/opt/connective/versions/%{version}/
# Strip build-tree RPATHs baked into plugin .so files (flutter build
# artifact); the bundle resolves libs via $ORIGIN-relative layout.
for so in %{buildroot}/opt/connective/versions/%{version}/lib/*.so; do
  patchelf --remove-rpath "$so" 2>/dev/null || :
done
chmod 755 %{buildroot}/opt/connective/versions/%{version}/connective
chmod 755 %{buildroot}/opt/connective/versions/%{version}/connectived
chmod 755 %{buildroot}/opt/connective/versions/%{version}/connective-helper
chmod 755 %{buildroot}/opt/connective/versions/%{version}/connective-updater
chmod 755 %{buildroot}/opt/connective/versions/%{version}/sing-box
ln -s versions/%{version} %{buildroot}/opt/connective/current
for b in connective connectived connective-helper connective-updater sing-box; do
  ln -s current/$b %{buildroot}/opt/connective/$b
done

install -d %{buildroot}%{_bindir}
ln -s /opt/connective/connective %{buildroot}%{_bindir}/connective

install -D -m 644 packaging/connective.desktop \
  %{buildroot}%{_datadir}/applications/connective.desktop
for s in 16 32 48 64 128 256; do
  install -D -m 644 packaging/icons/connective-$s.png \
    %{buildroot}%{_datadir}/icons/hicolor/${s}x${s}/apps/connective.png
done
# Master artwork (user-approved ConnectiveIcon.jpg).
install -D -m 644 packaging/icons/connective.jpg \
  %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/connective-master.jpg

install -d %{buildroot}%{_licensedir}/%{name}
install -m 644 packaging/THIRD_PARTY_NOTICES.md \
  %{buildroot}%{_licensedir}/%{name}/
install -m 644 packaging/LICENSE.sing-box \
  %{buildroot}%{_licensedir}/%{name}/
install -m 644 packaging/LICENSE.GPLv3 \
  %{buildroot}%{_licensedir}/%{name}/

%files
/opt/connective/
/usr/bin/connective
%{_datadir}/applications/connective.desktop
%{_datadir}/icons/hicolor/*/apps/connective.*
%{_datadir}/icons/hicolor/256x256/apps/connective-master.jpg
%license %{_licensedir}/%{name}/THIRD_PARTY_NOTICES.md
%license %{_licensedir}/%{name}/LICENSE.sing-box
%license %{_licensedir}/%{name}/LICENSE.GPLv3

%post
update-desktop-database %{_datadir}/applications >/dev/null 2>&1 || :
touch --no-create %{_datadir}/icons/hicolor >/dev/null 2>&1 || :
gtk-update-icon-cache -q %{_datadir}/icons/hicolor 2>/dev/null || :

%postun
update-desktop-database %{_datadir}/applications >/dev/null 2>&1 || :
touch --no-create %{_datadir}/icons/hicolor >/dev/null 2>&1 || :
gtk-update-icon-cache -q %{_datadir}/icons/hicolor 2>/dev/null || :

%changelog
* Thu Sep 24 2026 Connective Team - 0.4.0-1
- Consolidated UI: Servers and Subscriptions tabs folded into the
  dashboard (TUN switch, Update all, Add server, Import), restrained
  charcoal + green visual language, single server summary.
* Tue Sep 22 2026 Connective Team - 0.3.1-1
- Updater that finishes: daemon hands activation to a detached
  connective-updater (pkexec/UAC), app restarts into the new build,
  result announced at next boot. RPM adopts the versioned
  /opt/connective layout (versions/<ver> + current symlink).
* Tue Sep 22 2026 Connective Team - 0.3.0-1
- Stable 0.3.0: circular power connect button with orbit rings,
  traffic quota bar, concurrent IPC (no false backend-death on
  connect), per-call IPC timeouts, backend socket-path overrides.
* Mon Sep 22 2026 Connective Team - 0.3.0-0.beta.1
- Windows x64 prerelease train: Windows target, installer, updater.
* Mon Sep 21 2026 Connective Team - 0.2.1-0.test.1
- Updater test train: live GitHub provider, embedded release trust.
* Mon Sep 21 2026 Connective Team - 0.2.0-2
- Dashboard home redesign: connect + subscriptions + servers on one page,
  sticky connection bar, server selection, bundled country flags.
  Backend: state.get gains additive selectedServer field.
* Sun Sep 20 2026 Connective Team - 0.2.0-1
- Phase 4 desktop release: native build, IPC backend, sing-box 1.14.1.
