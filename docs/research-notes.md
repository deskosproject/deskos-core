# Research notes

Sections are dated where they were checked; undated sections were
retrieved on 2026-09-27. "Verified" means checked against a primary source
(vendor docs, upstream source, or the actual package or image); anything
else is marked as not verified.

## Base images

| Platform | Reference | Status |
|---|---|---|
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc:stream10` | Verified: pulled; also listed at docs.fedoraproject.org/en-US/bootc/base-images |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` | Verified: RHEL 10 image mode guide; tags `10.0`, `10.1`, `10.2`, `latest` (= 10.2) listed with `skopeo` |

- RHEL redistribution, from the RHEL 10 image mode guide: "The rhel-bootc
  and user-created containers based on rhel-bootc container image are
  subject to the Red Hat Enterprise Linux user license agreement (EULA).
  You are not allowed to publicly redistribute these images."
- RHEL builds need an entitled host. On a subscribed RHEL host,
  entitlements reach the build via `/usr/share/containers/mounts.conf`.
  An unsubscribed host needs an activation key inside the build, or
  bind-mounted repositories, and both are factory concerns.
- In both base images `/opt` and `/usr/local` are plain directories, not
  symlinks into `/var` (checked in
  `centos-bootc:stream10` and `rhel-bootc:10.2`). Vendor RPMs that install
  to `/opt` (Chrome) and binaries in `/usr/local/bin` are image-owned.

## Packages (CentOS Stream 10 AppStream/BaseOS, checked with dnf in the image)

- Environment group `workstation-product-environment` ("Workstation"):
  GNOME 49.5, gdm, ptyxis, nautilus, gnome-software, dconf 0.40.0,
  gsettings-desktop-schemas 47.1, `redhat-flatpak-repo`, and several GNOME
  Shell extensions (installed, not enabled).
- CS10 and RHEL 10 share comps group ids (`workstation-product-environment`,
  `graphical-server-environment`, `virtualization-host-environment`).
  **Not verified for RHEL** from RHEL repodata; the Anaconda docs list the
  same environments.
- `virt-manager`: RHEL 10 "Considerations in adopting RHEL 10", A.3 Moved
  packages, lists it moved from `rhel9-AppStream` to `rhel10-CRB`; the RHEL
  10 Package Manifest (section 2.3) states "Packages included in the
  CodeReady Linux Builder repository are unsupported." RHEL 9.0 release
  notes deprecated it in favor of the web console. On CS10 `virt-manager`
  5.1.0 is only in the disabled `crb` repository (`virt-manager-common` is
  in AppStream). EPEL 10.2 does not carry it. Flathub has
  `org.virt_manager.virt-manager` (verified by the project website).
- Not available on EL10 AppStream/BaseOS: `virt-manager`, `gnome-boxes`,
  `gnome-shell-extension-appindicator`, `gnome-terminal`. Available:
  `virt-install`, `virt-viewer`, `libvirt`, `qemu-kvm`, `cockpit`,
  `cockpit-machines`, `toolbox`, `podman`.
- `gnome-shell-extension-dash-to-dock` 102 is in CS10 AppStream
  (shell-version 45 to 49). Its presence in RHEL 10 AppStream is **not
  verified**; it is inferred from CS10.
- `firewalld` is not in the bootc base image.
- Desktop file ids on EL10: `firefox.desktop` (the upstream GNOME default
  `org.mozilla.Firefox.desktop` does not exist), `org.gnome.Nautilus.desktop`,
  `org.gnome.Ptyxis.desktop`, `org.gnome.TextEditor.desktop`,
  `org.gnome.Settings.desktop`, `org.gnome.Software.desktop`.
- `setroubleshoot` is a mandatory package of comps group
  `workstation-product` (dnf 4.20, CS10, 2026-09-29) and pulls
  `setroubleshoot-server` and `setroubleshoot-plugins`; no other package
  requires or recommends them. Image `0b238c0a` carried
  `setroubleshoot-3.3.37-3.el10`, `setroubleshoot-server-3.3.37-3.el10`
  and `setroubleshoot-plugins-3.3.15-3.el10`. With the three excluded the
  group transaction resolves (dnf warns `No match for group package
  "setroubleshoot"`) and also drops `python3-dasbus` and `xdg-utils`,
  which only setroubleshoot pulled in. The CS10 Platform excludes the
  three; audit logging and SELinux enforcement are unchanged. Core
  installs `xdg-utils` explicitly.
