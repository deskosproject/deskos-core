# Resources

Every DeskOS kind, how layers compose, and the software and GNOME model behind
them.

## Envelope

DeskOS configuration uses versioned, Kubernetes-inspired envelopes: it borrows the
API ideas (Group, Version, Kind, schemas, composition) without Kubernetes.

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
| `system.deskos.org` | `BootProfile`, `UpdatePolicy`, `TrustAnchor`, `NetworkProfile` |

All kinds are `v1alpha1`, with JSON Schemas in [`schemas/`](../schemas/).

## Resource roots

Resources live in YAML files under one or more resource roots. DeskOS Core is
embedded in `deskosctl` and loaded by default; an organization keeps its own root
and references Core by kind and name without copying it. `--no-core` drops the Core
to compile against a tree you pass, and `deskosctl core export DIR` writes it out.

- File and directory names have no meaning; asset paths resolve against the YAML
  file.
- Examples come from [`resources/`](../resources/) and [`examples/`](../examples/);
  one not in this repository is labelled illustrative.

## Resource kinds

| Area | Kinds | File |
|---|---|---|
| Core | `Workstation`, `Profile`, `Platform` | [core.md](resources/core.md) |
| Software | `PackageSet`, `RpmRepository`, `AptRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` | [software.md](resources/software.md) |
| Desktop | `GnomeProfile`, `Theme` | [desktop.md](resources/desktop.md) |
| System | `BootProfile`, `UpdatePolicy`, `TrustAnchor`, `NetworkProfile` | [system.md](resources/system.md) |

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="images/deskos-composition-light.svg" width="900">
  </picture>
</p>

A workstation composes from explicit semantic layers: `foundation` <
`organization` < `role` < `workstation`. The rules for set, keyed and scalar
classes and for provenance are in [architecture.md](architecture.md#composition).

## DeskOS Core

DeskOS Core is the reusable workstation foundation maintained by the project. The
public reference workstation uses CentOS Stream 10.

- RHEL: the same Core on the official RHEL 10 bootc base, never derived from the
  CentOS image.
- AlmaLinux 10 and Fedora 45: the same Core in example roots, built and booted by
  the weekly check but never published
  ([example-platforms.md](example-platforms.md)).
- Ubuntu 26.04: the same Core in its own repository, on a base owned by the Project
  DeskOS and without the RHEL family's software
  ([ADR 0014](adr/0014-platform-base-ownership.md),
  [ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

## Software model

DeskOS avoids becoming a universal package manager. The baseline prefers, in
order: a distribution RPM; an official vendor RPM, from its repository or as a
signed file; a verified upstream binary; a system Flatpak; a future managed web
application. Mise, SDKMAN, Homebrew, version managers, dotfiles and personal
Toolboxes are outside the baseline ([runtime.md](runtime.md#managed-baseline)).

## GNOME

DeskOS exposes administrator intent, not dconf keys. A profile sets window
controls, wallpapers and branding, fonts, icon and cursor themes, the color scheme
and accent, ordered favorites, dock behavior, idle timeout, lock behavior, whether
GNOME Software applies updates, hot corners, workspaces, app folders, the clock,
Num Lock at login and the terminal shortcut, without naming a schema or a key.

Window controls write exactly the buttons and side a profile declares; the plan
adds no app menu token ([ADR 0016](adr/0016-vendor-desktop-and-app-menu.md)).

A base that ships a desktop of its own declares `gnome.keepsVendorDefaults: true`
instead of `gnome.dconfDatabase`; the rules are in
[core.md](resources/core.md#platform).
