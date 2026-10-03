# Visual identity, boot splash and VM QA

Status: slices 1 (first-boot wallpaper), 2 (GDM login logo), 3
(BootProfile kind, ADR 0006), 4 (stock quiet graphical boot) and 5
(DeskOS splash watermark) are implemented and seen in CS10 Core VMs.
Slice 6, the session desktop logo and the remaining VM QA tiers are
proposals. Evidence was gathered on 2026-09-28 from the built CentOS
reference image (`localhost/deskos-core-centos10:freeze`), upstream
sources and one disposable experiment, then on booted CS10 Core VMs; see
`docs/research-notes.md` ("Visual identity and boot"). "Verified" below
means checked in the image or in primary source; "seen" means observed
on a booted screen. Nothing here is verified on RHEL with the current
Core.

## Goal

A DeskOS workstation should look like the organization's workstation from
power-on to desktop: boot splash, first-boot setup (GNOME Initial Setup),
login screen (GDM) and session. DeskOS Core supplies neutral defaults; an
organization overrides them with its own assets through the usual layers.
Distribution identity stays truthful: os-release, legal notices and the
distribution's logo packages are never replaced or falsified.

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
| Rollback | initramfs and kargs.d are image content; each deployment has its own boot entry | inferred, not tested |
| Dock | `gnome-shell-extension-dash-to-dock` is an optional package of the `gnome-desktop` comps group (not installed by it); DeskOS installs and enables it only when the effective `dock.enabled` is true | verified in the CS10 group and images; active in the CS10 instrumented session check |

## Decisions

1. **GIS wallpaper follows the effective wallpaper.** Lower
   `appearance.wallpaper` a second time into a GSettings vendor override
   (`/usr/share/glib-2.0/schemas/50_deskos.gschema.override`) and compile
   schemas in the build. No API change; the organization's wallpaper
   (highest layer) reaches first boot. No RPM is needed: the file is
   image-owned in `/usr`, does not replace `centos-logos` or
   `redhat-logos`, and ordering is documented GLib behavior (higher `nn_`
   wins). A branding RPM is only worth it if assets must be shared outside
   DeskOS images; not now.
2. **One key per surface.** `appearance.loginLogo` (PNG or SVG asset)
   targets GDM only, lowered to `/org/gnome/login-screen/logo`; it fails
   on a platform whose Platform fact `gnome.loginScreen` is not true. A
   session logo (background-logo extension) would be a separate, future
   `appearance.desktopLogo`, so one requested value never silently
   reaches only one of two surfaces.
3. **GIS welcome: documented limitation.** DeskOS builds artifacts on the
   distribution; it is not a new OS identity. The welcome image stays the
   distribution's compiled artwork and os-release stays as the base image
   ships it on every platform, so GIS says "Welcome to CentOS Stream 10"
   and GRUB lists the distribution's name.
4. **Dock stays semantic; Core enables it.** Keep `dock.*` in GnomeProfile
   as intent; Platform maps it to an extension. Unset or `false` means the
   extension is neither installed nor enabled (the package is optional in
   the EL10 GNOME group). Core sets only `dock.enabled: true`, keeping the
   extension's own defaults; an organization profile can set options or
   `dock.enabled: false`. *Implemented* (with `dock.showTrash`):
   - dock options (`position`, `behavior`, `iconSize`, `showTrash`) need
     an effective `dock.enabled: true`; with `enabled` unset they fail;
   - `enabled: false` at a layer masks options that won at lower layers
     (no extension keys, no package, a plan warning per masked option);
   - options at the same or a higher layer than an effective
     `enabled: false` fail, naming both sides;
   - `enabled: true` on a platform without the extension fails;
   - `enabled: false` writes `enabled-extensions` without the dock only if
     the platform's default list contains it or the setting is locked;
   - `deskosctl validate` prints plan warnings (including masks) to
     stderr and still exits 0.
   `shell.favorites` is native GNOME Shell state, ordered, and independent
   of the dock.
