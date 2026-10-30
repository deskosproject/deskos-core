# DeskOS architecture

Declared resources become a workstation image; runtime behaviour is in
[runtime.md](runtime.md).

## Boundary

DeskOS **compiles artifact state**. It does not reconcile running machines.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-flow-dark.svg">
    <img alt="DeskOS flow: organizational intent as a Workstation and Profiles in layers (workstation, role, organization and foundation, where the higher layer wins rather than file order), then deskosctl validating, composing, planning a typed IR and rendering a build context deterministically, with no shell during planning; podman builds one bootc OCI image per platform: CentOS Stream 10, published at quay.io/deskos/deskos-core-centos-stream-10; RHEL 10, built privately with a subscription and not published. bootc-image-builder makes QCOW2 disks and installer ISOs from those images" src="images/deskos-flow-light.svg" width="900">
  </picture>
</p>

Keeping 4,000 machines in a state is configuration management's job (Ansible,
Satellite, Foreman, MDM); DeskOS builds the baseline they operate on. No controller, agent or
reconciliation loop exists or is planned. Scheduled updates ([Updates](runtime.md#updates))
are timers staging the next published image.

## Design principles

Intent is data and compilation is deterministic. File order never defines
precedence. Providers emit typed IR, not shell. Core provides capabilities,
organizations provide policy and content. DeskOS owns the artifact, not the
machine. CI is an adapter, not the product. AI may assist development, never
compiler semantics. Existing Linux technology is composed, not reinvented.

## Resource model

```yaml
apiVersion: <group>.deskos.org/v1alpha1
kind: <Kind>
metadata: {name: <dns-label>}
spec: {...}
```

Identity is **GVK plus name**, unique across all loaded roots. Resources are plain
data: no anchors, aliases, merge keys, custom tags, shell, templates or hooks.

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile`, `Theme` |
| `system.deskos.org` | `BootProfile`, `UpdatePolicy`, `TrustAnchor`, `NetworkProfile` |

Schemas live in `schemas/` and are embedded in `deskosctl`. Every kind is checked
twice: against its JSON Schema, and by strict typed decoding with semantic
validation in its provider. Core is **embedded in `deskosctl`** and loads beside
the roots you pass; organizations keep their own repositories and reference Core
by name.

## Core, organizations and roles

- **`Platform`**: the bootc base, redistribution, RPM groups, GNOME and boot
  facts, image-update units, Flatpak capabilities and trust-store paths.
- **`Profile`**: a reusable fragment at a semantic layer.
- **`Workstation`**: one Platform plus Profiles; it compiles into one image.

Core is `resources/profiles/deskos-core.yaml`, a foundation profile, with
`deskos-gnome` for GNOME defaults and `deskos-dock` for the dock. A RHEL
workstation starts from `registry.redhat.io/rhel10/rhel-bootc`, never from the
CentOS-based DeskOS image. A test asserts that the **two shipped RPM platforms**,
CentOS Stream 10 and RHEL 10, produce the same Core content over the fields it
compares: groups, packages, enabled units, dconf defaults, installed files, default
target. AlmaLinux and Fedora have their own roots; Ubuntu has its own repository
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="images/deskos-composition-light.svg" width="900">
  </picture>
</p>

Layers: `foundation` < `organization` < `role` < `workstation`.

| Class | Used for | Rule |
|---|---|---|
| **set** | packages, groups, units, GNOME locks | union; duplicates collapse |
| **keyed** | repositories, RPM files, binary destinations, desktop entries, Flatpak remotes and applications, trust anchors | identical definitions deduplicate; different ones conflict at any layer |
| **scalar** | GNOME, boot and update settings | highest layer wins; different values at one layer conflict |

Every contribution carries provenance (resource, profile, layer, file, line) and
conflicts name every contributor. References are transitive (`PackageSet` to
repositories, `FlatpakSet` to remote); included resources join at the includer's
layer.

## What each layer owns

**The image carries policy, provisioning carries identity, Ansible carries state.**

Three layers, and a field that belongs to the wrong one becomes either drift or a
secret in a public artifact:

| Layer | Owns | Examples |
|---|---|---|
| The image (a `Workstation`) | the baseline that must be identical on every machine of a fleet | appearance and fonts, locks, privacy defaults, proxy, idle and lock policy, default applications, update policy, trust anchors, Flatpak and extensions |
| Provisioning (kickstart, PXE or Satellite) | the identity of one machine | hostname, network and 802.1X enrolment, disk layout, subscription, accounts and SSH keys, timezone and locale |
| Ansible, after the fact | state that changes per machine or per person | printers, VPN credentials, certificates that rotate, software installed later, a user's own settings |

A value that names a machine or a person is not image content, and a value that is
a secret is not resource content: the image would make it identical everywhere and
the repository would publish it. That is why `appearance.fonts.terminal` exists and
a proxy password does not (ADR 0025), and why the fleet's identity starts at the
installer, not in `deskosctl`.

## Providers and the registry

`internal/registry` maps GVK to a provider. A provider decodes and validates one
kind, may declare references and contribute to composition, and lowers composed
intent to Plan IR using only Platform facts. It has no side effects: no commands,
no network, reads inside the declaring root. An unknown kind fails with `no
provider registered for <group/version>, Kind <Kind>`.

## Plan (IR)

`internal/plan` is the typed IR, independent of YAML and of Containerfile
syntax; `deskosctl plan --format json` prints it.

- **Artifact**: base image, labels, `RpmRepository`, `AptRepository`,
  `RpmGroupInstall` (with `excludePackages`), `RpmInstall`, `PackageRemove`,
  `RpmFileInstall`, `VerifiedBinaryInstall`, `FileInstall`, `GeneratedFile`,
  `TrustAnchorInstall` (with `TrustStoreUpdate`), `DesktopEntry`,
  `IconCacheUpdate`, `DconfDatabase`, `GSettingsVendorDefault`, `KernelArgument`,
  `InitramfsRegeneration`, `PlymouthTheme`, `SystemdEnable`, `SystemdMask`,
  `ScheduledUpdate`, `DefaultTarget`.
- **Provisioning**: system Flatpak remotes and applications, materialized by
  upstream `flatpak preinstall`; an OCI remote with an authenticator carries the
  unit that points at it.
- **Enrollment**: reserved and empty.

Every item keeps its provenance; the plan is canonical, sorted, with no maps or
timestamps.

### Image identity summary

`/usr/share/deskos/image-info.json` (`deskos.image-info/v1`) names the workstation
and title, the platform and release, architectures, base reference and digest, plan
format, and the contributing profiles, sorted by layer and name. It comes from the
Plan, so it adds no input, and it is not the machine identity or `os-release`:
distribution identity stays the vendor's. It carries no build-host path, commit or
plan hash; the Plan remains the authoritative record.

## Containerfile backend

`internal/backends/containerfile` reads **only the Plan** and renders:

```text
Containerfile
plan.json                 canonical Plan
generated-manifest.json   every file with mode and sha256
repos/etc/yum.repos.d/    one file per RpmRepository
repos/etc/pki/rpm-gpg/    RPM repository keys, named by sha256
repos/etc/apt/sources.list.d/  one deb822 source per AptRepository
repos/etc/apt/keyrings/   APT keys, named by sha256
rpm-keys/                 public keys of RPM files, named by sha256
rootfs/                   image files, copied after package installation
```

The Containerfile, in order:

1. copies the repository files, so every later transaction installs from a
   declared source;
2. one group transaction with the platform's `--exclude` options, then one
   package transaction;
3. copies `rpm-keys/`, then in one RUN downloads each RPM, checks its SHA-256 and
   its signature against its own key in a temporary keyring, and installs them;
4. removes every installed `Platform.removePackages` name and fails if one
   remains; an absent name is a no-op (ADR 0022);
5. one verified download per binary artifact;
6. copies `rootfs/`;
7. validates the GSettings override with `glib-compile-schemas --strict`, then
   compiles it as packages do;
8. runs `dconf update`, regenerates the icon cache and the trust store, enables
   and masks units, sets the default target;
9. installs the Plymouth theme and rebuilds the initramfs when
   [Boot](runtime.md#boot) needs them;
10. cleans caches and ends with `bootc container lint`.

Every command comes from a typed operation; values are validated upstream and
quoted again. The plan is installed as `/usr/share/deskos/plan.json`; rendering
needs no credentials and no network.

### Build-host subscription state

On EL10 the subscription-manager dnf plugins run inside every transaction and
write `redhat.repo` plus state under `/var/lib/rhsm` and `/var/log/rhsm`. Each
transaction therefore mounts those directories as tmpfs, removes `redhat.repo` in
the same RUN (a later layer keeps it in history), and fails if one already exists;
the render rejects an `RpmRepository` with id `redhat`.

### Lint warnings

`bootc container lint` passes with **three warnings**, all from distribution
scriptlets: content in `/run` (cockpit, cups), users without sysusers.d entries
(libstoragemgmt, wsdd; on RHEL also avahi and libvirtdbus), and `/var` content
without tmpfiles.d entries. Seen on CS10 builds `50f8892a` and `77832d5c` and
RHEL 10.2 builds `81bdfeec` and `0e65bf4b`.

**Disposition (2026-10-24):** accept them, keep the lint in report mode, never
`--fatal-warnings`, never patch the packages. A warning from a DeskOS `RUN` step is
a defect and blocks; re-run each release.

## Public CentOS, private RHEL

`deskos-core-centos-stream-10` is redistributable and is the source of
`quay.io/deskos/deskos-core-centos-stream-10`.

> [!IMPORTANT]
> RHEL-derived images are covered by the RHEL EULA and are **never published**.
> The `rhel-10` Platform marks them `redistributable: false`, and plans and
> Containerfiles carry the notice. Subscription material never appears in
> resources.

## Platform capability gaps

A Platform lists packages it cannot supply under `unavailablePackages` with a
reason, and **composition fails with that reason** instead of substituting; the
example organization shows this with `virt-manager`, which EL10 ships only in CRB.
A group may list `excludePackages`, which become `--exclude` options of the group
transaction only, so a `PackageSet` naming one still installs it: CS10 excludes
setroubleshoot, RHEL 10 excludes `redhat-flatpak-preinstall-firefox` (its OCI
authenticator needs a session bus and would break `flatpak preinstall`). Core
installs the Firefox RPM on both EL platforms.

## Validation status

The matrix is [platforms.md](platforms.md); the records are here. Its RHEL column
is historical: the factory host was retired and this repository no longer builds
or boots RHEL, though composition and rendering stay covered by tests.

### Evidence

- **Artifact identity (CI).** Each leg of the weekly build writes
  `identity.json`: commit, run, plan and manifest hashes, image ID and digest,
  OCI archive, disk hash, and the published digest with its signature. It uploads
  even on failure. For a published digest it is also stored under
  `evidence.deskos.org`, keyed by digest and verified from the public URL.
- **CentOS Stream 10, built and booted.** `bootcheck.py` on Core disks (pixel
  classes; frames show the DeskOS splash and GNOME Initial Setup over the
  wallpaper) and `sessioncheck.py`, which runs the plan's probes from
  [checks.md](checks.md#workstation-behavior-matrix). The reference workstation:
  23 dconf defaults read twice, dock `Yes/ACTIVE`; the GDM login logo was seen in
  a Core VM.
- **Accessibility and one E2E step (2026-10-29).** The session exposes nine
  applications through AT-SPI (the shell, its `gsd-*` daemons, the portal, ibus,
  the alarm notifier), and launching the Calculator entry opens a
  `gnome-calculator` window; run `40795106343` on both images. Other scenarios are
  designed, not implemented ([session-e2e.md](design/session-e2e.md)).
- **RHEL 10.2, built and booted (historical).** Image `0e65bf4b` on base
  `d13af792…`; lint passed with the three warnings; all 75 layers scanned free of
  RHSM paths, entitlement serial, consumer UUID and hostname; `bootcheck.py` and
  `sessioncheck.py` passed on its disk under nested KVM on the factory VM.
- **Published, signed and durable (verified 2026-10-28).** One repository per
  shipped platform, published by the weekly build (run `40716569103`, commit
  `2ef9f75`) and promoted automatically (run `40723460708`):
  `quay.io/deskos/deskos-core-centos-stream-10` at `sha256:871e59cc…`, with
  `:<commit>`, `:latest`, `:candidate` and `:stable` at that digest. `cosign verify`
  with the
  workflow issuer and this repository's identity passes, with SBOM, exception and
  SLSA-provenance attestations bound to the digest. Sixteen records came back
  verified from `evidence.deskos.org`; the approval record names the build run.
  The same digest's ISO is signed and mirrored to `download.deskos.org`.
- **Example AlmaLinux and Fedora.** Their roots compose and render in CI, and
  `examples-weekly.yml` builds and boots them weekly without publishing.
- **Flatpak preinstall (RHEL).** A private RHEL 10.2 workstation with four
  Flathub apps installed all four from `/usr/share/flatpak/preinstall.d` with
  `flatpak preinstall --system` (no session bus).
- **Failed system units.** In AMD QEMU guests without `edac_mce_amd`, only
  `mcelog.service` fails ([tests/vm/README.md](../tests/vm/README.md)).

**Still open:** the remaining E2E scenarios, and an interactive GDM login (the
next step in Milestone 5).

## Supply-chain inputs

**Compilation is deterministic; builds are not yet bit-for-bit reproducible.**

- the base image tag, unless `bootc.digest` is set (both shipped Platforms pin a
  digest);
- repository metadata and packages: RPM repositories on the EL and Fedora
  platforms, APT sources on a Debian-family one.

`BinaryArtifact` pins an exact version and SHA-256; RPM files pin a SHA-256 and a
signing key; RPM repository keys are **local assets** read into the plan, so no
transaction trusts a key fetched by URL. Every input should have an immutable,
verifiable identity; the mechanism is not chosen.

## Runtime behaviour

Everything after boot is in [runtime.md](runtime.md).

## Release model

The **OCI image is the primary artifact**; QCOW2 and ISO derive from it.

- **One repository per shipped platform**, `quay.io/deskos/deskos-core-centos-stream-10`
  today, with `:stable` at the digest in [Validation
  status](#validation-status). The weekly build publishes by itself: it builds,
  scans and boots, then pushes and signs `:<commit>`, `:latest` and `:candidate` by
  digest; `promotion.yml` verifies the signature and the stored records and moves
  `:stable` with a recorded approval. Not every green `main` publishes; pin a
  digest in production.
- **A CentOS Stream 10 preview ISO** is at <https://download.deskos.org/>.
  `iso.yml` builds it from the published digest after verifying the image's
  keyless signature, and publishes it as a signed OCI artifact (`:candidate`,
  `sha256-<image digest>`, `:stable` once stable). It is a preview, not a supported
  channel; QCOW2 and RHEL ISOs are never published, and CI builds a QCOW2 only as
  test input.
- **Promotion without a rebuild** was exercised on 2026-10-26 with an approval
  record at `evidence.deskos.org` ([promotion.md](design/promotion.md)): the first
  moved `sha256:96fe80d3…`, a second after the Stream 10 base bump moved
  `sha256:649aff75…` (run `40412048361`). The record names digest, actor,
  approver, reason and index hash; no canary or pilot channel exists.

`deskosctl` is released as a linux/amd64 binary on GitHub Releases when a `vX.Y.Z`
tag is pushed. GitHub Actions is the reference CI, an adapter rather than part of
DeskOS semantics.
