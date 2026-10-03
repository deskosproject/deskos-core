# DeskOS architecture

How DeskOS turns resources into a workstation image: the boundary it keeps,
the resource model, composition, the typed plan, the Containerfile
backend, and what has been validated on each platform.

**Contents:**
[Boundary](#boundary) ·
[Design principles](#design-principles) ·
[Resource model](#resource-model) ·
[Core, organizations and roles](#core-organizations-and-roles) ·
[Composition](#composition) ·
[Providers and the registry](#providers-and-the-registry) ·
[Plan (IR)](#plan-ir) ·
[Containerfile backend](#containerfile-backend) ·
[Public CentOS, private RHEL](#public-centos-private-rhel) ·
[Platform capability gaps](#platform-capability-gaps) ·
[Validation status](#validation-status) ·
[Supply-chain inputs](#supply-chain-inputs) ·
[Managed baseline](#managed-baseline) ·
[System Flatpaks](#system-flatpaks) ·
[GNOME](#gnome) ·
[Boot](#boot) ·
[Drift and mutable `/etc`](#drift-and-mutable-etc) ·
[Provisioning and enrollment](#provisioning-and-enrollment) ·
[Release model](#release-model)

## Boundary

DeskOS **compiles desired artifact state**. It does not continuously
reconcile deployed endpoint state.

```text
organizational intent -> resources -> validation -> composition
  -> typed Plan -> backend -> build context -> bootc OCI image
                                                 -> QCOW2, ISO
```

DeskOS is finished once the artifact exists. Keeping 4,000 running
machines in a given state is the job of configuration management
(Ansible Automation Platform, Satellite, Foreman, MDM); DeskOS builds the
*baseline* those systems operate on.

> [!NOTE]
> There is no controller, agent or reconciliation loop, and none is
> planned.

## Design principles

- **Intent is data.**
- **Compilation is deterministic.**
- **File order does not define precedence.**
- **Providers emit typed IR**, not arbitrary shell.
- **DeskOS Core** provides capabilities and defaults; **organizations**
  provide policy and content.
- **DeskOS owns the artifact**, not the running machine.
- **CI is an adapter**, not the product.
- **AI may assist development**, but it is not part of compiler semantics.
- **Compose existing Linux technologies** rather than reinventing them.

## Resource model

Every resource is a versioned envelope:

```yaml
apiVersion: <group>.deskos.org/v1alpha1
kind: <Kind>
metadata: {name: <dns-label>}
spec: {...}
```

Identity is **GVK plus name** and must be unique across all loaded roots.
Resources are *plain data*: YAML anchors, aliases, merge keys and custom
tags are rejected, and no field accepts shell, templates or hooks.

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile` |
| `system.deskos.org` | `BootProfile` |

Schemas live in `schemas/` and are embedded in `deskosctl`. Each kind is
checked **twice**: against its JSON Schema, and by strict typed decoding
with semantic validation in its provider.

Several resource roots can be combined
(`deskosctl plan ./resources ./acme ...`). An organization keeps its own
repository of resources and references DeskOS resources by name; it
**never copies** them. Examples of every kind are in
[resources.md](resources.md).

## Core, organizations and roles

- **`Platform`**: facts about an OS target: bootc base image,
  redistribution constraints, the RPM groups behind DeskOS group names,
  GNOME integration facts (dconf database, default extensions, available
  extensions), Flatpak capabilities. *All platform differences live
  here.*
- **`Profile`**: a reusable fragment of intent at a semantic layer. It
  lists the resources it includes.
- **`Workstation`**: a concrete build target, one Platform plus Profiles.

DeskOS Core (`resources/profiles/deskos-core.yaml`) is a `foundation`
profile. The same Core composes onto `centos-stream-10` and `rhel-10`; a
RHEL workstation starts from `registry.redhat.io/rhel10/rhel-bootc`,
**never** from the CentOS-based DeskOS image. A test asserts both
platforms produce the same Core content.

## Composition

Layers: `foundation` < `organization` < `role` < `workstation`.

| Class | Used for | Rule |
|---|---|---|
| **set** | RPM packages, package groups, systemd units, GNOME locks | deterministic union; duplicates collapse |
| **keyed** | RPM repositories (by repo id), binary destinations, Flatpak remotes and applications | identical definitions deduplicate; different definitions conflict *at any layer* |
| **scalar** | GNOME settings | highest layer wins; different values *at one layer* conflict |

**Profile order and file order never matter.** Every contribution carries
*provenance* (resource, including profile, layer, source file and line),
and conflict errors list every contributor.

References are transitive: a `PackageSet` names the `RpmRepository`
resources its packages need, a `FlatpakSet` names its `FlatpakRemote`.
Included resources contribute at the layer of the including profile.

## Providers and the registry

`internal/registry` maps GVK to a provider. A provider decodes and
validates one kind, may declare references, may contribute to
composition, and owns a lowerer that turns composed intent into Plan IR
using only Platform facts. **Providers have no side effects**: no
commands, no network. Asset reads are confined to the declaring resource
root.

Unknown kinds fail with
`no provider registered for <group/version>, Kind <Kind>`. The provider
contract (decode, references, contributions, IR) is data in and data out,
so a future out-of-process provider protocol can implement it.

## Plan (IR)

`internal/plan` is the typed intermediate representation, independent of
YAML and of Containerfile syntax. `deskosctl plan --format json` prints
it.

- **Artifact**: base image, labels, `RpmRepository`, `RpmGroupInstall`
  (with `excludePackages`), `RpmInstall`, `VerifiedBinaryInstall`,
  `FileInstall`, `DconfDatabase` (defaults and locks),
  `GSettingsVendorDefault`, `KernelArgument`, `InitramfsRegeneration`,
  `PlymouthTheme`, `SystemdEnable`, `DefaultTarget`.
- **Provisioning**: system Flatpak remotes and applications, materialized
  on the machine by upstream `flatpak preinstall`.
- **Enrollment**: reserved and empty.

Every item keeps its provenance. The plan is **canonical**: sorted, no
maps, no timestamps.

## Containerfile backend

`internal/backends/containerfile` reads **only the Plan**. It renders:

```text
Containerfile
plan.json                 canonical Plan
generated-manifest.json   every file with mode and sha256
repos/etc/yum.repos.d/    one file per RpmRepository
rootfs/                   image files, copied after package installation
```

The Containerfile, in order:

1. one group transaction, with the platform's `--exclude` options;
2. copies repository files and runs one package transaction;
3. one verified download per binary artifact;
4. copies `rootfs/`;
5. checks the DeskOS GSettings override with
   `glib-compile-schemas --strict` against the installed schemas, then
   compiles them without `--strict`, as the packages do;
6. runs `dconf update`, enables units and sets the default target;
7. installs the Plymouth theme and rebuilds the initramfs when boot intent
   needs them (see [Boot](#boot));
8. cleans package caches and ends with `bootc container lint`.

Every generated command comes from a typed operation; values are
validated upstream and quoted again. The plan is also installed as
`/usr/share/deskos/plan.json`, so a future endpoint tool can report what
the image was built from.

**Rendering never needs credentials or network.** Building RHEL contexts
needs an entitled build host, which is a factory input.

### Build-host subscription state

On EL10 the subscription-manager dnf plugins run inside every `dnf`
transaction of a build. On an entitled host they write `redhat.repo`
(pointing at the host's entitlement certificate serial) and state under
`/var/lib/rhsm` and `/var/log/rhsm`. So every RPM transaction the backend
emits:

- mounts those two directories as **tmpfs**;
- removes the generated `/etc/yum.repos.d/redhat.repo` **in the same
  RUN**, before the layer is committed (removing it in a later layer would
  leave it in the image history);
- fails if a `redhat.repo` already exists, and the render rejects an
  `RpmRepository` with id `redhat`, so declared content is never removed.

subscription-manager's own tmpfiles.d entries recreate `/var/lib/rhsm`
and `/var/log/rhsm` at boot.

### Lint warnings

`bootc container lint` passes with **three warnings**, all produced by
distribution package scriptlets rather than by DeskOS:

- content in `/run` (cockpit, cups);
- users without sysusers.d entries (libstoragemgmt, wsdd; on RHEL also
  avahi and libvirtdbus);
- `/var` content without tmpfiles.d entries.

Seen on CS10 Core builds `50f8892a` and `77832d5c` (11 passed, 1 skipped)
and the RHEL 10.2 builds `81bdfeec` and `0e65bf4b` (10 passed, 1
skipped), where the RHSM tmpfs mounts keep `/var/log/rhsm/rhsm.log` out of
the image. They are non-blocking. Whether to seek upstream fixes, declare
exceptions or use `--fatal-warnings` is a release decision; DeskOS does
not alter package behavior to silence them.

## Public CentOS, private RHEL

The public reference workstation `deskos-core-centos10` is
redistributable and is the source of `quay.io/deskos/deskos-core`.

> [!IMPORTANT]
> RHEL-derived images are covered by the RHEL EULA and are **never
> published**. The `rhel-10` Platform marks them
> `redistributable: false`, and plans and Containerfiles carry that
> notice. Subscription material never appears in resources.

## Platform capability gaps

Organizational intent can ask for something a platform does not supply.
The Platform lists such packages under `unavailablePackages` with a
reason, and **composition fails with that reason** instead of dropping or
substituting the package. The example organization shows this with
`virt-manager`, which EL10 ships only in the CodeReady Linux Builder
repository (unsupported on RHEL 10); see
[`examples/example-org/README.md`](../examples/example-org/README.md).

A Platform package group may list `excludePackages`: group members the
platform does not install with the group. They become `--exclude` options
of the single group transaction only, so a `PackageSet` that names one
still installs it. The CS10 Platform excludes setroubleshoot this way; see
[research-notes.md](research-notes.md).

## Validation status

| Claim | CentOS Stream 10 | RHEL 10 |
|---|---|---|
| Core and the example organization compose and render | **validated** | **validated** |
| Same Core content on both platforms | **validated** (test) | **validated** (test) |
| Image builds, `bootc container lint` passes | **validated** (CI) | **validated** on the factory host [^rhel-build] |
| Packages install from the declared sources | **validated** | **validated** (package presence; only Firefox launched) |
| Build layers free of build-host identity | not rechecked since the fix | **validated**: 0 findings in all layers [^layers] |
| Boots to GNOME with DeskOS defaults | **validated** [^cs10-boot] | **validated** on the factory host [^rhel-boot] |
| Automated runs | `vm-bootcheck.yml`, manual; with `publish` it pushes the tested image [^cs10-run] | `tests/rhel/factory.py`, every head of `main`, as the `deskos/rhel10` commit status |
| Failed system units in the session check | none except `mcelog.service` in AMD QEMU guests [^mcelog] | same as CentOS Stream 10 |
| Flatpak preinstall materializes apps | remote and ref resolution checked | not exercised [^flatpak] |

[^rhel-build]: Image `0e65bf4b` on RHEL 10.2, base `d13af792...`; lint
passed with the three known warnings.
[^layers]: All 75 layers of `0e65bf4b` scanned for RHSM paths and the
build host's entitlement serial, consumer UUID and hostname.
[^cs10-boot]: `bootcheck.py` on Core disks (pixel classes; frames show the
DeskOS splash and GNOME Initial Setup over the DeskOS wallpaper);
`sessioncheck.py` confirms Dash to Dock active, favorites and wallpaper
as planned and Firefox running headless; the GDM login logo was seen in a
Core VM. No full E2E.
[^rhel-boot]: Same checks on the `0e65bf4b` disk, under nested KVM on the
entitled factory VM (example wallpaper); not in CI; no full E2E.
[^cs10-run]: Last run `37162282876` on commit `8ab0cc9`, with the approved
DeskOS identity; it published `quay.io/deskos/deskos-core:8ab0cc9...` and
`:latest`.
[^mcelog]: In AMD QEMU guests without `edac_mce_amd`; reported as a known,
non-blocking diagnostic with its raw failed state (see
`tests/vm/README.md` and [research-notes.md](research-notes.md)).
[^flatpak]: `example-devops-rhel10` declares no Flatpaks.

Confirmed on RHEL 10.2 by that build: `workstation-product-environment`,
`gnome-shell-extension-dash-to-dock` 102, Terraform 1.16.5, kubectl
1.37.1, VS Code 1.140.0 and Chrome 154 from their vendor repositories,
and the rhel9 `oc` 4.22.14 build (needs at most GLIBC_2.34).

**Still open:** whether RHEL's flatpak 1.16 reads
`/usr/share/flatpak/preinstall.d`, and E2E tests on either platform.

## Supply-chain inputs

**Compilation is deterministic; builds are not yet reproducible bit for
bit.** External inputs that can still change between builds:

- the base image tag, unless `bootc.digest` is set (CentOS Stream 10 and
  RHEL 10 pin their x86_64 manifests);
- RPM repository metadata and packages;
- RPM repository GPG keys, which are fetched by URL with no independent
  identity.

`BinaryArtifact` already pins an exact version and SHA-256. The goal is
that every external input has an immutable, verifiable identity;
candidate mechanisms (repository-local key assets, SHA-256, expected
fingerprints, lockfile entries) are not chosen yet.

## Managed baseline

DeskOS manages what the organization requires **for any authorized user
of the machine**, available before any user's `$HOME` exists. Personal
and project tooling (Homebrew, mise, SDKMAN, language version managers,
dotfiles, personal Toolboxes) stays outside the resource model.

Acquisition order:

1. distribution RPM;
2. official vendor RPM repository;
3. checksum-pinned upstream binary;
4. system Flatpak;
5. in the future, managed web applications.

`BinaryArtifact` requires an exact version, an https URL that is not a
floating location, a SHA-256 checksum, and installs only directly into
`/usr/local/bin` with mode `0755` or `0555`.

## System Flatpaks

DeskOS is **image-first and Flatpak-enabled**. Core installs Flatpak; it
does not add a remote. Remotes go to `/usr/share/flatpak/remotes.d` and
applications to `/usr/share/flatpak/preinstall.d`, both image-owned. A
generated unit runs `flatpak preinstall` once per boot, as the upstream
man page expects the OS to do. This is *artifact materialization*, not a
DeskOS controller: upstream decides what to install or remove from the
image's vendor list, and does not reinstall apps a user removed.

The unit is `WantedBy=multi-user.target`, `Type=exec`, and ordered after
`network-online.target` and `multi-user.target`. Ordering it after
`multi-user.target` suppresses the implicit `After=` a target adds to
units it wants (systemd v257 `unit_add_default_target_dependency`), so
**DeskOS adds no network wait to any boot target**; `Type=exec` means
nothing waits for the download itself. It does not restart.

Offline boot, observed by booting the reference image under systemd in a
container with no network and a 25-second simulated `nm-online` timeout:

- `gdm.service` is not ordered after the network, `multi-user.target` or
  this unit, and was active while `network-online.target` was still
  pending.
- `graphical.target` still waits for `network-online.target`, because
  platform units (`kdump`, `rsyslog`, `insights-client-boot`) are wanted
  by `multi-user.target` and ordered after it; on EL10 that wait is
  bounded by `NM_ONLINE_TIMEOUT=60`. That is platform behavior, not
  DeskOS behavior.
- Without network, `flatpak preinstall` prints "Nothing to do.", exits 0
  and records nothing, so the apps stay eligible: a later run with
  network proposed the install again (it was declined in the test, so an
  actual installation after an offline boot is unverified). A download
  failure is expected to exit non-zero and leave the unit failed, visible
  in `systemctl status deskos-flatpak-preinstall` and the journal; this
  was not tested.

GDM itself was not exercised in the container (it has no display) and a
real offline VM boot has not been run. The public reference adds Flathub
and Bazaar through the separate `deskos-reference-apps` profile.

## GNOME

`GnomeProfile` exposes **administrator vocabulary** (`windows.buttons`,
`shell.favorites`, `session.idle.blankAfter`, `dock.position`, ...). The
desktop lowerer maps it to dconf keys and writes them to the platform's
vendor database (`distro` on EL10). Wallpaper files are repository assets
resolved relative to the declaring file and installed under
`/usr/share/deskos/backgrounds/`.

**Dock.** Dock settings apply only while the effective `dock.enabled` is
true. A higher-layer `dock.enabled: false` masks lower-layer dock options
and is reported as a warning by `plan` and `validate`; dock options
without an enabled dock, or at or above the disabling layer, are errors.
A disable writes `enabled-extensions` without the dock only when the
platform enables it by default or `dock.enabled` is locked. Core sets
only `dock.enabled: true` (the extension's defaults); an organization's
`dock.enabled: false` removes the package and the extension entry.
Favorites are native and work without the dock.

**Login screen.** `appearance.loginLogo` sets the GDM login-screen logo;
GDM's dconf profile reads `distro`, which Platforms declare with
`gnome.loginScreen`.

**First boot.** GNOME Initial Setup uses its own dconf profile, which does
not read `distro`, so the effective wallpaper is also written as a GLib
vendor override (`/usr/share/glib-2.0/schemas/50_deskos.gschema.override`,
above the distribution's `10_` override) and schemas are compiled in the
build. Verified by GSettings resolution under the `gnome-initial-setup`
profile and seen on the first-boot screen of a CS10 Core VM.

**Defaults and locks.** The EL10 profile reads `user`, `local`, `site`,
`distro`, highest first. Verified on the built image:

| DeskOS writes | Effect |
|---|---|
| a **default** | a value in `local` or `site`, or a user change, overrides it |
| a **lock** | the `distro` value applies and is not writable; values and even locks in `local` or `site` are ignored |

> [!WARNING]
> A lock is artifact-level enforcement that also defeats `site` and
> `local` overrides. DeskOS Core uses no locks. Lock only settings the
> organization wants the *image* to enforce, and leave settings managed by
> Ansible, Satellite or similar tools as defaults (or unset).

## Boot

`BootProfile` (`splash: graphical|text`, `quiet`, `watermark`) lowers to:

- bootc kernel arguments in `/usr/lib/bootc/kargs.d/50-deskos.toml` (the
  platform's `rhgb` and `quiet`);
- for `graphical`: the platform's Plymouth packages, a dracut drop-in
  adding the Plymouth module, and a rebuild of every image kernel's
  initramfs (`/usr/lib/modules/<kver>/initramfs.img`) after the image
  files are in place. **The build fails** if no kernel is found or
  Plymouth is missing from an initramfs.

`text` adds no splash argument and no rebuild; `quiet: false` adds no
`quiet`. bootc applies kargs.d at install and applies its diff on
updates; the initramfs is image content, so updates and rollbacks carry
it with the deployment.

**Watermark.** `watermark` (a PNG, at most 1024x1024, inside the resource
root) needs an effective `splash: graphical`. It becomes an image-owned
two-step theme `deskos` in `/usr/share/plymouth/themes/deskos/`:

- a generated `deskos.plymouth` (the stock spinner layout, own
  `ImageDir`, no firmware BGRT logo);
- the watermark as `watermark.png`;
- the platform's stock spinner frames copied in the build, except their
  `watermark.png`.

`plymouth-set-default-theme deskos` selects it before the initramfs
rebuild, and the build fails unless each initramfs contains the theme and
selects it. A higher-layer `splash: text` masks a lower-layer watermark
with a plan warning; a watermark at or above a `text` layer, or with no
splash set, is an error. Core sets the DeskOS lockup; without a watermark
the platform's default theme stays.

**Diagnostics stay available:** ESC switches Plymouth to details, systemd
still prints failures under `quiet`, and editing the boot entry to drop
`rhgb quiet` gives full text. Firmware, bootloader and early kernel output
before Plymouth starts can still appear. The DeskOS splash is seen at boot
and shutdown on UEFI VMs. A serial console turns Plymouth to text mode, so
test QCOW2s are built with bootc-image-builder's
`--no-default-kernel-args`, which omits its `console=ttyS0` (see
`tests/vm/README.md`); the artifact's own kernel arguments are unchanged.

## Drift and mutable `/etc`

bootc updates `/usr` atomically, but **`/etc` is persistent and merged
three ways**. Files DeskOS writes under `/etc` (dconf keyfiles, repository
files) follow image updates only while they are locally unmodified. A new
release changes the image; it does not force a machine that was edited
locally back into line. Organizations that need guaranteed enforcement use
configuration management on top of the image. A future `deskos status` or
`deskos diff` may *report* drift; DeskOS will not enforce.

## Provisioning and enrollment

| Stage | Contents |
|---|---|
| **Artifact** | everything identical across machines: packages, branding, client binaries, descriptors |
| **Provisioning** | per-machine install choices, a local bootstrap account, storage and encryption, first-boot Flatpak materialization |
| **Enrollment** | FreeIPA or AD join, device certificates, VPN identities, EDR tenancy |

One image digest serves many machines with distinct identities, so **no
identity or secret is ever baked into the artifact**. Enrollment is
reserved in the Plan and not implemented.

## Release model

The **OCI image is the primary artifact**; QCOW2 and ISO derive from it.

- **Published today:** the CentOS Stream 10 image on
  `quay.io/deskos/deskos-core`, pushed by `vm-bootcheck.yml` with
  `publish` on `main` only after its boot and session checks pass, as
  `:<commit>` and `:latest`.
- **Not yet published:** QCOW2 and ISO, to go on S3-compatible object
  storage, not on GitHub; until then they are built locally (see
  [install.md](install.md)). The CI builds QCOW2 disks only as test input.
- **Planned:** promote the same digest through channels (candidate,
  canary, pilot, stable) without rebuilding. A mirror on
  `ghcr.io/deskosproject/deskos-core` is possible and not decided.

`deskosctl` itself is released as a linux/amd64 binary on GitHub Releases
when a `vX.Y.Z` tag is pushed. GitHub Actions is the reference CI; it is
an adapter, not part of DeskOS semantics.
