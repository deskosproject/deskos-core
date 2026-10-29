# Resources

The DeskOS resource model: every kind with an example from this repository, how
layers compose, and the software and GNOME model behind them.

## Envelope

DeskOS configuration uses **versioned, Kubernetes-inspired resource envelopes**:
it borrows the useful API ideas (Group, Version, Kind, schemas, composition)
*without requiring Kubernetes*.

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-developer-centos-stream-10
spec:
  platformRef: centos-stream-10
  profiles:
    - deskos-core
    - deskos-gnome
    - example-organization
    - example-developer
```

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile`, `Theme` |
| `system.deskos.org` | `BootProfile`, `UpdatePolicy`, `TrustAnchor` |

All kinds are `v1alpha1`, with JSON Schemas in [`schemas/`](../schemas/).

## Resource roots

Resources live in YAML files under **one or more resource roots**. DeskOS Core is
**embedded in `deskosctl`** and loaded by default; an organization keeps its own
root and references Core resources by kind and name **without copying them**.
`--no-core` drops the Core to compile against a tree you pass, and `deskosctl core
export DIR` writes it out.

- File and directory names have no meaning.
- Asset paths are relative to the YAML file.
- Examples come from [`resources/`](../resources/) and [`examples/`](../examples/);
  the ones that are not a file of this repository are labelled illustrative.

## Resource kinds

| Area | Kinds | File |
|---|---|---|
| Core | `Workstation`, `Profile`, `Platform` | [core.md](resources/core.md) |
| Software | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` | [software.md](resources/software.md) |
| Desktop | `GnomeProfile`, `Theme` | [desktop.md](resources/desktop.md) |
| System | `BootProfile`, `UpdatePolicy`, `TrustAnchor` | [system.md](resources/system.md) |

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="images/deskos-composition-light.svg" width="900">
  </picture>
</p>

A workstation is composed from **explicit semantic layers**:
`foundation` < `organization` < `role` < `workstation`. The full rules (set,
keyed and scalar classes, provenance) are in
[architecture.md](architecture.md#composition).

## DeskOS Core

**DeskOS Core** is the reusable workstation foundation maintained by the
project. The public reference workstation uses **CentOS Stream 10**.

- Organizations that require **RHEL** compile the same Core onto the official
  RHEL 10 bootc base; the RHEL artifact is **not derived** from the CentOS
  image.
- **AlmaLinux 10** and **Fedora 45** compose the same Core in example roots,
  not shipped platforms. They are built and booted by the weekly example check
  without being published; see
  [example-platforms.md](example-platforms.md).
- **Ubuntu 26.04** composes the same Core in its own repository, on a base owned by
  the Project DeskOS and without the RHEL family's software
  ([ADR 0014](adr/0014-platform-base-ownership.md), [ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

## Software model

DeskOS deliberately **avoids becoming a universal package manager**: the baseline
uses the delivery mechanism appropriate to the software, in order of preference:

1. distribution RPM;
2. official vendor RPM, from its repository or, when none is published, as a
   signed RPM file;
3. verified upstream binary;
4. system Flatpak;
5. future managed web applications.

User and project environments (mise, SDKMAN, Homebrew, version managers, dotfiles,
personal Toolboxes) are outside the baseline ([runtime.md](runtime.md#managed-baseline)).

## GNOME

DeskOS exposes **administrator intent**, not dconf implementation details. An
administrator describes window controls, wallpapers and branding, fonts, icon
and cursor themes, color scheme and accent color, ordered favorites, dock
behavior, idle timeout, lock behavior, whether GNOME Software applies updates,
hot corners, workspaces, app folders, the clock and Num Lock at login, and the
terminal shortcut — *without knowing* which GNOME schema or dconf key
implements them.

Window controls write exactly the buttons and the side a profile declares:
the plan adds no app menu token of its own
([ADR 0016](adr/0016-vendor-desktop-and-app-menu.md)).

A platform whose base ships a desktop of its own declares
`gnome.keepsVendorDefaults: true` instead of `gnome.dconfDatabase`. DeskOS then
writes no GNOME default or lock for it, and **any** GNOME setting a profile
declares there fails composition with the reason, at any layer. Such a platform
must also not declare `gnome.loginScreen`.
