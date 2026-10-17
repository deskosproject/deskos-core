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
| Fedora 44 | `quay.io/fedora/fedora-silverblue` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **local build** (2026-10-17), not in CI | not measured | none | Flatpak preinstall |
| Fedora 45 | `quay.io/fedora/fedora-silverblue` (vendor, **prerelease** on 2026-10-17) | x86_64 manifest, by digest | **validated** (test, CI) | not built | not measured | none | Flatpak preinstall |

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

### Fedora 44 and 45 — no published image; build from the vendor's Silverblue base

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
[`resources/platforms/fedora-44.yaml`](../resources/platforms/fedora-44.yaml) and
[`fedora-45.yaml`](../resources/platforms/fedora-45.yaml), one comment per fact.
[install.md](install.md#fedora-44-and-45) has the commands.

The artifact this family would publish is the shipped **reference workstation**,
`deskos-core-fedora44` / `deskos-core-fedora45` in
[`resources/workstations/`](../resources/workstations/) (Core + `Profile/fedora` +
the DeskOS look + the Flathub reference application), with **one image repository
per platform and release** — `quay.io/deskos/deskos-core-fedora44`,
`quay.io/deskos/deskos-core-fedora45` — and never a shared `:latest` tag, because a
Fedora release must not be able to change under a machine at `bootc upgrade`.
`examples/core-fedora-44/` and `examples/core-fedora-45/` remain the minimal
examples. Nothing is published for Fedora today: publishing is a manual,
human-gated step ([supply-chain.md](supply-chain.md)).

Composition and rendering for this family are covered by CI without touching a
workflow: the two reference workstations live in the embedded Core, so every
`deskosctl validate` run in `ci.yml` composes them, and `make validate` also names
the two example roots. What CI does **not** do for Fedora is build or boot an
image ([vm-bootcheck.yml](../.github/workflows/vm-bootcheck.yml) covers CentOS
Stream 10 and Ubuntu 26.04).

What is **not** measured matters as much: DeskOS has built the Fedora image from
the pinned base but has **not booted** one, so there is no boot, session or
first-boot evidence for this family, and no published image.

Two properties of this family are decisions, and they are visible in the table
above rather than hidden:

- **The release is in the resource, the image name and the support window.** The
  support policy is the **current Fedora release plus the previous one**: the
  reference workstation is what would be published, each release with its own
  image repository, and at each Fedora release the base digest is re-pinned and
  the platform re-validated, with the older platform resource retired then. Both
  steps are **manual**, as every other base pin is. On 2026-10-17 that pair is
  `fedora-44` and `fedora-45`.
- **`:45` was a prerelease when it was pinned** and its base enables Fedora's
  `updates-testing` repository (measured: `os-release` reads
  `45.20261016.n.0 (Silverblue Prerelease)`, and packages in the workstation
  transaction did come from `updates-testing`). `:44` is released and does not.
  Whether DeskOS should carry a prerelease at all is a human decision; it is
  recorded here rather than smoothed over.
