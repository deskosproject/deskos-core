# DeskOS architecture

How declared resources become one bootc image. Runtime behaviour is in
[runtime.md](runtime.md).

DeskOS compiles artifact state. It does not reconcile running machines: no
controller, no agent, no loop. Scheduled updates are timers that stage the next
published image.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="/images/deskos-flow-dark.svg">
    <img alt="DeskOS flow: organizational intent as a Workstation and Profiles in layers (workstation, role, organization and foundation, where the higher layer wins rather than file order), then deskosctl validating, composing, planning a typed IR and rendering a build context deterministically, with no shell during planning; podman builds one bootc OCI image per platform: CentOS Stream 10, AlmaLinux 10 and Fedora 45, which are published, and RHEL 10, built privately with a subscription and not published. bootc-image-builder writes a QCOW2 disk from any of them; only CentOS Stream 10 also has an installer ISO" src="/images/deskos-flow-light.svg" width="900">
  </picture>
</p>

## Boundary

| DeskOS | Configuration management |
|---|---|
| builds the baseline: resources to Plan to image | keeps 4,000 machines in a state: Ansible, Satellite, Foreman, MDM |
| owns the artifact and its evidence | owns the running machine, its drift and its remediation |

DeskOS does not replace those tools. A machine installs an image and later moves
to a newer one.

## Resource model

```yaml
apiVersion: <group>.deskos.org/v1alpha1
kind: <Kind>
metadata: {name: <dns-label>}
spec: {...}
```

