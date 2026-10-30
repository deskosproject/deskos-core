# Platforms

The status of every platform DeskOS compiles onto, and how to get a machine from
each **today**. When a platform changes, this is the one table to edit.

DeskOS compiles the same Core intent onto a platform's own bootc base
([ADR 0004](adr/0004-core-is-platform-independent.md)). A vendor with no bootc
base may be added as an organization-owned base with a named owner and a declared
tier ([ADR 0014](adr/0014-platform-base-ownership.md)). Ubuntu 26.04 is that case,
in [`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu).

## Status

| Platform | Base (owner) | Base pinned | Composes and renders | Image built | Booted and checked | Published image | System Flatpak |
|---|---|---|---|---|---|---|---|
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | **published** (`quay.io/deskos/deskos-core-centos-stream-10`) | Flatpak preinstall |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` (vendor, entitled) | x86_64 manifest, by digest | **validated** (test) | built by the organization | built by the organization | **never** | Flatpak preinstall |
| Fedora 45 | `quay.io/deskos/fedora-silverblue` (vendor, mirrored) | x86_64 index, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | not yet | Flatpak preinstall |

`(vendor)` means the platform's own supplier: Red Hat for RHEL, the CentOS
project for CentOS Stream. A session starts with the vendor's desktop where
`gnome.keepsVendorDefaults` is declared, DeskOS's own where it is not (ADR 0016).

"Composes and renders" is a deterministic plan and build context, not a built
image. Records are in [architecture.md](architecture.md#validation-status).

Ubuntu 26.04 is not a row here: no vendor publishes a bootc base for it, so the
project owns one, and its build, boot and publication live in
[`deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu) (ADR 0020).

AlmaLinux 10 is an **example platform**: not in `resources/platforms/`, not
published, not blocking a pull request. CI composes and renders it and a weekly
workflow builds and boots it; see [Example platforms](example-platforms.md).
Fedora 45 moved into the matrix in 2026-10-30; its base, mirror and pin are in
[Fedora 45](fedora.md).

## How to get a machine today

### CentOS Stream 10: pull the image, then install

The image is published at `quay.io/deskos/deskos-core-centos-stream-10` as
`:latest` and `:<commit>` by the weekly build, with its digest and run in
[architecture.md](architecture.md#validation-status). A preview ISO from a
published image is at <https://download.deskos.org/> (check its checksum); for a
QCOW2 or Anaconda ISO use `bootc-image-builder`:
[install.md](install.md#option-1-install-the-published-image).

### RHEL 10: build privately, never publish

An organization with RHEL subscriptions builds its own from the entitled base
and keeps it inside the organization; the RHEL EULA forbids redistributing it.
[install.md](install.md#rhel-10-private-builds-only).

### Ubuntu 26.04: built and published in its own repository

Ubuntu has no bootc base from its vendor, so the project owns one: base and
workstation live in
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu)
on `deskosproject/deskos-ubuntu-bootc`. It documents the build, a disk written
with `bootc install to-disk` (no Anaconda path), and what is worth testing
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

This repository still **composes and renders** a Debian-family workstation, and
its tests cover that. It no longer builds, boots or publishes one.

## Why GNOME

A desktop is adopted here on
[seven criteria](adr/0024-what-a-desktop-platform-must-expose.md): file-based
configuration with precedence, a lock layer, a greeter database, a first-boot setup
that can be skipped or preconfigured, an extension policy the image decides, an
offline expression, and state a check can read. Popularity is not one of them.
GNOME on EL10 meets all seven, and [the audit](gnome-coverage.md) records it:
nineteen guide items modelled exactly, one partially, the refusals named. Another desktop needs a kind, an ADR
and an audit, not a flag.

## Support and retirement

The matrix above is the support statement. It is **not an LTS promise**, and a
vendor's lifecycle dates do not become DeskOS dates.

- **Maintained artifacts today:** CentOS Stream 10, published as a Core image;
  RHEL 10, built privately under an entitlement; Ubuntu 26.04, in its own repo;
  the examples are built in CI, not supported.
- **Security maintenance** is a re-pin of the vendor base, then build, scan and
  boot checks. DeskOS does not backport fixes to older digests; a clean scan does
  not mean an image is secure
  ([ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md)).
- **Hardware records** are only those in
  [architecture.md](architecture.md#validation-status); "Booted and checked"
  means a virtual machine, not bare metal.
- **Upgrade paths:** `bootc upgrade` moves a machine forward, the previous
  deployment back; across platforms it is a reinstall, not an upgrade.
- **Losing a maintainer or a vendor base** retires the platform: it leaves
  `resources/platforms/`, the publish matrix and the CI image legs, and moves to
  an example root or is removed. It never quietly keeps "published". Re-promotion
  follows [example-platforms.md](example-platforms.md#re-promoting-an-example-platform).
- **Consumer notice:** a status change lands here and in the release notes. No
  telemetry, no fleet action; a machine changes only with its update reference.