5. **Boot is not GNOME.** Plymouth and kernel arguments are independent of
   the desktop, so they do not belong in GnomeProfile, and Platform only
   holds facts. Boot intent is the tenth kind,
   `system.deskos.org/v1alpha1 BootProfile` (ADR 0006):

       spec:
         splash: graphical | text     # graphical: rhgb + Plymouth in initramfs
         quiet: true | false          # quiet
         watermark: ./example-mark.png # optional; two-step watermark (PNG)

   Scalars with the usual layering. Raw kernel arguments are not exposed.
   The platform supplies the karg names, dracut module and theme base.

## Typed IR (additions)

- `GSettingsVendorDefault{schema, key, type, value, setting, provenance}`
  → one generated `.gschema.override`, then `glib-compile-schemas --strict`.
- `KernelArguments{args[], provenance}` → `/usr/lib/bootc/kargs.d/50-deskos.toml`.
- `InitramfsRegeneration{dracutModules[], provenance}` → drop-in
  `/usr/lib/dracut/dracut.conf.d/50-deskos.conf` and one fixed
  `dracut --force` RUN for the image kernel after all files are in place.
- `PlymouthTheme{name, dir, framesFrom, watermark, provenance}` →
  `/usr/share/plymouth/themes/deskos/deskos.plymouth` (fixed two-step
  keyfile), the watermark as a file install, the stock spinner frames
  copied in the build, and `plymouth-set-default-theme deskos`
  (`Theme=` in `/etc/plymouth/plymouthd.conf`) before the initramfs rebuild.

No user-authored Plymouth script themes, raw dconf or shell.

## Boot safety

- Passphrase prompts: two-step renders ask-password dialogs; without
  Plymouth in the initramfs, systemd's console prompt is used. Both must
  be exercised (LUKS VM) before enabling by default.
- Diagnostics stay reachable: ESC shows details; with `quiet` systemd
  still prints failures (`show_status=error`); editing the boot entry to
  remove `rhgb quiet` or adding `plymouth.enable=0` gives full text.
  Document these for operators.
- Not everything disappears: firmware, bootloader and early kernel
  messages before Plymouth can still show.
- Accessibility: text mode stays available (`splash: text`); verify
  Plymouth's keyboard/braille behavior before a default change.

## Example organization assets

`examples/example-org/assets/` holds placeholder artwork drawn in this
repository. A real organization supplies its own marks with owner, source
file and permission recorded in its `PROVENANCE.md`. Its splash is the
stock two-step spinner with its own watermark; custom frames, if ever
needed, are a typed PNG frame set, never a script theme.

## Slices (each reviewable on its own)

1. **GIS/first-boot wallpaper** via vendor override (decision 1).
   *Implemented:* Plan `gsettingsVendorDefaults`, generated
   `/usr/share/glib-2.0/schemas/50_deskos.gschema.override`,
   `glib-compile-schemas --strict` before `dconf update`. Verified at the
   GSettings level (GIS profile resolves the Core or example wallpaper
   instead of `centos-day.png`) and seen on the first-boot screen of a
   Core VM.
2. **GDM login logo** (`appearance.loginLogo`). *Implemented:* asset
   installed under `/usr/share/deskos/branding/`, dconf default
   `/org/gnome/login-screen/logo` in the `distro` db, platform capability
   `gnome.loginScreen`. Core ships the DeskOS lockup and the example
   organization a placeholder wordmark (`PROVENANCE.md` next to each). Verified by GSettings under
   the `gdm` profile and seen on the login screen of a Core VM.
   A session `desktopLogo` is a later, separate slice.
3. **Kind decision** for boot intent (decision 5). *Decided:* ADR 0006.
4. **Quiet graphical boot** with the stock theme. *Implemented:*
   `BootProfile{splash, quiet}`; Core sets `graphical` + `quiet: true`;
   kargs.d `["quiet", "rhgb"]`; dracut drop-in `add_dracutmodules+=" plymouth "`;
   initramfs rebuilt per image kernel with an in-build `lsinitrd` check.
   Seen on UEFI VMs; a QCOW2 from bootc-image-builder with its default
   `console=ttyS0` shows text instead (see "Test disks" below).
