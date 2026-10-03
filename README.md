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

**DeskOS is an open workstation artifact factory.** An organization
describes its workstation as *declarative resources* (applications,
desktop settings, branding, boot appearance) and DeskOS **compiles** them
into a bootable container image for **CentOS Stream 10** or **RHEL 10**,
from which QCOW2 disks and installer ISOs are built.

> **A workstation is compiled from organizational intent.**

## How it works

    resources ──▶ validation ──▶ composition ──▶ typed plan ──▶ Containerfile ──▶ bootc image ──▶ QCOW2 / ISO

A `Workstation` names a **platform** and the **profiles** it is made of:

```yaml
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
```

`deskosctl plan` shows the composed workstation *before anything is
built*, and `deskosctl render` writes a **deterministic** build context:
the same inputs always give the same bytes. Every resource kind, with an
example, is in [**docs/resources.md**](docs/resources.md).

## Composition

Profiles sit at explicit layers,
`foundation` < `organization` < `role` < `workstation`, so an
organization overrides DeskOS Core **on purpose**, never by file order.
Here the organization shortens Core's screen blank from `10m` to `5m`:

```yaml
apiVersion: desktop.deskos.org/v1alpha1
kind: GnomeProfile
metadata:
  name: example-desktop      # included by example-baseline (organization)
spec:
  defaults:
    session:
      idle:
        blankAfter: 5m       # wins over Core's 10m
    dock:
      position: bottom
```

- **Sets** (packages, units) are unioned.
- **Scalars** (GNOME settings) take the highest layer; two different
  values *at the same layer* are an error.
- **Keyed** definitions (repositories, Flatpaks, binaries) must agree.

Every value in the plan keeps its *provenance*: the resource, layer and
file it came from.

## Get DeskOS

DeskOS Core for CentOS Stream 10 is published as a bootable container
image; it is **the image that passed the boot and session checks**:

```bash
sudo podman pull quay.io/deskos/deskos-core:latest
```

Downloadable disks are not published yet. Make a **QCOW2** or an
**installer ISO** from the image with `bootc-image-builder`, as described
in [**docs/install.md**](docs/install.md).

> [!IMPORTANT]
> **RHEL 10 images are never published.** Organizations with RHEL
> subscriptions build them privately and keep them inside the
> organization; see [docs/install.md](docs/install.md#rhel-10-private-builds-only).

## What DeskOS is not

- **Not a distribution with editions.** Organizations compose their own
  workstations from DeskOS Core, their baseline and their roles.
- **Not configuration management.** DeskOS owns the *image*, not the
  running machine; Ansible, Satellite, MDM and EDR keep managing
  endpoints.
- **Not a developer environment manager.** Personal tooling (Homebrew,
  mise, dotfiles, Toolbox contents) stays outside the managed baseline.

## Status

> [!NOTE]
> **Experimental.** The resource APIs are `v1alpha1` and will change.

| | CentOS Stream 10 | RHEL 10 |
|---|---|---|
| Compose and render | deterministic | deterministic |
| Image build, `bootc container lint` | CI | entitled factory host |
| Boot to GNOME, session checks | CI (`vm-bootcheck`) | factory host (`deskos/rhel10` status) |
| Published image | `quay.io/deskos/deskos-core` | **never** |

Details: [validation status](docs/architecture.md#validation-status).

## Documentation

| | |
|---|---|
| [**Installing DeskOS**](docs/install.md) | QCOW2 and ISO from the published image; private RHEL 10 builds |
| [**Resources**](docs/resources.md) | every resource kind with an example; composition; software and GNOME model |
| [**Architecture**](docs/architecture.md) | design principles, compiler pipeline, validation status |
| [**Development**](docs/development.md) | building `deskosctl` and the image, tests, releases |
| [ADRs](docs/adr/) · [Roadmap](docs/roadmap.md) · [Research notes](docs/research-notes.md) | decisions, milestones, upstream facts |

## License

**Code:** [Apache License 2.0](LICENSE).
**DeskOS artwork** (`resources/assets/deskos/`, `docs/images/`): CC BY-SA
4.0; see [PROVENANCE.md](resources/assets/deskos/PROVENANCE.md).
