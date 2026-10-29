# DeskOS architecture

Resources become a workstation image: boundary, resource model, composition,
typed Plan, Containerfile backend, and what is verified on each platform. What
the image does once it boots is [runtime.md](runtime.md).

## Boundary

DeskOS **compiles artifact state**. It does not reconcile running machines.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-flow-dark.svg">
    <img alt="DeskOS flow: organizational intent as a Workstation and Profiles in layers — workstation, role, organization and foundation, where the higher layer wins rather than file order — then deskosctl validating, composing, planning a typed IR and rendering a build context deterministically, with no shell during planning; podman builds one bootc OCI image per platform: CentOS Stream 10, published at quay.io/deskos/deskos-core-centos-stream-10; RHEL 10, built privately with a subscription and not published. bootc-image-builder makes QCOW2 disks and installer ISOs from those images" src="images/deskos-flow-light.svg" width="900">
  </picture>
</p>

Keeping 4,000 machines in a state is configuration management's job (Ansible,
Satellite, Foreman, MDM). DeskOS builds the baseline those systems operate on.
There is no controller, agent or reconciliation loop, and none is planned.
Scheduled updates ([Updates](#updates)) are image content: timers that stage the
next published image.

## Design principles

- Intent is data; compilation is deterministic.
- File order never defines precedence.
- Providers emit typed IR, not shell.
- Core provides capabilities; organizations provide policy and content.
- DeskOS owns the artifact, not the running machine.
- CI is an adapter, not the product.
- AI may assist development; it is not part of compiler semantics.
- Compose existing Linux technology rather than reinventing it.

## Resource model

```yaml
apiVersion: <group>.deskos.org/v1alpha1
kind: <Kind>
metadata: {name: <dns-label>}
spec: {...}
```

Identity is **GVK plus name**, unique across all loaded roots. Resources are
plain data: no anchors, aliases, merge keys, custom tags, shell, templates or
hooks.

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile`, `Theme` |
| `system.deskos.org` | `BootProfile`, `UpdatePolicy`, `TrustAnchor` |

Schemas live in `schemas/` and are embedded in `deskosctl`. Every kind is checked
twice: against its JSON Schema, and by strict typed decoding with semantic
validation in its provider.

Core is **embedded in `deskosctl`** and loads beside the roots you pass; several
organization roots can be combined. Organizations keep their own repositories
and reference Core by name, never copying it.

## Core, organizations and roles

- **`Platform`**: every platform difference — bootc base, redistribution,
  RPM groups, GNOME facts, boot facts, image-update units, Flatpak capabilities,
  trust-store paths.
- **`Profile`**: a reusable fragment at a semantic layer.
- **`Workstation`**: one Platform plus Profiles; it compiles into one image.

Core is `resources/profiles/deskos-core.yaml`, a foundation profile, with
`deskos-gnome` for GNOME defaults and `deskos-dock` for the dock. A RHEL
workstation starts from `registry.redhat.io/rhel10/rhel-bootc`, never from the
CentOS-based DeskOS image. A test asserts that the **two shipped RPM platforms**,
CentOS Stream 10 and RHEL 10, produce the same Core content over the fields it
compares (groups, packages, enabled units, dconf defaults, installed files,
default target). AlmaLinux and Fedora have their own roots; Ubuntu is in its own
repository ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

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

Profile order and file order never matter. Every contribution carries provenance
(resource, profile, layer, file, line) and conflicts list every contributor.
References are transitive: a `PackageSet` names its repositories, a `FlatpakSet`
its remote. Included resources contribute at the layer of the including profile.

## Providers and the registry

`internal/registry` maps GVK to a provider. A provider decodes and validates one
kind, may declare references and contribute to composition, and lowers composed
intent to Plan IR using only Platform facts. Providers have no side effects: no
commands, no network, and asset reads stay inside the declaring root. Unknown
kinds fail with `no provider registered for <group/version>, Kind <Kind>`.

## Plan (IR)

`internal/plan` is the typed IR, independent of YAML and of Containerfile
syntax. `deskosctl plan --format json` prints it.

- **Artifact**: base image, labels, `RpmRepository`, `AptRepository`,
  `RpmGroupInstall` (with `excludePackages`), `RpmInstall`, `PackageRemove`,
  `RpmFileInstall`, `VerifiedBinaryInstall`, `FileInstall`, `GeneratedFile`,
  `TrustAnchorInstall` (with `TrustStoreUpdate`), `DesktopEntry`,
  `IconCacheUpdate`, `DconfDatabase`, `GSettingsVendorDefault`, `KernelArgument`,
  `InitramfsRegeneration`, `PlymouthTheme`, `SystemdEnable`, `SystemdMask`,
  `ScheduledUpdate`, `DefaultTarget`.
- **Provisioning**: system Flatpak remotes and applications, materialized by
  upstream `flatpak preinstall`; an OCI remote with an authenticator also carries
  the unit that points the remote at it.
- **Enrollment**: reserved and empty.

Every item keeps its provenance. The plan is canonical: sorted, no maps, no
timestamps.

### Image identity summary

`/usr/share/deskos/image-info.json` (`deskos.image-info/v1`) names the
workstation and title, the platform and release, architectures, base reference
and digest, plan format and the profiles that contributed, sorted by layer and
name. It is generated from the Plan, so it adds no input.

It is not the machine identity and not `os-release`: the distribution identity
stays the vendor's. It carries no build-host path, commit or plan hash. The Plan
remains the authoritative record.

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
2. one group transaction with the platform's `--exclude` options;
3. one package transaction;
4. copies `rpm-keys/`, then in one RUN downloads each RPM, checks its SHA-256,
   checks its signature against its own key in a temporary keyring, and installs
   them; the image keyring is untouched;
5. removes every installed `Platform.removePackages` name and fails if one
   remains; an absent name is a no-op (ADR 0022);
6. one verified download per binary artifact;
7. copies `rootfs/`;
8. validates the GSettings override with `glib-compile-schemas --strict`, then
   compiles it as packages do;
9. runs `dconf update`, regenerates the icon cache and the trust store when
   needed, enables and masks units, sets the default target;
10. installs the Plymouth theme and rebuilds the initramfs when
    [Boot](#boot) needs them;
11. cleans caches and ends with `bootc container lint`.

Every command comes from a typed operation; values are validated upstream and
quoted again. The plan is installed as `/usr/share/deskos/plan.json`.
**Rendering needs no credentials and no network.**

### Build-host subscription state

On EL10 the subscription-manager dnf plugins run inside every transaction and
write `redhat.repo` plus state under `/var/lib/rhsm` and `/var/log/rhsm`. So
every emitted transaction mounts those directories as tmpfs, removes
`redhat.repo` in the same RUN (a later layer would leave it in history), and
fails if one already exists; the render also rejects an `RpmRepository` with id
`redhat`, so declared content is never removed.

### Lint warnings

`bootc container lint` passes with **three warnings**, all from distribution
scriptlets: content in `/run` (cockpit, cups), users without sysusers.d entries
(libstoragemgmt, wsdd; on RHEL also avahi and libvirtdbus), and `/var` content
without tmpfiles.d entries. Seen on CS10 builds `50f8892a` and `77832d5c` and
RHEL 10.2 builds `81bdfeec` and `0e65bf4b`.

**Disposition (2026-10-24):** accept them, keep the lint in report mode, never
`--fatal-warnings`, never patch the packages. A warning from a DeskOS `RUN` step
is a defect and blocks. Re-run each release.

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
reason, and **composition fails with that reason** instead of substituting. The
example organization shows this with `virt-manager`, which EL10 ships only in
CRB. A group may list `excludePackages`, which become `--exclude` options of the
group transaction only, so a `PackageSet` naming one still installs it: CS10
excludes setroubleshoot, RHEL 10 excludes `redhat-flatpak-preinstall-firefox`
(its OCI authenticator needs a session bus, which would break every system
`flatpak preinstall`). Core installs the Firefox RPM on both EL platforms.

## Validation status

The canonical matrix is [**platforms.md**](platforms.md); the evidence is here.
The RHEL column there is historical: the factory host was retired, and this
repository no longer builds or boots RHEL. Composition and rendering for RHEL
stay covered by tests.

### Evidence

- **Artifact identity (CI).** Each gate leg writes `identity.json`: commit, run,
  plan and manifest hashes, image ID and digest, OCI archive, disk hash, and the
  published digest with signature evidence when present. It is uploaded even on
  failure, and a successful leg checks it is complete. For a published digest it
  is also stored under `evidence.deskos.org`, keyed by digest and verified from
  the public URL.
- **CentOS Stream 10, built and booted.** `bootcheck.py` on Core disks (pixel
  classes; frames show the DeskOS splash and GNOME Initial Setup over the DeskOS
  wallpaper) and `sessioncheck.py` on the instrumented boot. The session check
  asserts the plan itself: every declared dconf default read twice, every enabled
  extension actually loaded, and the bytes of every installed binary against its
  pinned checksum; plus favorites, wallpaper and headless Firefox. On the
  reference workstation that is 23 defaults, both reads verified, the dock
  `Yes/ACTIVE`. The GDM login logo was seen in a Core VM.
- **Accessibility and one E2E step (2026-10-29).** The session exposes its
  applications through AT-SPI (nine on the reference image: the shell, its
  `gsd-*` daemons, the portal, ibus, the alarm notifier), and launching the
  Calculator desktop entry makes a new `gnome-calculator` window appear in the
  tree. Run `40795106343` on both images. The remaining scenarios are designed,
  not implemented ([session-e2e.md](design/session-e2e.md)).
- **RHEL 10.2, built and booted (historical).** Image `0e65bf4b` on base
  `d13af792…`; lint passed with the three warnings; all 75 layers scanned free of
  RHSM paths, entitlement serial, consumer UUID and hostname; `bootcheck.py` and
  `sessioncheck.py` passed on its disk under nested KVM on the factory VM.
- **Published, signed and durable (verified 2026-10-28).** One repository per
  shipped platform, published by the weekly gate (run `40716569103`, commit
  `2ef9f75`) and promoted automatically (run `40723460708`):
  `quay.io/deskos/deskos-core-centos-stream-10` at `sha256:871e59cc…`, with
  `:<commit>`, `:latest`, `:candidate` and `:stable` at that digest. `cosign
  verify` with the workflow issuer and this repository's identity passes, with
  SBOM, exception and SLSA-provenance attestations bound to the digest. Sixteen
  evidence objects were fetched back and verified from `evidence.deskos.org`,
  and the approval record names the gate run. The ISO for the same digest is
  `quay.io/deskos/deskos-core-centos-stream-10-iso`, signed and mirrored to
  `download.deskos.org`.
- **Example AlmaLinux and Fedora.** Their roots compose and render in CI, and
  `examples-weekly.yml` builds and boots them weekly without publishing.
- **Flatpak preinstall (RHEL).** A private RHEL 10.2 workstation with four
  Flathub apps installed all four from `/usr/share/flatpak/preinstall.d` with
  `flatpak preinstall --system` (no session bus).
- **Failed system units.** In AMD QEMU guests without `edac_mce_amd`,
  `mcelog.service` is the only tolerated failure, reported as a known
  diagnostic ([tests/vm/README.md](../tests/vm/README.md)).

**Still open:** the remaining E2E scenarios, and interactive GDM login.

## Supply-chain inputs

**Compilation is deterministic; builds are not yet bit-for-bit reproducible.**

- the base image tag, unless `bootc.digest` is set — both shipped Platforms pin
  a digest;
- repository metadata and packages: RPM repositories on the EL and Fedora
  platforms, APT sources on a Debian-family one.

`BinaryArtifact` pins an exact version and SHA-256; RPM files pin a SHA-256 and a
signing key; RPM repository keys are **local assets** read into the plan, so no
transaction trusts a key fetched by URL. The goal is an immutable, verifiable
identity for every input; the mechanism is not chosen.

## Runtime behaviour

What the image does once it boots — managed baseline, system Flatpaks, GNOME,
boot and trust, updates, `/etc` drift, and the provisioning boundary — is
[runtime.md](runtime.md). The plan format it consumes is above.

## Release model

The **OCI image is the primary artifact**; QCOW2 and ISO derive from it.

- **One repository per shipped platform**, `quay.io/deskos/deskos-core-centos-stream-10`
  today, with `:stable` at the digest in [Validation
  status](#validation-status). The weekly gate publishes by itself: builds,
  scans, boots, pushes and signs `:<commit>`, `:latest` and `:candidate` by
  digest; `promotion.yml` then verifies signature and evidence and moves
  `:stable` with a recorded approval. Not every green `main` publishes; pin a
  digest in production.
- **A CentOS Stream 10 preview ISO** is at <https://download.deskos.org/>.
  `iso.yml` builds it from the published digest after verifying the image's
  keyless signature and publishes it as a signed OCI artifact
  (`:candidate`, `sha256-<image digest>`, `:stable` once the image is stable). It
  is a preview, not a supported channel. QCOW2 files and RHEL ISOs are not
  published; CI builds a QCOW2 only as test input.
- **Promotion without a rebuild** was exercised on 2026-10-26 with an approval
  record in the evidence bucket ([promotion.md](design/promotion.md)): the first
  moved `sha256:96fe80d3…`, a second after the Stream 10 base bump moved
  `sha256:649aff75…` (run `40412048361`). The record names digest, actor,
  approver, reason and evidence index hash. No canary or pilot channel.

`deskosctl` is released as a linux/amd64 binary on GitHub Releases when a `vX.Y.Z`
tag is pushed. GitHub Actions is the reference CI, an adapter rather than part of
DeskOS semantics.
