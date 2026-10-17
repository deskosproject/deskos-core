# Platforms

The canonical status of every platform DeskOS compiles onto, and how to get a
machine from each **today**. Every other page links here instead of repeating a
matrix; when a platform changes, this is the one table to edit.

DeskOS compiles the same Core intent onto a platform's own bootc base
([ADR 0004](adr/0004-core-is-platform-independent.md)). A platform whose vendor
publishes no bootc base may be added as an organization-owned base with a named
owner and a declared tier ([ADR 0014](adr/0014-platform-base-ownership.md)) —
Ubuntu is that case. DeskOS **publishes** only CentOS Stream 10.

## Status

| Platform | Base (owner) | Base pinned | Composes and renders | Image built | Booted and checked | Published image | System Flatpak |
|---|---|---|---|---|---|---|---|
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | `quay.io/deskos/deskos-core` | Flatpak preinstall |
| AlmaLinux 10 | `quay.io/almalinuxorg/almalinux-bootc` (vendor, **experimental**) | x86_64 manifest, by digest | **validated** (CI) | **CI** | **CI**: GNOME; session check | none | `preinstall` via `rhel-family`; apps not verified |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` (vendor, entitled) | x86_64 manifest, by digest | **validated** (test) | built by the organization | built by the organization | **never** | Flatpak preinstall |
| Ubuntu 26.04 | `quay.io/deskos/deskos-ubuntu-base` (the DeskOS project, tier `organization`) | manifest, by digest | **validated** (golden test, CI) | **CI** | **CI**: GNOME Initial Setup; first-boot check; no session check | none | **none** declared |

"Composes and renders" means the workstation compiles to a deterministic plan and
build context. It does **not** mean the image builds or boots — that is the next
two columns. Evidence, per claim, is in
[architecture.md](architecture.md#validation-status) and the run artifacts.

## How to get a machine today

### CentOS Stream 10 — install from the published image

This is the only platform DeskOS publishes. Make a QCOW2 (or an Anaconda ISO)
from the published image with `bootc-image-builder`:
[install.md](install.md#centos-stream-10).

### RHEL 10 — build privately, never publish

An organization with RHEL subscriptions builds its own from the entitled base
and keeps it inside the organization; the RHEL EULA forbids redistributing it.
[install.md](install.md#rhel-10-private-builds-only).

### AlmaLinux 10 — no published image; built and booted in CI

AlmaLinux is a public, subscription-free EL10 base, so a build needs no
entitlement. CI builds and boots a derived AlmaLinux image
(`example-core-almalinux-10`) and runs the boot, session and first-boot checks on
it. That exercises the paths the example's profiles need — its package group, the
Plymouth packages, the GNOME facts — not every capability a platform declares:
the example declares no `FlatpakSet` or `TrustAnchor`, and does not test the base
updater. The base is the vendor's, and it is **experimental**. DeskOS publishes no
AlmaLinux image: to make your own, the render, build and QCOW2 steps are the same
shape as CentOS Stream 10 ([install.md](install.md#almalinux-10)).

### Ubuntu 26.04 — no published image and no DeskOS ISO; write a disk directly

DeskOS publishes no Ubuntu image and implements no installer ISO for it: there is
no ISO path here, built or validated. What **is** validated is
`bootc install to-disk`. The honest path for a reader who wants an installed
Ubuntu machine today:

1. **Build the image** from a resource root (for example
   `examples/core-ubuntu-2604/`) and tag it with the reference your machines will
   update from:
   [install.md](install.md#ubuntu-2604).
2. **Write a disk** with `bootc install to-disk` from a compressed OCI layout —
   the recipe is proven in
   [`tests/vm/README.md`](../tests/vm/README.md#building-a-test-disk).

Three things that are easy to conflate and must not be:

- **A lab disk is not a delivery.** The CI job writes a raw disk by installing
  from a local OCI layout with **no `--target-imgref`**
  (`.github/workflows/vm-bootcheck.yml`); that proves the image builds, installs
  and boots, and nothing about where a machine updates from afterwards.
- **A real install points `--target-imgref` at a derivative you publish** (a
  private registry, or one you make public), never at the local layout and never
  at the Ubuntu base — pointing it at the base would replace your derivative with
  the base at the next update.
- **Publishing the derivative is the operator's step, untested by CI.** DeskOS
  publishes no Ubuntu image; a green CI run says nothing about a registry you
  publish to.

See [install.md](install.md#ubuntu-2604) for the commands, and
[operations.md](operations.md) for how the installed machine updates.
