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
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | none yet (destination `quay.io/deskos/deskos-core-centos10`) | Flatpak preinstall |
| AlmaLinux 10 | `quay.io/almalinuxorg/almalinux-bootc` (vendor, **experimental**) | x86_64 manifest, by digest | **validated** (CI) | **CI** | **CI**: GNOME; session check | none | `preinstall` via `rhel-family`; apps not verified |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` (vendor, entitled) | x86_64 manifest, by digest | **validated** (test) | built by the organization | built by the organization | **never** | Flatpak preinstall |
| Ubuntu 26.04 | `quay.io/deskos/deskos-ubuntu-base` (the DeskOS project, tier `organization`) | manifest, by digest | **validated** (golden test, CI) | **CI** | **CI**: GNOME; session check; first-boot check | none | **none** declared |
| Fedora 44 | `quay.io/fedora/fedora-silverblue` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **local build**, 2026-10-17 | not measured | none | Flatpak preinstall |

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

### Fedora 44 — no published image; build from the vendor's Silverblue base

Fedora is published by the Fedora Project, is subscription-free, and its bootc
base is its own, so this platform needs no entitlement and no organization-owned
base: it follows ADR 0004's official-base rule on
`quay.io/fedora/fedora-silverblue` — Fedora's *desktop* image, not the minimal
`quay.io/fedora/fedora-bootc`, which carries no GNOME, no GDM, no Flatpak and no
Plymouth at all (measured). The measured facts behind the platform —
`workstation-product-environment` as the comps environment dnf5 resolves, Flatpak
1.18.4 with `flatpak preinstall`, `/etc/pki/ca-trust/source/anchors` with
`update-ca-trust`, `gtk-update-icon-cache`, the bootc update units, GDM's
`/etc/gdm/custom.conf` — are in
[`resources/platforms/fedora-44.yaml`](../resources/platforms/fedora-44.yaml), one
comment per fact. [install.md](install.md#fedora-44) has the commands.

The artifact this platform would publish is the shipped **reference workstation**
`deskos-core-fedora44` ([`resources/workstations/`](../resources/workstations/)):
Core + `Profile/fedora` + the DeskOS look + the Flathub reference application,
under **its own image repository** — `quay.io/deskos/deskos-core-fedora44` — and
never a shared `:latest` tag, because a Fedora release must not be able to change
under a machine at `bootc upgrade` ([roadmap.md](roadmap.md) holds the plan for a
second Fedora release). Nothing is published for Fedora today: publishing is a
manual, human-gated step ([supply-chain.md](supply-chain.md)).

Composition and rendering for this platform are covered by CI: the reference
workstation lives in the embedded Core, so every `deskosctl validate` run in
`ci.yml` composes it, the Fedora golden pins its Containerfile, and
`ci.yml` renders it. What CI does **not** do for Fedora is build or boot an image
([vm-bootcheck.yml](../.github/workflows/vm-bootcheck.yml) covers CentOS Stream
10, AlmaLinux 10 and Ubuntu 26.04).

**Build evidence, measured 2026-10-17 on the pinned base.** Two images were built
locally, both on Fedora 44 and both finishing that day: the reference workstation
`deskos-core-fedora44` (image `5b44f2829d1e`, 07:17 +0200) and the example
`example-core-fedora-44` (image `c384826141cf`, 07:07 +0200). Each ran
`bootc container lint` (12 checks passed, 1 skipped, 2 warnings:
`nonempty-run-tmp`, `var-tmpfiles`) and each put the DeskOS Plymouth theme, with
its frames, into the rebuilt initramfs. **No Fedora 45 image was built** — 45's
numbers come from inspecting the pinned base and from
`dnf group install --assumeno`, not from a build. Neither image was installed.

What is **not** measured matters as much: **no Fedora disk has been booted**, so
there is no boot, session or first-boot evidence for this platform, and nothing is
published for it.

Two properties of this platform are decisions, and they are visible in the table
above rather than hidden:

- **The release is in the resource, the image name and the support window.** A
  second Fedora release is a new platform resource, a new re-pinned digest, a
  re-validation and a new image repository; retirements and re-pins are **manual**,
  as every other base pin is ([roadmap.md](roadmap.md)).
- **Fedora 45 is not offered.** When `:45` was measured it was a prerelease
  (`45.20261016.n.0 (Silverblue Prerelease)`) whose base enables Fedora's
  `updates-testing` repository, so DeskOS offers **44 only**. The 45 evaluation
  stays as an example-only root with its own Platform resource,
  [`examples/core-fedora-45/`](../examples/core-fedora-45/README.md): it is
  deliberately absent from the table above, has no reference workstation and no
  image, and exists to check the next release before anyone commits to it. It was
  **not built**.