5. **DeskOS/organization splash** (watermark theme). *Implemented:*
   `BootProfile.watermark`; Core ships the DeskOS lockup as a 255x48 RGBA
   PNG (`resources/assets/deskos/PROVENANCE.md`); an organization replaces it with
   its own BootProfile at a higher layer. Boot and shutdown on UEFI VMs
   show the DeskOS splash with no firmware or CentOS mark.
6. **Favorite ids and asset checks** in the built image (roadmap item).

## Test disks

bootc-image-builder adds `rw console=tty0 console=ttyS0` to QCOW2 images
(`osbuild/images` `bootc_kernel_options`). With a serial console active,
Plymouth forces details mode. `build --no-default-kernel-args` omits those
arguments; bootc still adds `rw`, and the disk boots with the image's own
`quiet rhgb`, as an installed workstation does. DeskOS test disks use it;
the artifact is unchanged.

Known remaining text on CS10: the firmware logo, the GRUB menu and its
"Booting" line, and four kernel lines "Unmaintained driver is detected:
cnic / bnx2i" at boot, visible again briefly at shutdown. The dracut
`iscsi` hook (`parse-iscsiroot.sh`, dracut-network 107-11.el10 and
dracut-ng `main`) loads those offload drivers unconditionally. Unresolved;
DeskOS does not disable hardware support to hide the warning.

## VM QA

Implemented for CS10 in `tests/vm/`: Tier A without reference matching
(`bootcheck.py`, pixel classes) and a Tier B subset (`sessioncheck.py`,
systemd credentials over SMBIOS and a serial report, no SSH or qecore);
see `docs/roadmap.md` Milestone 3 for what remains. The rest of this
section is the proposal.

Two tiers, reported separately, each result tied to the candidate image
digest, QCOW2 SHA-256, harness commit, test-suite commit and runner image
digest; finite timeouts; required checks fail closed; nothing is
published unless the required tier passes. An AI assistant may triage
failures and propose fixes; it cannot mark a failing gate as passed.

- **Tier A, unmodified artifact.** Boot the exact QCOW2 in QEMU/KVM with
  no kernel-argument or service changes. Evidence: QEMU monitor
  screendumps at fixed checkpoints (splash, GIS or GDM), boot timing, and
  the console log if the image itself enables one. Checks: reaches GIS on
  first boot within the timeout; the splash and GIS screens match approved
  references within a tolerance; shutdown and reboot complete. This is
  the only tier that proves untouched first boot.
- **Tier B, instrumented session.** A disposable derived test layer or
  first-boot test provisioning adds a test user, SSH key, autologin and
  qecore; every perturbation is listed in the report. Scenarios: GDM logo
  and session wallpaper/logo, favorites exist and launch, browser loads a
  local test page served by the harness, offline login (network detached
  via QEMU monitor), reboot and shutdown splash, dconf defaults and locks.
- **Tier C, lifecycle.** bootc upgrade to a second candidate, reboot,
  rollback; kargs and initramfs follow the deployment.

Bluefin's testsuite (`projectbluefin/testsuite` at
`66d66512e7cfc608f39a5240af97928071eac409`, 2026-09-27) shows the pattern:
QEMU on `ubuntu-latest`, serial log artifact, behave + qecore-headless.
Its `e2e.yml` boots with `selinux=0`, masks about twenty services
(including `NetworkManager-wait-online`, `firewalld`,
`flatpak-preinstall`) and pre-bakes GDM autologin, so it is Tier B only.
Proposal: a small DeskOS-owned behave suite using qecore directly, reusing
Bluefin's harness ideas (QEMU invocation, readiness polling, serial
capture), rather than adopting the Bluefin suite; qecore on EL10 is
unverified and needs a spike.
