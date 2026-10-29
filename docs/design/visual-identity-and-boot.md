# Visual identity, boot splash and VM QA

**Status:** slices 1 (first-boot wallpaper), 2 (GDM login logo), 3
(`BootProfile` kind, [ADR 0006](../adr/0006-bootprofile-tenth-kind.md)),
4 (quiet graphical boot) and 5 (DeskOS splash) are **implemented** and seen in
CS10 Core VMs; slice 6 and the remaining VM QA tiers are *proposals*.

Evidence from 2026-09-28: the built CentOS reference image
(`localhost/deskos-core-centos-stream-10:freeze`), upstream sources, one
experiment and booted CS10 Core VMs
([research-notes.md](../research-notes.md#visual-identity-and-boot-2026-09-28)).

> [!NOTE]
> Verified on CS10 unless stated. On RHEL 10.2 the retired factory host booted
> `example-devops-rhel-10` through the same boot and session checks.

## Goal

A DeskOS workstation should look like the organization's from power-on to
desktop: splash, first-boot setup (GNOME Initial Setup), GDM and session. Core
supplies neutral defaults that an organization overrides, and **distribution
identity stays truthful:** os-release and logo packages are never falsified.

## Evidence matrix

| Surface | Mechanism on EL10 | Status |
|---|---|---|
| Session wallpaper | `org.gnome.desktop.background` in DeskOS `distro` dconf db | implemented; seen booted (owner screenshots) |
| First-boot (GIS) wallpaper | GIS dconf profile is `user-db:user` + `file-db:/usr/share/gnome-initial-setup/initial-setup-dconf-defaults` (only a lockdown key), so GIS falls back to the schema default, which `centos-logos` sets in `10_org.gnome.desktop.background.default.gschema.override` (`centos-day.png`). A `50_` vendor override compiled with `glib-compile-schemas --strict` changes what the GIS profile resolves (experiment) | supported GLib mechanism; GSettings-level verified; seen in a CS10 Core VM |
| First-boot (GIS) logo | Welcome page image is a compiled resource (`resource:///org/gnome/initial-setup/initial-setup-welcome.svg`, GIS 46.7); title uses os-release `PRETTY_NAME`; `vendor.conf` only skips pages | **no supported hook**; not changeable without patching GIS or falsifying os-release |
| GDM logo | GDM profile is `user, gdm, local, site, distro` + greeter file-db, so DeskOS `distro` db applies; gdm ships an override `logo='/usr/share/pixmaps/fedora-gdm-logo.png'`; a `distro` value wins (experiment) | GSettings-level verified; seen in a CS10 Core VM |
| GDM background | GNOME Shell theme, not a GSettings key | out of scope (would need a Shell theme patch) |
| Session logo | `background-logo@fedorahosted.org` enabled by default; its schema defaults point at `/usr/share/fedora-logos/*.svg`, absent on CS10, so it draws nothing today; keys `logo-file`, `logo-file-dark`, `logo-position`, `logo-size`, `logo-border`, `logo-opacity`, `logo-always-visible` | settable via `distro` db (experiment); not seen on screen |
| Boot splash theme | Plymouth 24.004.60; default theme `bgrt` (two-step module, `ImageDir` spinner, watermark = `spinner/watermark.png` owned by `centos-logos`); admin selection in `/etc/plymouth/plymouthd.conf` | verified in image |
| Splash in early boot | base initramfs has **no Plymouth** (0 entries; built before the Workstation group added plymouth); bootc dracut drop-ins do not add it | verified; requires in-image `dracut` regeneration with a drop-in under `/usr/lib/dracut/dracut.conf.d` (bootc `docs/src/initramfs.md`, Fedora bootc initramfs guide) |
| Graphical vs text splash | Plymouth shows the graphical splash only with `rhgb`, `splash`, `splash=silent` or `plymouth.graphical`; `single`, `splash=verbose` force details; ESC toggles details (`src/main.c`) | verified in source |
| Quiet boot | `/usr/lib/bootc/kargs.d/*.toml` (`kargs = [...]`); base image ships none; bootc applies kargs.d at install and applies the kargs.d diff on each update, keeping machine-local args (`bootc_kargs.rs`, `install.rs`, docs) | verified in source/docs; `quiet` makes systemd `show_status` default to `error` (systemd(1)) |
| Rollback | initramfs and kargs.d are image content; each deployment has its own boot entry | rollback and boot recovery tested by `lifecycle.yml`; that a deployment boots *its own* kargs and initramfs is not measured |
| Dock | `gnome-shell-extension-dash-to-dock` is delivered as the pinned upstream v110 artifact (GNOME Shell 45–51); DeskOS installs and enables it only when a profile selects `deskos-dock`, which the reference workstations do | verified in the CS10 group and images; active in the CS10 instrumented session check |

## Decisions

1. **GIS wallpaper follows the effective wallpaper.** Lower `appearance.wallpaper`
   into a GSettings vendor override
   (`/usr/share/glib-2.0/schemas/50_deskos.gschema.override`) compiled in the
   build, so the highest layer reaches first boot; the file replaces neither
   `centos-logos` nor `redhat-logos`, and higher `nn_` wins.
2. **One key per surface.** `appearance.loginLogo` (PNG or SVG) targets GDM only,
   lowered to `/org/gnome/login-screen/logo`; it fails where the Platform fact
   `gnome.loginScreen` is not true. A session logo is a separate
   `appearance.desktopLogo`.
3. **GIS welcome: documented limitation.** DeskOS builds artifacts on the
   distribution, not a new OS identity; the welcome image stays the compiled
   artwork and os-release as the base ships it.
4. **Dock stays semantic; the workstation enables it.** Keep `dock.*` in
   GnomeProfile and enable the dock in `shell.extensions` (ADR 0017); `false`, or
   no decision, means the extension is neither installed nor enabled.
   *Implemented:* dock options need an effective decision or they fail; `false`
   masks lower-layer settings with a plan warning; settings at or above that layer
   fail; platforms that do not model dash-to-dock fail. The session check covers
   `enabled-extensions`; `deskosctl validate` warns on `stderr`.
5. **Boot is not GNOME.** Plymouth and kernel arguments are independent of the
   desktop, so they belong in neither GnomeProfile nor Platform facts. Boot
   intent is the tenth kind, `system.deskos.org/v1alpha1 BootProfile`
   ([ADR 0006](../adr/0006-bootprofile-tenth-kind.md)):

   ```yaml
   spec:
     splash: graphical | text     # graphical: rhgb + Plymouth in initramfs
     quiet: true | false          # quiet
     watermark: ./example-mark.png # optional; two-step watermark (PNG)
   ```

   Scalars with the usual layering; raw kernel arguments are not exposed. The
   platform supplies karg names, dracut module and theme base.

## Typed IR (additions)

- `GSettingsVendorDefault{schema, key, type, value, setting, provenance}` → one
  generated `.gschema.override`, then `glib-compile-schemas --strict`.
- `KernelArguments{args[], provenance}` → `/usr/lib/bootc/kargs.d/50-deskos.toml`.
- `InitramfsRegeneration{dracutModules[], provenance}` → drop-in
  `/usr/lib/dracut/dracut.conf.d/50-deskos.conf` and one `dracut --force` RUN.
- `PlymouthTheme{name, dir, framesFrom, watermark, provenance}` →
  `/usr/share/plymouth/themes/deskos/deskos.plymouth`, the watermark file, the
  stock spinner frames, and `plymouth-set-default-theme deskos`.

**No user-authored Plymouth script themes, raw dconf or shell.**

## Boot safety

- **Passphrase prompts**: two-step renders ask-password dialogs; without
  Plymouth in the initramfs, systemd's console prompt is used. Both need a LUKS
  VM run first.
- **Diagnostics stay reachable**: ESC shows details; with `quiet` systemd still
  prints failures (`show_status=error`); removing `rhgb quiet` or adding
  `plymouth.enable=0` gives full text. Text mode stays available
  (`splash: text`); verify Plymouth's keyboard/braille behavior first.

## Example organization assets

Artwork placeholders: `examples/organization-and-role/assets/`, with a
`PROVENANCE.md` recording owner, source and permission.

## Slices (each reviewable on its own)

1. **GIS/first-boot wallpaper** via vendor override (decision 1) — *implemented*.
2. **GDM login logo** (`appearance.loginLogo`, `gnome.loginScreen`) —
   *implemented*; a session `desktopLogo` is a later slice.
3. **Kind decision** for boot intent — *decided:* ADR 0006.
4. **Quiet graphical boot** — *implemented*: `BootProfile{splash, quiet}`,
   kargs.d `["quiet", "rhgb"]`, dracut `add_dracutmodules+=" plymouth "`.
5. **DeskOS/organization splash** — *implemented* as `BootProfile.watermark`, a
   255x48 RGBA PNG (`resources/assets/deskos/PROVENANCE.md`).

## Test disks

`bootc-image-builder` adds `rw console=tty0 console=ttyS0` to QCOW2 images
(`osbuild/images` `bootc_kernel_options`), and a serial console makes Plymouth
force details mode. `build --no-default-kernel-args` omits those arguments, so
the disk boots with the image's own `quiet rhgb`; **DeskOS disks use it.**

Known remaining text on CS10: the firmware logo, the GRUB menu and its "Booting"
line, and four "Unmaintained driver is detected: cnic / bnx2i" kernel lines. The
dracut `iscsi` hook (`parse-iscsiroot.sh`, dracut-network 107-11.el10 and
dracut-ng `main`) loads those drivers unconditionally. *Unresolved*.

## VM QA

Implemented in `tests/vm/` (CS10 in CI; the AlmaLinux/Fedora examples weekly;
Ubuntu in its own repository; RHEL on the retired factory host): Tier A without
reference matching (`bootcheck.py`) and a Tier B subset (`sessioncheck.py`,
systemd credentials over SMBIOS, serial report). See
[`docs/roadmap.md`](../roadmap.md#milestone-3-vm-boot-and-desktop-tests)
Milestone 3; *the rest is the proposal.*

Tiers are reported separately, each result tied to the candidate digest, the
QCOW2 SHA-256, harness and test-suite commits and the runner image digest;
**nothing is published unless the required tier passes**.

- **Tier A, unmodified artifact.** Boot the exact QCOW2 in QEMU/KVM with no
  kernel-argument or service changes. Evidence: screendumps at fixed checkpoints
  (splash, GIS or GDM), boot timing, console log. Checks: reaches GIS on first
  boot, reboot completes, and the frames are **classified, not matched against
  references** — matching was proposed here and rejected: a reference library is a
  maintenance product, and a false failure in a weekly gate costs more than it
  proves.
- **Tier B, instrumented session.** A disposable test layer or first-boot
  provisioning adds a test user, an SSH key and autologin, all listed in the
  report. Scenarios: GDM and session branding, favorites, a local test page,
  offline login, shutdown splash, dconf defaults and locks.
- **Tier C, lifecycle.** bootc upgrade, reboot, rollback; kargs and initramfs
  follow the deployment.
