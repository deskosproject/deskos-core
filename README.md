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
  <a href="https://github.com/deskosproject/deskos-core/releases/latest"><img alt="release" src="https://img.shields.io/github/v/release/deskosproject/deskos-core?cacheSeconds=86400"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/github/license/deskosproject/deskos-core?cacheSeconds=86400"></a>
</p>

**DeskOS is an open workstation artifact factory.** An organization
describes its workstation as *declarative resources* (applications,
desktop settings, branding, boot appearance) and DeskOS **compiles** them
into a bootable container image for **CentOS Stream 10** or **RHEL 10**,
from which QCOW2 disks and installer ISOs are built.

> **A workstation is compiled from organizational intent.**

## The five-minute tour

An organization with more than a handful of Linux desktops ends up with
**snowflakes**: each machine was installed once, by hand, and then drifted.
Nobody can say what is on them, reproduce one, or prove one is safe to
ship. DeskOS takes the machine definition out of each installer's hands: you
**describe** the workstation in versioned text, and DeskOS **compiles** it
into one image — built, tested, signed and published like any other
artifact in your pipeline.

### The four words

- **Resource** — a YAML file describing one thing: a package set, a
  repository, a desktop setting, a theme, an update policy. There are
  **thirteen kinds**; each is small and composable. See
  [docs/resources.md](docs/resources.md).
- **Profile** — a named group of resources at a **layer**:
  `foundation` < `organization` < `role` < `workstation`. A higher layer
  overrides a lower one *on purpose*, never by file order.
- **Workstation** — the machine: a platform (CentOS Stream 10 or RHEL 10)
  plus the profiles it is made of. This is the file you author.
- **Plan, then render** — the compiler reads every resource, composes them
  into a typed **plan** (each value keeps the resource and layer it came
  from), and `render` writes a **deterministic** build context: the same
  inputs always give the same bytes.

### See it work

Get `deskosctl` and stay in the directory where you downloaded it (step 1 of
the [Quickstart](#quickstart)), then look at the workstation DeskOS Core
already ships — no files to write yet:

```bash
# Core is embedded: this composes cleanly with no resources of your own.
./deskosctl-$VERSION-linux-amd64 validate
# The composed machine, before anything is built.
./deskosctl-$VERSION-linux-amd64 plan --workstation deskos-core-centos10
```

`plan` prints what the image will contain and where each value came from.
To author your own machine, copy the
[example organization](https://github.com/deskosproject/deskos-core/tree/main/examples/baseline-and-role)
— a `Workstation` plus the profiles it is made of — and point `deskosctl` at
that directory. Building the image and a disk needs `podman` and root; that
is the rest of the [Quickstart](#quickstart).

## How it works

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/deskos-flow-dark.svg">
    <img alt="DeskOS flow: organizational intent as Workstation and Profiles in layers, deskosctl validates, composes, plans and renders a deterministic build context, podman builds a bootc image per platform, CentOS Stream 10 published to quay.io and RHEL 10 built privately, then bootc-image-builder makes QCOW2 disks and installer ISOs" src="docs/images/deskos-flow-light.svg" width="900">
  </picture>
</p>

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

A `Workstation` is the resource you author: a machine definition. It
compiles into a workstation **image** (a bootable container image). The
name is DeskOS's composition kind, not a Fedora Workstation edition.

## Quickstart

Compiling a workstation is five commands, and this is exactly what CI runs
on `main`:

```bash
# 1. Get deskosctl of one release. DeskOS Core is embedded in it, so there is
#    nothing else to download. (SHA256SUMS also lists the resources tar, kept
#    for inspection; --ignore-missing checks only what you fetched.)
VERSION=v0.9.2
base="https://github.com/deskosproject/deskos-core/releases/download/$VERSION"
curl -fL -O "$base/deskosctl-$VERSION-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
chmod +x "deskosctl-$VERSION-linux-amd64"

# 2. Validate Core, then read the composed workstation.
./deskosctl-$VERSION-linux-amd64 validate
./deskosctl-$VERSION-linux-amd64 plan --workstation deskos-core-centos10

# 3. Render a deterministic build context: the same inputs give the same
#    bytes, and generated-manifest.json records their SHA-256.
./deskosctl-$VERSION-linux-amd64 render \
    --workstation deskos-core-centos10 --output ctx

# 4. Build the bootable container image.
sudo podman build -t localhost/deskos-core-centos10:test ctx

# 5. Optional: a QCOW2 disk or an installer ISO from that image.
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args localhost/deskos-core-centos10:test
```

**This flow has already been run, and its result is published.** The
[supply-chain workflow](.github/workflows/supply-chain.yml) is manual
(`workflow_dispatch` with `publish=true`): it builds the image, scans it,
boots a QCOW2 made from the same image (`bootcheck.py`, `sessioncheck.py`),
then pushes and signs `quay.io/deskos/deskos-core` as `:latest` and
`:<commit>`. Pull the simple tag to learn; pin a digest in production:

```bash
sudo podman pull quay.io/deskos/deskos-core:latest
```

Compilation is deterministic — the rendered context is byte-identical for
the same inputs, and its manifest carries their SHA-256 — so anyone can
reproduce it from the same commit; the *build* is reproducible but not yet
bit-for-bit.

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="docs/images/deskos-composition-light.svg" width="900">
  </picture>
</p>

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
image; the digest to pull is in the [Quickstart](#quickstart) above.
Downloadable disks are not published yet: make a **QCOW2** or an
**installer ISO** from the image with `bootc-image-builder`, as described
in [**docs/install.md**](docs/install.md).

> [!IMPORTANT]
> **RHEL 10 images are never published.** Organizations with RHEL
> subscriptions build them privately and keep them inside the
> organization; see [docs/install.md](docs/install.md#rhel-10-private-builds-only).

## What DeskOS is not

- **Not a distribution with editions.** Organizations compose their own
  workstations from DeskOS Core, their baseline and their roles. A
  `Workstation` here is the machine you compose, not the Fedora
  Workstation edition and not a DeskOS edition.
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
| Published image | `quay.io/deskos/deskos-core` (`:latest` + `:<commit>`) | **never** |
| GNOME (pinned base) | Shell 49.5, mutter 49.4, schemas 47.1 | Shell 49.4, mutter 49.4, schemas 47.1 (RHEL 10.2) |

Details: [validation status](docs/architecture.md#validation-status).

## Documentation

By what you want to do:

| | |
|---|---|
| **Evaluate** | the tour above; [**Architecture**](docs/architecture.md) for the pipeline and design principles |
| **Create and install** | [**Installing DeskOS**](docs/install.md): QCOW2 and ISO from the published image; private RHEL 10 builds |
| **Define resources** | [**Resources**](docs/resources.md): every kind with an example; composition; software and GNOME model |
| **Develop** | [**Development**](docs/development.md): build `deskosctl`, run the tests, cut a release |
| **Audit the supply chain** | [**Supply chain**](docs/supply-chain.md): SBOM, scan, exceptions, signing; [ADRs](docs/adr/) |
| **Track** | [Roadmap](docs/roadmap.md) · [Research notes](docs/research-notes.md) |

## License

**Code:** [Apache License 2.0](LICENSE).
**DeskOS artwork** (`resources/assets/deskos/`, `docs/images/`): CC BY-SA
4.0; see [PROVENANCE.md](resources/assets/deskos/PROVENANCE.md).
