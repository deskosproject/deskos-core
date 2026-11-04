# Runtime behaviour

What a DeskOS image does after it boots: baseline, Flatpaks, desktop, boot,
trust, updates, `/etc` drift and the provisioning boundary. The compiler is
[architecture.md](architecture.md).

## Managed baseline

DeskOS manages what an organization requires for any authorized user, before a
`$HOME` exists. Personal tooling (Homebrew, mise, SDKMAN, dotfiles, personal
Toolboxes) stays outside the model.

| Order | Source | Pin |
|---|---|---|
| 1 | distribution RPM | the repository's version |
| 2 | official vendor repository, or a vendor-signed RPM file | URL and checksum per update |
| 3 | upstream binary | exact version, https URL and SHA-256 |
| 4 | system Flatpak | application id and branch |
| 5 | managed web application | later |

`BinaryArtifact` installs into `/usr/local/bin` with mode `0755` or `0555`. The
fetch uses `curl`, which the platform's image must provide; a base that neither
ships nor declares it fails late at build.

The backend resolves symlinks when it creates the destination, because `install

-D` cannot traverse a `/usr/local` that is a dangling symlink during the build.

A single-file artifact may declare a `desktopEntry`. The lowerer emits the entry,
the icon and an icon-cache update; `Exec` and `TryExec` are the artifact's
destination, never authored text.

`PackageSet.rpmFiles` needs the same URL and checksum plus an armoured public key
inside the root. The build fails unless the RPM is signed by that key.

## System Flatpaks

An image that declares a system Flatpak channel installs Flatpak and adds no
remote. Remotes go to `/usr/share/flatpak/remotes.d`; applications to
`/usr/share/flatpak/preinstall.d`.

A generated unit runs `flatpak preinstall` once per boot; upstream decides what
to install and does not reinstall an app a user removed.

| Property | Value | Why |
|---|---|---|
| unit ordering | after `network-online.target` and `multi-user.target` | no boot target gains a network wait |
| before each attempt | `flatpak repair`, then `flatpak update --appstream` | an interrupted pull or a stale summary otherwise blocks |
| retry | `Restart=on-failure`, `RestartSec=60`, no start limit | a slow link is not a failed unit |
| stop | within 20 s | a boot is not held |

A remote is OSTree by default. `transport: oci` carries `Url=oci+https://…` and
no signing key, because the registry and its images are what get verified. An
OCI remote with an authenticator gets `deskos-flatpak-oci-
authenticator.service`, ordered before preinstall.

Credentials are not in the artifact: the authenticator reads them from the
machine.

Offline boot, in a container with no network and a 25-second simulated
`nm-online` timeout: GDM was active while `network-online.target` was pending, and
`flatpak preinstall` printed "Nothing to do." Installing after an offline boot is
unverified.

## GNOME

`GnomeProfile` exposes administrator vocabulary (`windows.buttons`,
`shell.favorites`, `session.idle.blankAfter`, `dock.position`). The lowerer writes
dconf keys to the platform's vendor database (`distro` on EL10). Wallpaper files
are repository assets under `/usr/share/deskos/backgrounds/`.

| Area | Keys | Rule |
|---|---|---|
| Dock | `shell.extensions`, `dock.*` | dock settings apply only while the extension is enabled; a higher layer disabling it masks them with a warning, and dock settings without a dock are errors |
| Appearance | `appearance.colorScheme`, `accentColor` | the enums of `gsettings-desktop-schemas` 47.1; Core sets neither |
| Terminal shortcut | `keyboard.terminal` | a `.desktop` id bound to Ctrl+Alt+T; EL10 has no native key |
| Login screen | `appearance.loginLogo` | GDM reads `distro`, which Platforms declare with `gnome.loginScreen` |
| First boot | GLib vendor override | Initial Setup has its own profile and does not read `distro` |

`software.updates` maps to GNOME's update settings:

| Value | allow | download | Effect |
|---|---|---|---|
| `automatic` | true | true | downloads and applies what needs no reboot |
| `manual` | true | false | offers updates; the user applies them |
| `disabled` | false | false | no Updates page; another updater owns updates |

`deskos-gnome` sets `manual`, unlocked.

EL10 reads `user`, `local`, `site`, `distro`, highest first:

| DeskOS writes | Effect |
|---|---|
| a default | `local`, `site` or a user change overrides it |
| a lock | the `distro` value applies and is not writable; `local` and `site` locks are ignored |

> **Warning:**
> A lock is image-level enforcement that defeats `site` and `local`. Core uses
> none. Lock only what the image must enforce.
Not modeled: the Initial Setup third-party repositories page, which needs
`fedora-third-party`, absent on EL10.

## Boot

