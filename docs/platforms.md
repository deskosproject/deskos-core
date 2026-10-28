# Platforms

The canonical status of every platform DeskOS compiles onto, and how to get a
machine from each **today**. Every other page links here instead of repeating a
matrix; when a platform changes, this is the one table to edit.

DeskOS compiles the same Core intent onto a platform's own bootc base
([ADR 0004](adr/0004-core-is-platform-independent.md)). A platform whose vendor
publishes no bootc base may be added as an organization-owned base with a named
owner and a declared tier ([ADR 0014](adr/0014-platform-base-ownership.md)) —
Ubuntu 26.04 is that case, and it is built and published in a repository of its own,
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu).

## Status

| Platform | Base (owner) | Base pinned | Composes and renders | Image built | Booted and checked | Published image | System Flatpak |
|---|---|---|---|---|---|---|---|
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | **published** (`quay.io/deskos/deskos-core-centos-stream-10`) | Flatpak preinstall |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` (vendor, entitled) | x86_64 manifest, by digest | **validated** (test) | built by the organization | built by the organization | **never** | Flatpak preinstall |

`(vendor)` in the table means the base is the platform's own supplier's: Red Hat
for RHEL, the CentOS project for CentOS Stream. The distinction matters beyond
the base: the desktop a session starts with is the vendor's on a platform that
declares `gnome.keepsVendorDefaults`, and DeskOS's own where it does not
(ADR 0016).

"Composes and renders" means the workstation compiles to a deterministic plan
and build context. It does **not** mean the image builds or boots — that is the
next two columns. Evidence, per claim, is in
[architecture.md](architecture.md#validation-status) and the run artifacts.

AlmaLinux 10 and Fedora 45 are **example platforms**, not shipped platforms. They
are not in `resources/platforms/`, are not published, and do not block a pull
request. They still compose and render in CI and are built and booted by a
weekly example workflow. To build one locally or re-promote it, see
[Example platforms](example-platforms.md).

## How to get a machine today

### CentOS Stream 10 — pull the image, then install

The image is published at `quay.io/deskos/deskos-core-centos-stream-10` as
`:latest` and `:<commit>` by the weekly gate, with its digest
and the run in [architecture.md](architecture.md#validation-status). A preview
ISO built from a published image is at <https://download.deskos.org/>; verify its
checksum. Or make a QCOW2 (or an Anaconda ISO) from the image with
`bootc-image-builder`: [install.md](install.md#install-a-published-image).

### RHEL 10 — build privately, never publish

An organization with RHEL subscriptions builds its own from the entitled base
and keeps it inside the organization; the RHEL EULA forbids redistributing it.
[install.md](install.md#rhel-10-private-builds-only).

### Ubuntu 26.04 — built and published in its own repository

Ubuntu has no bootc base from its vendor, so the project owns one, and both the
base and the workstation that builds on it live in a repository of their own:
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu),
on base `deskosproject/deskos-ubuntu-bootc`. That repository documents how to
build it, how to write a disk with `bootc install to-disk` — there is no Anaconda
path on this platform — and what is worth testing
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

This repository still **composes and renders** a Debian-family workstation, and
its tests say so. It no longer builds, boots or publishes one: the virtual
machines, the publish leg and the platform's resources moved with it.

## Support and retirement

The matrix above is the support statement. It is **not an LTS promise**, and a
vendor's lifecycle dates do not become DeskOS dates.

- **Maintained artifacts today:** CentOS Stream 10, published as a Core image;
  RHEL 10, built privately by an organization that owns the entitlement; Ubuntu
  26.04, built and published in its own repository. AlmaLinux 10 and Fedora 45
  are examples: composed and built in CI, but not supported artifacts.
- **Security maintenance** is a deliberate re-pin of the vendor base followed by
  the image build, scan and boot checks. DeskOS does not backport fixes to older
  image digests, and a clean scan is not a claim that an image is secure
  ([ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md)).
- **Hardware evidence** is claimed only where
  [architecture.md](architecture.md#validation-status) records it. "Booted and
  checked" in the matrix is virtual-machine evidence unless that page says
  otherwise; bare metal is not implied.
- **Upgrade paths:** within a platform, a machine moves forward with `bootc
  upgrade` and back with the previous deployment. Moving between platforms is a
  reinstall, not an upgrade; the image reference is the boundary.
- **Losing a maintainer or a vendor base:** the platform stops being maintained.
  It moves to an example root or is removed; it does not silently keep the
  "published" status. A new maintainer or base re-enters through the
  re-promotion steps below.
- **Consumer notice:** a status change lands in this file and in the release
  notes. There is no telemetry and no automatic fleet action; a machine changes
  only when its update reference or its deployment changes.
- **Retiring a platform** means removing it from `resources/platforms/`, the
  publish matrix and the image legs of CI. Its example may stay in `examples/`
  as an example, non-published root.
- **Re-promoting an example platform** follows
  [example-platforms.md](example-platforms.md): make the base durable, move the
  Platform back, add the reference workstation and CI/publish legs, regenerate
  goldens, then validate, build, boot and only then publish.
