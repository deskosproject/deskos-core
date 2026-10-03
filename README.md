<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/deskos-lockup-dark.svg">
    <img alt="DeskOS" src="docs/images/deskos-lockup-light.svg" width="360">
  </picture>
</p>

<p align="center"><strong>Managed Linux workstations, built like infrastructure.</strong></p>

<p align="center">
  <a href="https://github.com/deskosproject/deskos-core/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/deskosproject/deskos-core/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/deskosproject/deskos-core/actions/workflows/vm-bootcheck.yml"><img alt="vm-bootcheck" src="https://github.com/deskosproject/deskos-core/actions/workflows/vm-bootcheck.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/deskosproject/deskos-core/releases/latest"><img alt="release" src="https://img.shields.io/github/v/release/deskosproject/deskos-core"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/github/license/deskosproject/deskos-core"></a>
</p>

DeskOS is an open workstation artifact factory. An organization describes
its workstation as declarative resources (applications, desktop
settings, branding, boot appearance) and DeskOS compiles them into a
bootable container image for CentOS Stream 10 or RHEL 10, from which QCOW2
disks and installer ISOs are built.

> A workstation is compiled from organizational intent.

## How it works

    resources ──▶ validation ──▶ composition ──▶ typed plan ──▶ Containerfile ──▶ bootc image ──▶ QCOW2 / ISO

A workstation names a platform and the profiles it is made of. Profiles sit
at explicit layers (`foundation < organization < role < workstation`), so
an organization overrides DeskOS Core defaults on purpose, never by file
order:

    apiVersion: core.deskos.org/v1alpha1
    kind: Workstation
    metadata:
      name: example-devops-centos10
    spec:
      platformRef: centos-stream-10
      profiles:
        - deskos-core        # foundation, shipped by DeskOS
        - example-baseline   # organization
        - example-devops     # role

    apiVersion: desktop.deskos.org/v1alpha1
    kind: GnomeProfile
    metadata:
      name: example-desktop
    spec:
      defaults:
        appearance:
          wallpaper:
            light: ../assets/example-org.svg
        session:
          idle:
            blankAfter: 5m
        dock:
          position: bottom

`deskosctl plan` shows the composed workstation and which layer won each
setting; `deskosctl render` writes a deterministic build context. Every
resource kind, with an example, is in [docs/resources.md](docs/resources.md).

## Get DeskOS

DeskOS Core for CentOS Stream 10 is published as a bootable container
image, the one that passed the boot and session checks:

    sudo podman pull quay.io/deskos/deskos-core:latest

Downloadable disks are not published yet; make a QCOW2 or an installer ISO
from the image with bootc-image-builder as described in
[docs/install.md](docs/install.md). RHEL 10 images are never published:
organizations with subscriptions build them privately, as described there.

## What DeskOS is not

- Not a distribution with editions: organizations compose their own
  workstations from DeskOS Core, their baseline and their roles.
- Not configuration management: DeskOS owns the image, not the running
  machine. Ansible, Satellite, MDM and EDR keep managing endpoints.
- Not a developer environment manager: personal tooling (Homebrew, mise,
  dotfiles, Toolbox contents) stays outside the managed baseline.

## Status

Experimental; the resource APIs are `v1alpha1` and will change. Composition
and rendering are deterministic. The CentOS Stream 10 image builds, boots
to GNOME and passes the session checks in CI; RHEL 10 builds pass the same
checks on an entitled factory host. Details:
[validation status](docs/architecture.md#validation-status).

## Documentation

- [Installing DeskOS](docs/install.md): QCOW2 and ISO from the published
  image, private RHEL 10 builds.
- [Resources](docs/resources.md): resource kinds, composition layers,
  software and GNOME model.
- [Architecture](docs/architecture.md), [ADRs](docs/adr/),
  [roadmap](docs/roadmap.md), [research notes](docs/research-notes.md).
- [Development](docs/development.md): building `deskosctl` and the image,
  tests, releases.

## License

Code: [Apache License 2.0](LICENSE). DeskOS artwork
(`resources/assets/deskos/`, `docs/images/`): CC BY-SA 4.0, see
[PROVENANCE.md](resources/assets/deskos/PROVENANCE.md).