- `xdg-utils-1.2.0-4.el10.noarch`: CS10 `appstream` (dnf repoquery,
  2026-09-29); present in RHEL 10.2 image `81bdfeec`, vendor Red Hat,
  signed with key `199e2f91fd431d51`. The RHEL repository id is not
  verified.
- RHEL 10.2 image `81bdfeec` carries `setroubleshoot-3.3.35-4`,
  `setroubleshoot-server-3.3.35-4` and `setroubleshoot-plugins-3.3.14-11`.
  The RHEL comps membership is **not verified**; the `rhel-10` Platform
  keeps them until it is.
- RHEL 10.2: `redhat-backgrounds-100.3-2` ships
  `10_org.gnome.desktop.screensaver.default.gschema.override` with
  `picture-uri-dark`, a key `gsettings-desktop-schemas-47.1-4` does not
  define for `org.gnome.desktop.screensaver`. `glib-compile-schemas
  --strict` on the full directory fails; without `--strict` only that key
  is ignored. Hence only the DeskOS override is compiled strictly.

## GNOME administration

- GNOME admin guide (help.gnome.org/admin/system-admin-guide): system
  defaults are dconf keyfiles in `/etc/dconf/db/<db>.d/`, locks in
  `<db>.d/locks/`, compiled with `dconf update`.
- EL10 dconf ships `/etc/dconf/profile/user` as `user-db:user`,
  `system-db:local`, `system-db:site`, `system-db:distro`. DeskOS writes
  vendor defaults to `distro`, the lowest system database, leaving `local`
  and `site` to runtime configuration management. Verified on the built
  reference image: a `distro` default is overridden by a `local` value; a
  `distro` lock makes the `distro` value win over `local` (even when
  `local` has its own lock) and reports `writable: false`.
- `authselect` also writes to the `distro` database (`distro.d/20-authselect`
  and its own lock), so DeskOS files there use the distinct name
  `50-deskos`.
- EL10 defaults that DeskOS must preserve: `org.gnome.shell
  enabled-extensions` defaults to `['background-logo@fedorahosted.org']`;
  gsettings-desktop-schemas carries a RHEL override setting Red Hat Text
  fonts.
- dconf also supports `file-db:` and profiles under `/usr/share/dconf`, but
  `/etc/dconf/profile/user` wins, so using them would still require
  replacing an `/etc` file. DeskOS does not use them.

## Flatpak

- `preinstall.d`: upstream feature since Flatpak 1.17.0, first stable in
  1.18.0. Files in `/usr/share/flatpak/preinstall.d/*.preinstall`, group
  `[Flatpak Preinstall <ref>]`, keys `Install`, `Branch`, `IsRuntime`,
  `CollectionID`. `flatpak preinstall` synchronizes (installs and removes)
  to the vendor list; apps a user uninstalls are not reinstalled.
- CS10 ships flatpak 1.18.0. RHEL backported preinstall support into
  1.16.0-1 (c10s changelog, RHEL-26066); RHEL 10.2 package version is
  1.16.0-9 according to AlmaLinux mirrors (**not verified** on RHEL itself).
- The man page says "The OS runs flatpak preinstall -y (or its GUI
  equivalent) on system startup". Neither EL10 platform ships a unit for
  it, so DeskOS generates a single unit that runs that command.
- Remotes: `/usr/share/flatpak/remotes.d/*.flatpakrepo` (image-owned) and
  `/etc/flatpak/remotes.d` (wins). `DeploySideloadCollectionID` sets the
  remote collection id. Verified in a CS10 container: the rendered Flathub
  remote loads with collection `org.flathub.Stable` and `flatpak preinstall`
  resolves Bazaar and its runtime from it.
- Flathub signing key: fingerprint
  `6E5C05D979C76DAF93C081354184DD4D907A7CAE`, expires 2027-06-14. The key
  in `flathub.flatpakrepo` and `flathub.gpg` are byte-identical.
- Bazaar: Flathub id `io.github.kolunmi.Bazaar` (GPL-3.0-or-later). No RPM
  exists in EL10, EPEL10 or Fedora.

## Vendor repositories (example organization)

All repomd.xml endpoints returned HTTP 200 for EL10 x86_64.

