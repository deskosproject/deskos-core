# DeskOS documentation

New here? Start with the [README](../README.md), then
[Platforms](platforms.md) for what each platform can do today.

## The words this repository uses

They are few, and most confusion comes from two of them meaning more than one thing.

- **Core**: what this repository ships — the compiler (`deskosctl`), the resource model, the
  profiles and reference workstations a flavor builds on.
- **Platform**: an operating-system target and the facts that come with it. One resource per
  platform (`centos-stream-10`, `ubuntu-2604`), holding its base image, its package manager,
  its package groups, its boot splash, and what it can and cannot do. Everything that is
  specific to one platform lives in its `Platform` resource and nowhere else.
- **Base image**: the OS image a platform starts `FROM` — the vendor's own
  (`quay.io/centos-bootc/centos-bootc`) or, where the vendor publishes no bootc base, one
  the project owns and supports itself (ADR 0014).
- **Vendor**: the supplier of that base — Canonical for Ubuntu, Red Hat for RHEL, the CentOS
  and AlmaLinux projects, the Fedora Project. A **software** vendor whose repository an
  organization installs from is a different party and a different decision; the Ubuntu
  example is one of those.
- **Workstation**: the thing a person installs. One platform plus the profiles to select.
  `deskos-core-<platform>` are the ones this repository ships; `example-<platform>` are the
  examples that show how they are put together.
- **Profile**: a layer of intent — `foundation`, `organization`, `role`, `workstation`. A
  profile selects resources; it does not describe a machine on its own.
- **Baseline**: the boundary of what DeskOS manages at all — machine-wide software an
  organization requires for any authorized user, complete before any user's home directory
  exists (ADR 0005). It is **not** a layer and not a file: what an organization's own layer
  carries is a profile with layer `organization`, and the example that shows it is
  [`examples/organization-and-role/`](../examples/organization-and-role/).
- **Resource**: the declarative unit you write, one of fourteen kinds ([Resources](resources.md)
  lists them all with an example).
- **Desktop**: the GNOME part of a workstation, and the word covers three things that are
  easy to mix up: the **session** (what starts, what the user sees), its **defaults** (dconf
  and GSettings values, `GnomeProfile`), and its **look** (wallpaper, login logo, icons,
  extensions, `Theme` and the `deskos-look` profile). It is also a *platform fact*: a
  platform that declares `gnome.keepsVendorDefaults` keeps the desktop its vendor ships, and
  DeskOS writes no default there (ADR 0016). Apps are neither: they are `PackageSet`, a
  Flatpak remote, or a signed vendor repository.
- **Flavor**: an organization's own resource root, built on Core's profiles and styles —
  its own package sources, its own desktop choices, its own workstations. It references
  Core's resources by name and copies none of them.
- **Artifact**: what composition and rendering produce — a deterministic **plan** and a
  build **context**. The image a `podman build` makes from that context is the thing that
  boots.

| Page | What it is |
|---|---|
| [Platforms](platforms.md) | **The canonical matrix**: base owner, what is verified, and how to install each platform |
| [Installing DeskOS](install.md) | QCOW2, the Anaconda ISO, private RHEL builds, AlmaLinux, Fedora 44, and the Ubuntu `bootc install to-disk` path |
| [Architecture](architecture.md) | The pipeline and boundary, the resource model, composition, the Plan, the Containerfile backend, and the evidence behind the claims |
| [Resources](resources.md) | Every one of the fourteen kinds, with an example |
| [Operations](operations.md) | Updates, inspect, apply, roll back, change the update source |
| [Supply chain](supply-chain.md) | SBOM, scan, exceptions, signing, verifying a digest |
| [Development](development.md) | Build `deskosctl`, run the tests, cut a release |
| [Installer ISO](installer-iso.md) | The Anaconda kickstart, and the warning that it erases every disk |
| [Design: visual identity and boot](design/visual-identity-and-boot.md) | Branding, splash, first-boot appearance |
| [VM harness](../tests/vm/README.md) | The boot, session and first-boot checks, and their transport limits |
| [Roadmap](roadmap.md) · [Research notes](research-notes.md) | What is done, planned, and measured — the notes are a dated archive |
| [ADRs](adr/README.md) | The decisions behind all of it |
