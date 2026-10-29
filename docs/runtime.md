# Runtime behaviour

What a DeskOS image does once it boots: what is baseline, how Flatpaks arrive,
the desktop, boot and trust, updates, `/etc` drift, and the provisioning
boundary. The compiler is [architecture.md](architecture.md).

## Managed baseline

DeskOS manages what an organization requires **for any authorized user**,
available before any `$HOME` exists. Personal tooling (Homebrew, mise, SDKMAN,
version managers, dotfiles, personal Toolboxes) stays outside the model.

Acquisition order: distribution RPM; official vendor repository (or a
vendor-signed RPM file, which needs a new URL and checksum per update);
checksum-pinned upstream binary; system Flatpak; managed web applications later.

`BinaryArtifact` requires an exact version, an https URL that is not a floating
location, a SHA-256, and installs into `/usr/local/bin` with mode `0755` or
`0555`. The fetch uses `curl`, which **the platform's image must provide**; a
platform that neither ships nor declares it fails late at build, because "what a
base provides" is not modeled. The backend creates the destination directory with
symlinks resolved (`readlink -m`), because `install -D` cannot traverse a
`/usr/local` that is a dangling symlink during the build (measured on the Ubuntu
base, 2026-10-18).

A single-file `BinaryArtifact` may declare a `desktopEntry`: the lowerer emits a
`DesktopEntry`, the icon `FileInstall` and an `IconCacheUpdate`, whose `Exec` and
`TryExec` are the artifact's destination, never authored text.

`PackageSet.rpmFiles` needs the same URL and checksum plus an armoured public key
inside the root; the build fails unless the RPM is signed by that key.

## System Flatpaks

An image that declares a system Flatpak channel installs Flatpak and adds no
remote. Remotes go to `/usr/share/flatpak/remotes.d` and applications to
`/usr/share/flatpak/preinstall.d`, both image-owned. A generated unit runs
`flatpak preinstall` once per boot; upstream decides what to install and does not
reinstall apps a user removed.

The unit is `WantedBy=multi-user.target`, `Type=exec`, ordered after
`network-online.target` and `multi-user.target`, so **no boot target gains a
network wait**. Before each attempt it runs `flatpak repair` (an interrupted pull
leaves a ref that preinstall will not refresh) and `flatpak update --appstream`
(a stale summary reports a declared app as not found). It retries until it
succeeds (`Restart=on-failure`, `RestartSec=60`, no start limit) and stops within
20 s.

A remote is OSTree by default; `transport: oci` carries `Url=oci+https://…` and no
signing key, because the registry and its images are what get verified. An OCI
remote with an authenticator gets `deskos-flatpak-oci-authenticator.service`,
which runs `flatpak remote-add --if-not-exists --from` then
`flatpak remote-modify --authenticator-name`, ordered before the preinstall unit.
Credentials are not in the artifact: the authenticator reads them from the
machine.

Offline boot, observed in a container with no network and a 25-second simulated
`nm-online` timeout: `gdm.service` was active while `network-online.target` was
pending; `graphical.target` still waits for the network because platform units
are ordered after `multi-user.target`, bounded by `NM_ONLINE_TIMEOUT=60`;
`flatpak preinstall` printed "Nothing to do." and recorded nothing, so the apps
stay eligible (installing after an offline boot is unverified; a download failure
was not tested). GDM was not exercised in the container.

## GNOME

`GnomeProfile` exposes administrator vocabulary (`windows.buttons`,
`shell.favorites`, `session.idle.blankAfter`, `dock.position`) and the lowerer
writes dconf keys to the platform's vendor database (`distro` on EL10). Wallpaper
files are repository assets installed under `/usr/share/deskos/backgrounds/`.

**Dock.** `shell.extensions` names the UUID and whether it runs; `dock.*`
settings apply only while that decision is true. A higher-layer `enabled: false`
masks lower-layer dock settings and warns; dock settings without an enabled dock
are errors. Favorites are native and work without the dock.

**Appearance.** `appearance.colorScheme` and `accentColor` set `color-scheme` and
`accent-color` with the enums of `gsettings-desktop-schemas` 47.1. Core sets
neither.

**GNOME Software.** `software.updates` maps to `allow-updates` and
`download-updates`:

| Value | allow | download | Effect |
|---|---|---|---|
| `automatic` | true | true | GNOME default: downloads and applies what needs no reboot |
| `manual` | true | false | offers updates; the user applies them |
| `disabled` | false | false | no Updates page; another updater owns updates |

`deskos-gnome` sets `manual` as a default, no lock.

**Terminal shortcut.** `keyboard.terminal` names a `.desktop` id bound to
Ctrl+Alt+T; EL10 has no native terminal key. **Not modeled:** the Initial Setup
third-party repositories page, which needs `fedora-third-party` (absent on EL10).

**Login screen.** `appearance.loginLogo` sets the GDM logo; GDM's profile reads
`distro`, which Platforms declare with `gnome.loginScreen`.