| Software | Delivery | Source |
|---|---|---|
| Terraform | `https://rpm.releases.hashicorp.com/RHEL/$releasever/$basearch/stable`, key `https://rpm.releases.hashicorp.com/gpg` | developer.hashicorp.com/terraform/install; `RHEL/10` exists |
| VS Code | `https://packages.microsoft.com/yumrepos/vscode`, key `https://packages.microsoft.com/keys/microsoft.asc` | code.visualstudio.com/docs/setup/linux |
| Chrome | `https://dl.google.com/linux/chrome/rpm/stable/x86_64`, key `https://dl.google.com/linux/linux_signing_key.pub` | google.com/linuxrepositories. The RPM writes its own `google-chrome.repo` and `/etc/cron.daily/google-chrome` |
| kubectl | `https://pkgs.k8s.io/core:/stable:/v1.37/rpm/` | kubernetes.io install guide; stable is v1.37.1; repo is per minor |
| oc | `openshift-client-linux-amd64-rhel9-4.22.14.tar.gz` from mirror.openshift.com, sha256 `73d4204f…99f6b` | stable channel 4.22.14; `sha256sum.txt.gpg` verified with Red Hat release key 2 (`567E347AD0044ADE55BA8A5F199E2F91FD431D51`); tarball contains `oc`, `kubectl` (hard link), `README.md`. No rhel10 build exists; the rhel9 build links only against glibc 2.34 symbols. Red Hat support of the rhel9 build on RHEL 10 is **not verified** |

## systemd ordering

- EL10 `gdm.service` is ordered after getty, plymouth, rc-local and
  systemd-user-sessions only; nothing orders it after `multi-user.target`
  or the network. `graphical.target` is ordered after `multi-user.target`.
- Targets gain `After=` on units they want unless the target is already
  ordered before the unit (systemd v257, `src/core/unit.c`,
  `unit_add_default_target_dependency`, "Don't create loops"). A
  `WantedBy=multi-user.target` unit with only `After=network-online.target`
  therefore makes `multi-user.target` and `graphical.target` wait for the
  network; adding `After=multi-user.target` removes that.
- EL10 enables `NetworkManager-wait-online.service` (`nm-online -s -q`,
  `NM_ONLINE_TIMEOUT=60`), and `kdump`, `rsyslog` and `insights-client-boot`
  already order `multi-user.target` after `network-online.target`.
- `flatpak preinstall` with no network: "Nothing to do.", exit 0, no state
  written; a later run with network proposes the install again.

## Go

Tests pass unchanged on Go 1.26.8 and 1.27.1. `golang.org/x/text` v0.42.0
declares `go 1.26.0`, and Go 1.26 is the oldest supported release, so
`go.mod` requires 1.26.0.

## Visual identity and boot (2026-09-28)

Checked in `localhost/deskos-core-centos10:freeze` (gnome-initial-setup
46.7-3, gdm 47.0-32, plymouth 24.004.60-17, background-logo 49.0-1,
centos-logos 100.5-1, dracut 107-11, bootc 1.16.13, kernel
6.12.0-271.el10). RHEL images not rechecked for these items.

- dconf profiles: GIS (`/usr/share/dconf/profile/gnome-initial-setup`) is
  `user-db:user` + `file-db:/usr/share/gnome-initial-setup/initial-setup-dconf-defaults`
  (only `/org/gnome/desktop/lockdown/disable-lock-screen`); GDM
  (`/usr/share/dconf/profile/gdm`) is `user`, `gdm`, `local`, `site`,
  `distro` + `file-db:/usr/share/gdm/greeter-dconf-defaults`.
- `centos-logos` owns
  `10_org.gnome.desktop.background.default.gschema.override`
  (`centos-day.png`/`centos-night.png`), the wallpaper seen in GIS. gdm
  owns `org.gnome.login-screen.gschema.override`
  (`logo='/usr/share/pixmaps/fedora-gdm-logo.png'`). glib-compile-schemas(1):
  higher-numbered `nn_` override files take precedence.
- Experiment (disposable image, no network): a
  `50_deskos.gschema.override` for `picture-uri` plus a `distro` keyfile
  for `login-screen/logo` and background-logo `logo-file`. With
  `DCONF_PROFILE=gnome-initial-setup` the wallpaper resolved to the DeskOS
  file and the logos stayed at package defaults; with `gdm` and `user`
  all three resolved to the DeskOS values. GSettings resolution only.