`BootProfile` (`splash: graphical|text`, `quiet`, `watermark`) lowers to bootc
kernel arguments in `/usr/lib/bootc/kargs.d/50-deskos.toml`. For `graphical` it
also adds the platform's Plymouth packages, a dracut drop-in and an initramfs
rebuild for every kernel.

The build fails if no kernel is found or Plymouth is missing from an initramfs.
`text` adds no splash argument and no rebuild.

`watermark` (PNG, at most 1024x1024, inside the root) needs `splash: graphical`
and becomes an image-owned two-step theme `deskos`. A higher-layer `text` masks a
watermark with a warning; a watermark at or above a `text` layer is an error.

Diagnostics stay available: ESC shows details, systemd prints failures under
`quiet`, and dropping `rhgb quiet` gives full text. A serial console turns
Plymouth to text, so test QCOW2s use `--no-default-kernel-args`.

## Trust store

`TrustAnchor` places CA certificates in the platform trust store: `anchors[]` in
the anchors directory, `blocklist[]` where p11-kit reads it as distrusted. It
lowers to one file per certificate and one `trust.updateCommand` run after `COPY
rootfs/`.

On EL10 that is `update-ca-trust`, which regenerates `/etc/pki/ca-
trust/extracted/`.

Certificates are keyed by `name`: identical definitions deduplicate, different
ones conflict, and anchoring plus distrusting one name fails. The file must be a
PEM `CERTIFICATE`; a private key is refused. A platform without `trust` facts
fails composition, and a `blocklist` without a `blocklistDir` fails too.

Core ships `deskos-trust` for the October 2026 eMudhra roots (RHSB-2026-012).
Organizations add their own at any layer.

## Updates

Every shipped Platform lists `bootc-fetch-apply-updates.timer` as an image-update
unit. Where the base enables it, its service runs `bootc upgrade --apply`, which
reboots on publish. That path was checked on CentOS Stream 10.

`UpdatePolicy` has an `image` and a `flatpak` area, each with `automatic`,
`schedule: daily|weekly` and `requireACPower`; `image` also has an apply window.
It masks the platform's `updates.imageUpdateUnits` for `image.automatic` and
generates:

| Unit | Command |
|---|---|
| `deskos-image-update.service` | `/usr/bin/bootc upgrade --quiet` |
| `deskos-flatpak-update.service` | `/usr/bin/flatpak update --system --noninteractive --assumeyes` |
| `deskos-image-apply.service` (opt-in) | `deskos window` guard, then `bootc upgrade --apply --quiet` |

`bootc upgrade` without `--apply` downloads and stages; `ostree-finalize-staged`
applies at the next shutdown or reboot. No generated unit reboots unless an
organization sets `image.apply.enabled: true`; a test rejects `--apply`,
`--soft-reboot` and `--download-only` in every rendered Core and example image.

Timers use `OnCalendar=daily|weekly`, `RandomizedDelaySec=2h` and
`Persistent=true`. `requireACPower` adds `ConditionACPower=true`: on battery the
run is skipped, not failed. Services follow `network-online.target`; timers add no
boot ordering.

An apply policy ([ADR 0023](adr/0023-scheduled-apply-window.md)) adds `deskos-
image-apply.timer`, with `days` plus `at` or `window.start` and `duration`, an
optional IANA `timezone`, `AccuracySec=1min` and `Persistent=false`. The service
runs `deskos window` as an `ExecCondition`, so a window that elapsed during
suspend is skipped.

Exact `at` allows five minutes. A missed window is never caught up.

Core sets image updates daily on AC power; system Flatpak updates are the EL
family's policy. `automatic: false` keeps the platform updater masked and
schedules nothing. Masks live in `/etc`, so a local unmask survives updates.

A staging run skipped on battery is not lost: a generated udev rule starts the
same service when AC comes online.

Not supported: metered networks. Not checked: RHEL 10, battery. The operator view
is [operations.md](operations.md).

## Drift and mutable `/etc`

bootc updates `/usr` atomically. `/etc` persists and follows image updates only
while locally unmodified. `deskos check` reports the drift it can measure and
publishes what it leaves out. DeskOS does not enforce or reconcile the machine
against its plan (ADR 0018).

## Provisioning and enrollment

| Stage | Contents |
|---|---|
| Artifact | everything identical across machines: packages, branding, client binaries, descriptors |
| Provisioning | per-machine install choices, local bootstrap account, storage and encryption, first-boot Flatpak materialization |
| Enrollment | FreeIPA or AD join, device certificates, VPN identities, EDR tenancy |

One digest serves many machines, so no identity or secret is ever in the
artifact. Enrollment is reserved in the Plan and not implemented.