**First boot.** GNOME Initial Setup has its own dconf profile, which does not
read `distro`, so the effective wallpaper is also a GLib vendor override
(`/usr/share/glib-2.0/schemas/50_deskos.gschema.override`, above the
distribution's `10_`), compiled in the build. Verified by resolution under the
`gnome-initial-setup` profile and on the first-boot screen of a CS10 Core VM.

**Defaults and locks.** EL10 reads `user`, `local`, `site`, `distro`, highest
first, so a DeskOS default can be overridden and a DeskOS lock cannot:

| DeskOS writes | Effect |
|---|---|
| a **default** | `local`, `site` or a user change overrides it |
| a **lock** | the `distro` value applies and is not writable; `local` and `site` locks are ignored |

> [!WARNING]
> A lock is image-level enforcement that defeats `site` and `local`. Core uses
> none. Lock only what the image must enforce; leave Ansible- or
> Satellite-managed settings as defaults.

## Boot

`BootProfile` (`splash: graphical|text`, `quiet`, `watermark`) lowers to bootc
kernel arguments in `/usr/lib/bootc/kargs.d/50-deskos.toml`, and for `graphical`
to the platform's Plymouth packages, a dracut drop-in and a rebuild of every
image kernel's initramfs. **The build fails** if no kernel is found or Plymouth is
missing from an initramfs. `text` adds no splash argument and no rebuild.

`watermark` (PNG, at most 1024x1024, inside the root) needs `splash: graphical`
and becomes an image-owned two-step theme `deskos`: a generated `deskos.plymouth`
(stock spinner layout, own `ImageDir`), the watermark, and the platform's stock
frames except their `watermark.png`. `plymouth-set-default-theme deskos` selects
it before the rebuild. A higher-layer `text` masks a watermark with a warning; a
watermark at or above a `text` layer is an error.

Diagnostics stay available: ESC shows details, systemd prints failures under
`quiet`, and dropping `rhgb quiet` gives full text. A serial console turns
Plymouth to text, so test QCOW2s are built with `--no-default-kernel-args` (see
`tests/vm/README.md`).

## Trust store

`TrustAnchor` places CA certificates in the platform trust store: `anchors[]`
into the anchors directory, `blocklist[]` where p11-kit reads it as
**distrusted**. It lowers to one image file per certificate and one
`trust.updateCommand` run after `COPY rootfs/`. On both EL platforms that is
`update-ca-trust`, which regenerates `/etc/pki/ca-trust/extracted/`, so a chain
ending at a blocklisted CA is rejected and TLS is not weakened.

Certificates are keyed by `name`: identical definitions deduplicate, different
ones conflict at any layer, and anchoring plus distrusting one name fails. The
file must be a PEM `CERTIFICATE`; a private key is refused. A platform without
`trust` facts fails composition, as does a `blocklist` on a platform with no
`blocklistDir`: gaps are named, never dropped.

Core ships `deskos-trust` for the October 2026 eMudhra roots (RHSB-2026-012);
organizations add their own at any layer. The render rejects a certificate that
shares a path with another image file, and limits assets to the resource root.

## Updates

Every shipped Platform lists `bootc-fetch-apply-updates.timer` as an image-update
unit. Where the base enables it, its service runs `bootc upgrade --apply`, which
**reboots** on publish; that path was checked end to end on CentOS Stream 10.

`UpdatePolicy` (`image` and `flatpak`, each `automatic`, `schedule:
daily|weekly`, `requireACPower`; `image` also an `apply` window) lowers to masks
of the platform's `updates.imageUpdateUnits` for any `image.automatic`, and to
generated units:

| Unit | Command |
|---|---|
| `deskos-image-update.service` | `/usr/bin/bootc upgrade --quiet` |
| `deskos-flatpak-update.service` | `/usr/bin/flatpak update --system --noninteractive --assumeyes` |
| `deskos-image-apply.service` (opt-in) | `deskos window` guard, then `bootc upgrade --apply --quiet` |

`bootc upgrade` without `--apply` downloads and stages; `ostree-finalize-staged`
applies at the next shutdown or reboot. **No generated unit reboots unless an
organization sets `image.apply.enabled: true`**; a test rejects `--apply`,
`--soft-reboot` and `--download-only` in every rendered Core and example image.

Timers use `OnCalendar=daily|weekly`, `RandomizedDelaySec=2h` and
`Persistent=true`. `requireACPower` adds `ConditionACPower=true`: on battery the
run is skipped, not failed. Services follow `network-online.target`; timers add no
boot ordering.

An `apply` policy ([ADR 0023](adr/0023-scheduled-apply-window.md)) adds
`deskos-image-apply.timer`, with `days` plus `at` or `window.start` and
`duration`, an optional IANA `timezone`, `AccuracySec=1min` and
`Persistent=false`. The service runs `deskos window` as an `ExecCondition`, so a
calendar event that elapsed during suspend is skipped outside the local interval;
exact `at` allows five minutes. A missed window is never caught up.

Core sets image updates daily on AC power; system Flatpak updates are the EL
family's policy. `automatic: false` keeps the platform updater masked and
schedules nothing; no image intent keeps the platform's rebooting timer. Masks
live in `/etc`, so a local unmask survives updates. A staging run skipped on
battery is not lost: a generated udev rule starts the same service when AC comes
online, and `ConditionACPower` still decides, so a battery-only event does nothing.
**Not supported:** metered networks. Checked with
`systemd-analyze verify` and on a CentOS Stream 10 VM: the update staged with the
same boot ID, a reboot booted it with rollback intact, the bootc timer stayed
masked, and the Flatpak update succeeded. Not checked: RHEL 10, battery. The
operator view is [operations.md](operations.md).

## Drift and mutable `/etc`

bootc updates `/usr` atomically; **`/etc` persists** and follows image updates
only while locally unmodified. `deskos check` reports the drift it can measure and
publishes what it leaves out. DeskOS does not enforce and does not reconcile the
machine against its plan (ADR 0018).

## Provisioning and enrollment

| Stage | Contents |
|---|---|
| **Artifact** | everything identical across machines: packages, branding, client binaries, descriptors |
| **Provisioning** | per-machine install choices, local bootstrap account, storage and encryption, first-boot Flatpak materialization |
| **Enrollment** | FreeIPA or AD join, device certificates, VPN identities, EDR tenancy |

One digest serves many machines, so **no identity or secret is ever in the
artifact**. Enrollment is reserved in the Plan and not implemented.