- GIS 46.7 source
  (https://gitlab.gnome.org/GNOME/gnome-initial-setup/-/raw/46.7/gnome-initial-setup/pages/welcome/gis-welcome-page.ui,
  `gis-welcome-page.c`, `gnome-initial-setup.c`): welcome image is
  `resource:///org/gnome/initial-setup/initial-setup-welcome.svg`; title
  from `G_OS_INFO_KEY_PRETTY_NAME`; `vendor.conf` supports only
  `[pages] skip`, `new_user_only`, `existing_user_only`.
- background-logo schema defaults: `/usr/share/fedora-logos/fedora_lightbackground.svg`
  and `..._darkbackground.svg`; `/usr/share/fedora-logos` does not exist on CS10.
- Plymouth: `plymouthd.defaults` `Theme=bgrt`; `bgrt.plymouth` uses
  `ModuleName=two-step`, `ImageDir=/usr/share/plymouth/themes//spinner`;
  `spinner/watermark.png` is owned by `centos-logos`. Source
  https://gitlab.freedesktop.org/plymouth/plymouth/-/raw/24.004.60/src/main.c:
  default splash only with `rhgb`, `splash`, `splash=silent` or
  `plymouth.graphical`; `single`/`1`/`s`/`S`/`-S`/`splash=verbose` give
  details; ESC toggles details.
- initramfs: `/usr/lib/modules/6.12.0-271.el10.x86_64/initramfs.img`
  contains no Plymouth files (`lsinitrd`); bootc dracut drop-ins
  (`20-bootc-base`, `22-bootc-generic`, `30-bootc-standard`) do not add it;
  `45plymouth` is installed. bootc v1.16.13 `docs/src/initramfs.md`:
  initramfs is image content, regenerate in the build with a drop-in in
  `/usr/lib/dracut/dracut.conf.d` and `DRACUT_NO_XATTR=1 dracut -vf`.
- Initramfs rebuild pattern: bootc v1.16.13 `docs/src/initramfs.md` and
  https://docs.fedoraproject.org/en-US/bootc/initramfs/ (retrieved
  2026-09-28) both use a drop-in `add_dracutmodules+=" <module> "` under
  `/usr/lib/dracut/dracut.conf.d/` and
  `env DRACUT_NO_XATTR=1 dracut -vf /usr/lib/modules/$kver/initramfs.img "$kver"`,
  passing the image kernel version explicitly (both derive it with
  `kver=$(ls /usr/lib/modules)`, which assumes one kernel). The CS10
  `45plymouth` dracut module's `check()` requires `plymouthd`, `plymouth`
  and `plymouth-set-default-theme` (from `plymouth-scripts`); the stock
  `bgrt` theme comes from `plymouth-theme-spinner` and
  `plymouth-plugin-two-step` (via `plymouth-system-theme`).
- Theme selection and initramfs content (Plymouth 24.004.60):
  `plymouth-set-default-theme <name>` (plymouth-scripts) writes
  `Theme=<name>` under `[Daemon]` in `/etc/plymouth/plymouthd.conf`,
  which takes precedence over `plymouthd.defaults`;
  `scripts/plymouth-populate-initrd.in` copies the selected theme
  directory and its `ImageDir` (`inst_recur`, `cp -L`) and the
  `plymouthd.conf`. `src/plugins/splash/two-step/plugin.c` always loads
  `<ImageDir>/watermark.png` and reads `UseFirmwareBackground` per mode
  (default false); `src/libply/ply-key-file.c` skips `#` lines. The
  spinner frames are owned by `plymouth-theme-spinner`, the spinner
  watermark by `centos-logos`. A scratch build of the DeskOS theme and
  initramfs steps on `localhost/deskos-core-centos10:freeze` gave
  `Theme=deskos` in the image and the initramfs, only the `deskos`,
  `details` and `text` themes in the initramfs, and `rpm -V` changes only
  in `plymouthd.conf`.
- Kernel arguments: base image has an empty `/usr/lib/bootc/kargs.d`.
  bootc v1.16.13 `docs/src/building/kernel-arguments.md` and
  `crates/lib/src/bootc_kargs.rs` (`compute_new_kargs`): kargs.d is read at
  install (`install.rs`, `get_kargs_in_root`) and its diff applied on
  update, keeping machine-local arguments; removing image kargs locally is
  undefined behavior. systemd(1): `quiet` makes `systemd.show_status`
  default to `error`.