Identity is GVK plus name, unique across every loaded root. Resources are plain
data: no anchors, aliases, merge keys, custom tags, shell, templates or hooks.

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile`, `Theme` |
| `system.deskos.org` | `BootProfile`, `UpdatePolicy`, `TrustAnchor`, `NetworkProfile` |

Schemas live in `schemas/` and are embedded in `deskosctl`. Every kind is checked
twice: against its JSON Schema, then by strict typed decoding with semantic
validation in its provider. Core is embedded in the binary and loads beside the
roots passed on the command line.

## Core, organizations and roles

| Kind | What it is |
|---|---|
| `Platform` | the bootc base, redistribution, RPM groups, GNOME and boot facts, image-update units, Flatpak capabilities, trust-store paths |
| `Profile` | a reusable fragment at one semantic layer |
| `Workstation` | one Platform plus Profiles; it compiles into one image |

Core is a foundation profile with `deskos-gnome` for GNOME defaults and `deskos-
dock` for the dock. A RHEL workstation starts from
`registry.redhat.io/rhel10/rhel-bootc`, never from the CentOS-based DeskOS
image.

AlmaLinux and Fedora are shipped platforms with their own roots; Ubuntu lives in
its own repository ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="/images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="/images/deskos-composition-light.svg" width="900">
  </picture>
</p>
Layers: `foundation` < `organization` < `role` < `workstation`.

| Class | Used for | Rule |
|---|---|---|
| set | packages, groups, units, GNOME locks | union; duplicates collapse |
| keyed | repositories, RPM files, binary destinations, desktop entries, Flatpak remotes and applications, trust anchors | identical definitions deduplicate; different ones conflict at any layer |
| scalar | GNOME, boot and update settings | highest layer wins; different values at one layer conflict |

Every contribution carries provenance: resource, profile, layer, file, line.
Conflicts name every contributor. References are transitive (a `PackageSet` to
its repositories, a `FlatpakSet` to its remote), and an included resource joins
at the includer's layer.

## What each layer owns

The image carries policy, provisioning carries identity, Ansible carries state.

| Layer | Owns | Examples |
|---|---|---|
| Image | the baseline identical on every machine of a fleet | appearance, fonts, locks, privacy, proxy, idle policy, default apps, update policy, trust anchors, Flatpak, extensions |
| Provisioning (kickstart, PXE, Satellite) | the identity of one machine | hostname, network and 802.1X, disk layout, subscription, accounts, SSH keys, timezone, locale |
| Ansible, after the fact | state that changes per machine or per person | printers, VPN credentials, rotating certificates, later software, a user's own settings |

A value that names a machine or a person is not image content. A secret is not
resource content: the image would make it identical everywhere and the
repository would publish it. That is why `appearance.fonts.terminal` exists and a
proxy password does not (ADR 0025).

## Providers and registry

`internal/registry` maps a GVK to a provider. A provider decodes and validates
one kind, declares references, contributes to composition, and lowers composed
intent to Plan IR using only Platform facts.

A provider has no side effects: no commands, no network, reads inside the
declaring root. An unknown kind fails with
`no provider registered for <group/version>, Kind <Kind>`.

## Plan (IR)

`internal/plan` is the typed IR, independent of YAML and Containerfile syntax.
`deskosctl plan --format json` prints it.

| Part | Holds |
|---|---|
| Artifact | base image, labels, repositories, group and package installs, removals, RPM files, verified binaries, files, trust anchors, desktop entries, icon cache, dconf, kernel arguments, initramfs, Plymouth, units, scheduled updates, default target |
| Provisioning | system Flatpak remotes and applications, materialized by `flatpak preinstall` |
| Enrollment | reserved and empty |

Every item keeps its provenance. The Plan is canonical, sorted, with no maps and
no timestamps.

`/usr/share/deskos/image-info.json` (`deskos.image-info/v1`) names the
workstation, platform, architectures, base reference and digest, plan format and
contributing profiles. It comes from the Plan and is not machine identity or
`os-release`.

## Containerfile backend

`internal/backends/containerfile` reads only the Plan and renders:

```text
Containerfile
plan.json                 canonical Plan
generated-manifest.json   every file with mode and sha256
repos/etc/yum.repos.d/    one file per RpmRepository
repos/etc/pki/rpm-gpg/    RPM keys, named by sha256
repos/etc/apt/sources.list.d/
repos/etc/apt/keyrings/   APT keys, named by sha256
rpm-keys/                 public keys of RPM files, named by sha256
rootfs/                   image files, copied after package installation
```

The Containerfile, in order:

1. copies the repository files;
2. runs one group transaction with the platform's `--exclude` options, then one
   package transaction;
3. copies `rpm-keys/`, then downloads each RPM in one RUN, checks its SHA-256 and
   signature against its own key, and installs it;
4. removes every `Platform.removePackages` name and fails if one remains;
5. downloads and verifies each binary artifact;
6. copies `rootfs/`;
7. validates the GSettings override with `glib-compile-schemas --strict`, then
   compiles it;
8. runs `dconf update`, regenerates the icon cache and trust store, enables and
   masks units, sets the default target;
9. installs the Plymouth theme and rebuilds the initramfs when Boot needs them;
10. cleans caches and ends with `bootc container lint`.

Every command comes from a typed operation. Values are validated upstream and
quoted again. The Plan is installed as `/usr/share/deskos/plan.json`. Rendering
needs no credentials and no network.

On EL10 the subscription-manager plugins write `redhat.repo` and state under
`/var/lib/rhsm` and `/var/log/rhsm`. Every transaction mounts those directories
as tmpfs, removes `redhat.repo` in the same RUN, and fails if it already exists.
The render rejects an `RpmRepository` with id `redhat`.

## Lint warnings

`bootc container lint` passes with three warnings, all from distribution
scriptlets: content in `/run` (cockpit, cups), users without sysusers.d entries
(libstoragemgmt, wsdd; on RHEL also avahi and libvirtdbus), and `/var` content
without tmpfiles.d entries. They are accepted, the lint stays in report mode,
and a warning from a DeskOS RUN step blocks.

Re-run each release.

## Platform capability gaps

A Platform lists packages it cannot supply under `unavailablePackages`, with a
reason. Composition fails with that reason instead of substituting.

A group may list `excludePackages`: those become `--exclude` options of the
group transaction only, so a `PackageSet` naming one still installs it. CS10
excludes setroubleshoot; RHEL 10 excludes `redhat-flatpak-preinstall-firefox`,
whose OCI authenticator needs a session bus.

## Public CentOS, private RHEL

`deskos-core-centos-stream-10` is redistributable and is the source of
`quay.io/deskos/deskos-core-centos-stream-10`. RHEL-derived images are covered
by the RHEL EULA and are never published: the `rhel-10` Platform marks them
`redistributable: false`, and plans and Containerfiles carry the notice.

Subscription material never appears in resources.

## Validation status

The matrix is [platforms.md](platforms.md). Digests and their evidence are at

<https://evidence.deskos.org>; this table names what was run, not a digest that
moves.

| Platform | Last known | Where |
|---|---|---|
| CentOS Stream 10 | built, booted and session-checked on the 2026-11-03 base (run `41766072582`) | `image-bootcheck.yml`, `tests/vm` |
| AlmaLinux 10 | published by the weekly build; boot and session checks run in the publish leg | `image-publish.yml` |
| Fedora 45 | same run, on `45.20261103.0` | `image-bootcheck.yml` |
| RHEL 10 | built and booted on the retired factory (historical); composition and rendering stay tested here | [ADR 0014](adr/0014-platform-base-ownership.md) |

Update and rollback are not inferred from unit tests. `lifecycle-test.yml`
installs a CentOS Stream 10 disk, moves A to B and back, injects a power loss in
the deployment write and again after the staged entry is active, and recovers a
candidate that cannot boot. The run records deployment identities, `/var` and
`/etc` state, and one journal per boot ([design](design/lifecycle.md)).

The session check reads every dconf default, extension, favorite, wallpaper and
binary the Plan declares. It also opens applications through AT-SPI, opens
Settings and closes a file-manager window. A sidebar click and lock/unlock remain
designed ([session-e2e.md](design/session-e2e.md)).

Failed system units: in AMD QEMU guests without `edac_mce_amd`, only
`mcelog.service` fails. `deskos check` names anything else.

The FreeIPA join, the direct AD join and the AD login through a FreeIPA trust
pass on the current base: the directory user's home is created by
`oddjob-mkhomedir`, `/var/home` carries its SELinux label, and the GDM session
belongs to the AD account. The design note names the runs and the assertions
([identity validation](design/identity-validation.md)).

Two gaps stay open: the remaining E2E scenarios, and bare-metal validation. A
virtual machine is not a laptop.

## Supply chain inputs

Compilation is deterministic. Image builds are not yet bit-for-bit reproducible.

| Input | Pinned by |
|---|---|
| base image | tag plus digest on every shipped Platform |
| RPM and APT repositories | repository metadata and signing keys |
| binary artifacts | exact version and SHA-256 |
| RPM files | SHA-256 and a signing key |

## Release model

The OCI image is the primary artifact; QCOW2 and ISO derive from it. One
repository per shipped platform.

A weekly build publishes `:<commit>`, `:latest` and `:candidate` by digest,
after building, scanning and booting. `:stable` moves only through
[promotion.yml](design/promotion.md), which verifies the signature and the
records. The promotion runs once for every `publish (…)` leg the run completed:
CentOS Stream 10, AlmaLinux 10 and Fedora 45 all receive `:stable` automatically.
A leg that failed is left alone and does not hold back the others.

Pin a digest in production.

`deskosctl` is a linux/amd64 binary on GitHub Releases when a `vX.Y.Z` tag is
pushed. GitHub Actions is the reference CI, an adapter rather than part of DeskOS
semantics.