- CS10 `gnome-desktop` comps group: `gnome-shell-extension-background-logo`
  is mandatory, `gnome-shell-extension-dash-to-dock` is optional (not
  installed by the group); DeskOS installs it when `dock.enabled` is true
  (Core sets it). Default
  `enabled-extensions` is `['background-logo@fedorahosted.org']` and no
  override or dconf file enables dash-to-dock.
- Bluefin testsuite, commit `66d66512e7cfc608f39a5240af97928071eac409`
  (2026-09-27T18:11:40Z):
  https://github.com/projectbluefin/testsuite/blob/66d66512e7cfc608f39a5240af97928071eac409/.github/workflows/e2e.yml
  (identical to the copy reviewed), `tests/common/features/common_dconf.feature`,
  `tests/lifecycle/features/bootc.feature`. The workflow boots QEMU with
  `selinux=0`, `systemd.mask=` for about twenty services and pre-baked
  GDM autologin; GUI suites run behave with qecore-headless.

## Reference projects

- `ublue-os/bluefin` `THEPATTERN.md` (a report comparing ublue-os and
  projectbluefin): digest-pinned upstream base, build once and promote the
  same digest after E2E, cheap PR validation (about 2 minutes) before
  40-minute builds, QEMU/KVM desktop E2E on stock runners, keyless cosign,
  `bootc container lint --fatal-warnings` as the last build step.
- `mrguitar/rhel-bootc-workstation`: `FROM rhel-bootc:10.1`; groups
  "Container Management", gnome-desktop, hardware-support, multimedia,
  networkmanager-submodules, "Workstation", "Virtualization Host"; adds
  cockpit-machines/cockpit-podman; enables `cockpit.socket`,
  `podman-auto-update.timer`; ends with `bootc container lint`; ISO via
  bootc-image-builder `anaconda-iso`.

## License history

The `deskosproject` GitHub organization (2016 to 2018) holds RPM packaging
repositories under mixed licenses: GPL-2.0, GPL-3.0, NOASSERTION and
unlicensed. There is no project-wide license to inherit; this repository
is Apache-2.0.

## Kernel warnings: cnic / bnx2i at boot (tracking)

Every CS10 boot prints four kernel lines, "Warning: Unmaintained driver is
detected: cnic / cnic_init / bnx2i / bnx2i_mod_init", before the splash;
they are visible again briefly at shutdown. DeskOS leaves them in place
and does not blacklist the drivers.

- Cause: dracut's iSCSI module (`dracut-network-107-11.el10`,
  `95iscsi/parse-iscsiroot.sh`, lines 100 to 104) runs `modprobe -b -q`
  for `qla4xxx`, `cxgb3i`, `cxgb4i`, `bnx2i` and `be2iscsi` on every boot
  whose initramfs includes the module, with or without an iSCSI root.
  dracut-ng `main` (`modules.d/74iscsi/parse-iscsiroot.sh`) is the same;
  those lines come from commit `5850486fbc3e` (2024-03-31, which split an
  earlier single `modprobe --all`). PRs #1121 and #1233 (2025, iSCSI
  offload boot fixes) do not change them.
- The warning text comes from the EL10 kernel marking these drivers
  unmaintained (inferred; not checked in kernel source).
- Existing reports, searched on 2026-09-29: none about this behavior.
  - dracut-ng/dracut issues and PRs for `bnx2i` and `parse-iscsiroot`:
    only #1121, #1233 and unrelated items.
  - Red Hat Jira (`redhat.atlassian.net`): RHEL-58078 shows the lines in
    an unrelated boot log (closed, Cannot Reproduce); nothing under the
    dracut component.
- Follow-up: a report to dracut-ng, and possibly to RHEL (component
  dracut), asking that the offload modprobes run only when iSCSI root or
  firmware boot is configured. Not filed yet.

## Distribution issues seen by the session check (tracking)

Seen on kvm3, AMD Ryzen 5 3600 with host-passthrough CPU, instrumented
session boot of Core CS10 (`tests/vm/sessioncheck.py`, 2026-09-29):

- `mcelog.service` fails: "AMD Processor family 23: mcelog does not
  support this processor. Please use the edac_mce_amd module instead."
  mcelog comes from the distribution package group and does not support
  newer AMD families. Its unit normally skips it when `edac_mce_amd` is
  loaded (`ConditionPathExists=!/sys/module/edac_mce_amd/initstate`), and
  also requires `/dev/mcelog`. In this CS10 guest, `/dev/mcelog` was
  present and `edac_mce_amd` was absent, so mcelog ran and exited 1; on the
  kvm3 host, EDAC was live and the unit was skipped. Public reports:
  - [Red Hat KCS 158503](https://access.redhat.com/solutions/158503),
    "Why mcelogd fails to start", Solution Verified, updated 2026-09-17,
    lists RHEL 10, same family 23 error.
  - [Red Hat KCS 5673601](https://access.redhat.com/solutions/5673601),
    "mcelog and edac_mce_amd kernel module does not work on a virtual
    machine with AMD CPU", Solution Verified 2024-06-14; it covers RHEL 7
    and 8 virtual machines (VMware, Azure), not CS10.
  - [Fedora Bugzilla 1827890](https://bugzilla.redhat.com/show_bug.cgi?id=1827890),
    component mcelog, Fedora 32, same message, CLOSED NOTABUG; the
    discussion notes the service is skipped when EDAC is loaded.

  No public CS10-specific report was found. The project also tracks
  RHEL-3674 ("Installer adds mcelog on unsupported AMD systems", closed
  Won't Do) as an internal note; it is not verified here as public or as
  the same issue.
- SELinux AVC denials, recorded in `audit.log`. With setroubleshoot
  installed they appear as the SELinux Troubleshooter "AVC denial"
  notification on first login; the CS10 Platform excludes it (see
  Packages).
  - `bootupd_t` (`lsblk` reading `/etc/group` and `/run/mount`),
    permissive. Known: RHEL-174888 and RHEL-219156 (selinux-policy,
    Release Pending).
  - `tuned_t` (`chcon`, capability `mac_admin`), denied. No existing
    report found (Red Hat Jira, 2026-09-29).
  - None came from the test instrumentation.

### mcelog on AMD: kvm3 host vs CS10 guest (2026-09-29)

Virtualized CS10 test observation, not a bare-metal CS10 result. The guest
is Core from commit `7650192` (image `77832d5c`), booted on kvm3 with
`-cpu host`, and the session check records `platform_diagnostics`.

- Host (kvm3 bare metal, RHEL 10.2, kernel `6.12.0-211.51.1.el10_2`,
  `mcelog-202-1.el10`, AMD Ryzen 5 3600, family 23): `edac_mce_amd`
  `initstate=live`, `/dev/mcelog` present, `mcelog.service`
  `ConditionResult=no`, inactive/dead, `Result=success`. The unit's
  `ConditionPathExists=!/sys/module/edac_mce_amd/initstate` skips it.
  Kernel: `EDAC MC: Ver: 3.0.0`, `MCE: In-kernel MCE decoding enabled.`
- Guest (CS10, kernel `6.12.0-271.el10`, `mcelog-210-1.el10`, same CPU
  family 23): `edac_mce_amd` absent, `/dev/mcelog` present,
  `mcelog.service` `ConditionResult=yes`, failed, `Result=exit-code`,
  `ExecMainStatus=1` ("CPU is unsupported"). Kernel: only
  `EDAC MC: Ver: 3.0.0`.
- `mcelog.service` stays failed, and the session report shows it as
  "FAIL (known, non-blocking)" with the raw `system_failed_units`. The
  session result is pass only for this verified signature (see
  `tests/vm/README.md`); any other failed unit, or mcelog failing
  differently, is a FAIL. The desktop checks (Wayland, Dash to Dock,
  favorites, wallpaper, Firefox, user units) pass.
- Not established:
  - why the guest does not load `edac_mce_amd`;
  - a controlled comparison: host and guest differ in kernel and mcelog
    versions;
  - CS10 Core on bare-metal AMD, and any RHEL guest;
  - whether the GitHub-hosted run (AMD family 25) has the same cause: it
    predates these diagnostics.
- Local evidence, not part of the repository:
  `/tmp/deskos-mcediag-evidence-20260929/` and `/tmp/deskos-mcediag-report.md`.
