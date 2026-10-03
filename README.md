# DeskOS

**Managed Linux workstations, built like infrastructure.**

DeskOS is an open workstation artifact factory.

It turns organizational workstation requirements into reproducible
workstation definitions and testable Linux workstation artifacts.

The core idea is simple:

> **A workstation is compiled from organizational intent.**

DeskOS is not another catalog of Linux distribution flavors, and it is
not a configuration-management system.

A Linux distribution provides an operating system. An organization still
has to define its workstation: desktop behavior, applications, security
software, branding, developer capabilities, identity prerequisites,
trusted content, and lifecycle expectations.

DeskOS provides a declarative resource model and compiler for building
that artifact.

## Status

DeskOS is currently an experimental architectural implementation.

The first milestone is deliberately narrow: prove that one semantic
DeskOS Core definition can be composed onto both CentOS Stream 10 and
RHEL 10, while allowing an organization to add its own baseline and
role-specific requirements.

The APIs are currently `v1alpha1` and will change.

What has been shown so far:

- Loading, composition, the Plan and rendering are deterministic:
  identical inputs give byte-identical plans and build contexts. Built
  images are not yet bit-for-bit reproducible, because base image tags,
  repository metadata, vendor packages and remote GPG keys can still move.
- The same DeskOS Core composes and renders for CentOS Stream 10 and
  RHEL 10.
- The CentOS Stream 10 contexts build with Podman and pass `bootc container
  lint` (with non-fatal warnings from distribution packages). A disk of
  the reference image passes `tests/vm/bootcheck.py`, a local QEMU/UEFI
  boot check: the pixel classifier sees the graphical splash, then a
  stable graphical screen, then an ACPI power-off. The captured frames
  show the DeskOS splash and GNOME Initial Setup. The check does not
  identify screens or test applications. `tests/vm/sessioncheck.py`, an
  instrumented boot of the same disk with a test user, checks the GNOME
  session against the Plan (Dash to Dock active, favorites, wallpaper,
  Firefox headless, failed units). In AMD QEMU guests `mcelog.service`
  fails because `edac_mce_amd` is not loaded; the check reports that
  exact signature as a known, non-blocking diagnostic and keeps the raw
  failed state. A manual GitHub workflow runs the same build, boot and
  session checks; it is not a full E2E test.

RHEL 10: `example-devops-rhel10`, built on an entitled factory VM from a
digest-pinned RHEL 10.2 base, passes `bootc container lint` with every
layer free of build-host subscription state, and its QCOW2 passes
`bootcheck.py` and `sessioncheck.py` there under nested KVM. These checks
run manually; there is no RHEL CI and no E2E test. See
[docs/architecture.md](docs/architecture.md#validation-status).

License: [Apache License 2.0](LICENSE).

## What DeskOS builds

Conceptually:

    organizational intent
            |
            v
      DeskOS resources
            |
            v
        validation
            |
            v
        composition
            |
            v
         typed plan
            |
            v
         compiler
            |
            v
        bootc OCI image
          /         \
       QCOW2        ISO

The OCI image is the primary operating-system artifact.

Disk images and installation media are derived from it.

## What DeskOS does not do

DeskOS does not continuously reconcile deployed endpoints.

It is not a replacement for Ansible Automation Platform, Satellite,
Foreman, endpoint management, EDR, or other runtime fleet-management
systems.

DeskOS builds the baseline those systems operate on.

A deployed workstation may contain mutable state, and runtime drift is a
separate operational concern.

## Getting DeskOS

Downloadable QCOW2 and ISO images are not published yet. Until they are,
create them from the container image with bootc-image-builder. You need
Linux with rootful Podman and about 20 GB free.

### CentOS Stream 10

DeskOS Core for CentOS Stream 10 is published as a bootable container
image. Each published image is the one that passed the boot and session
checks for that commit; it is tagged with the commit ID and `latest`:

    sudo podman pull quay.io/deskos/deskos-core:latest

A QCOW2 disk for a virtual machine:

    mkdir -p output
    sudo podman run --rm -it --privileged --pull=missing \
        --security-opt label=type:unconfined_t \
        -v ./output:/output \
        -v /var/lib/containers/storage:/var/lib/containers/storage \
        quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
        build --type qcow2 --no-default-kernel-args \
        --chown "$(id -u):$(id -g)" \
        quay.io/deskos/deskos-core:latest

The disk is `output/qcow2/disk.qcow2` (10.5 GiB virtual by default). It
has no user account: GNOME Initial Setup creates the first one at first
boot.
`--no-default-kernel-args` keeps the image's own boot arguments (quiet
graphical splash) instead of the builder's serial console. For a larger
root filesystem, mount a `config.toml` (`-v ./config.toml:/config.toml:ro`
and `--config /config.toml` after `build`) containing:

    [[customizations.filesystem]]
    mountpoint = "/"
    minsize = "40 GiB"

An installer ISO uses `--type anaconda-iso` with a kickstart in
`config.toml`; the kickstart DeskOS needs, and the warning that it erases
every disk, are in [docs/installer-iso.md](docs/installer-iso.md).
Machines installed from it update from the image reference it was built
from (`quay.io/deskos/deskos-core:latest`) with `bootc upgrade`.

### RHEL 10 (private builds only)

DeskOS publishes only CentOS Stream 10 artifacts. RHEL-based images, disks
and ISOs must never be published: the RHEL EULA forbids public
redistribution. An organization with RHEL subscriptions builds its own,
on a registered RHEL 10 host logged in to `registry.redhat.io` (as root,
since the build runs as root), and keeps the result inside the
organization.

Get the DeskOS resources and build `deskosctl` from source (Go 1.26):

    git clone https://github.com/deskosproject/deskos-core.git
    cd deskos-core
    go build -o bin/deskosctl ./cmd/deskosctl

Declare a RHEL 10 workstation in a resource root of your own, for example
`my-org/workstations/core-rhel10.yaml`:

    apiVersion: core.deskos.org/v1alpha1
    kind: Workstation
    metadata:
      name: deskos-core-rhel10
    spec:
      displayName: DeskOS Core (RHEL 10)
      platformRef: rhel-10
      profiles:
        - deskos-core

Render and build it. Tag it with the reference your machines should update
from, in a registry that is private to your organization:

    ./bin/deskosctl render ./resources ./my-org \
        --workstation deskos-core-rhel10 --output ./dist/deskos-core-rhel10
    sudo podman build -t registry.example.internal/deskos/core-rhel10:latest \
        ./dist/deskos-core-rhel10

Then create the QCOW2 (or the ISO, as above) with the RHEL builder:

    mkdir -p output
    sudo podman run --rm -it --privileged --pull=missing \
        --security-opt label=type:unconfined_t \
        -v ./output:/output \
        -v /var/lib/containers/storage:/var/lib/containers/storage \
        registry.redhat.io/rhel10/bootc-image-builder@sha256:7f5baead2d4ac2a1035900ced31e4e7600fc98f69aa45ee5d05639bca028e00b \
        build --type qcow2 --no-default-kernel-args \
        --chown "$(id -u):$(id -g)" \
        registry.example.internal/deskos/core-rhel10:latest

Every layer of the build is free of the build host's subscription state
(see [docs/architecture.md](docs/architecture.md#validation-status)), but
the image, disks and ISOs are still RHEL derivatives: push them only to
registries and storage that are private to your organization.

## Resource model

DeskOS configuration uses versioned, Kubernetes-inspired resource
envelopes:

    apiVersion: core.deskos.org/v1alpha1
    kind: Workstation
    metadata:
      name: acme-developer-rhel10
    spec:
      platformRef: rhel-10
      profiles:
        - deskos-core
        - acme-baseline
        - acme-developer

DeskOS borrows the useful API ideas (Group, Version, Kind, schemas and
composition) without requiring Kubernetes.

The v1alpha1 kinds are `Platform`, `Profile` and `Workstation`
(`core.deskos.org`); `PackageSet`, `RpmRepository`, `BinaryArtifact`,
`FlatpakRemote` and `FlatpakSet` (`software.deskos.org`);
`GnomeProfile` (`desktop.deskos.org`); and `BootProfile`
(`system.deskos.org`). Their JSON Schemas are in
[`schemas/`](schemas/).

### Examples

Resources live in YAML files under one or more resource roots. DeskOS
ships `./resources`; an organization keeps its own root and references
DeskOS resources by kind and name without copying them. File and directory
names have no meaning, and asset paths are relative to the YAML file. All
examples below are files from this repository
([`resources/`](resources/) and [`examples/example-org/`](examples/example-org/)).

A Profile places resources at one semantic layer:

    apiVersion: core.deskos.org/v1alpha1
    kind: Profile
    metadata:
      name: example-baseline
    spec:
      layer: organization
      description: Example organization baseline for every workstation.
      resources:
        - kind: PackageSet
          name: example-baseline
        - kind: GnomeProfile
          name: example-desktop

Packages, RPM groups (named by the Platform) and units to enable:

    apiVersion: software.deskos.org/v1alpha1
    kind: PackageSet
    metadata:
      name: deskos-core
    spec:
      groups:
        - workstation
      packages:
        - bootc
        - NetworkManager
        - firefox
        - flatpak
        - firewalld
        - xdg-utils
      enableUnits:
        - firewalld.service

An official vendor repository:

    apiVersion: software.deskos.org/v1alpha1
    kind: RpmRepository
    metadata:
      name: vscode
    spec:
      id: code
      displayName: Visual Studio Code
      baseURL: https://packages.microsoft.com/yumrepos/vscode
      gpgKeys:
        - https://packages.microsoft.com/keys/microsoft.asc

A verified upstream binary, pinned by version and SHA-256:

    apiVersion: software.deskos.org/v1alpha1
    kind: BinaryArtifact
    metadata:
      name: openshift-client
    spec:
      version: 4.22.14
      source:
        url: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.22.14/openshift-client-linux-amd64-rhel9-4.22.14.tar.gz
        sha256: 73d4204fe2d028a5fb3b05f71da174915442c445d3b417321c635bf17d099f6b
      archive: tar.gz
      files:
        - path: oc
          destination: /usr/local/bin/oc
          mode: "0755"

A system Flatpak remote and applications preinstalled from it:

    apiVersion: software.deskos.org/v1alpha1
    kind: FlatpakRemote
    metadata:
      name: flathub
    spec:
      title: Flathub
      url: https://dl.flathub.org/repo/
      collectionID: org.flathub.Stable
      gpgKeyFile: keys/flathub.gpg
    ---
    apiVersion: software.deskos.org/v1alpha1
    kind: FlatpakSet
    metadata:
      name: deskos-reference-apps
    spec:
      remote: flathub
      applications:
        - id: io.github.kolunmi.Bazaar
          branch: stable

GNOME intent; an organization layer overrides the Core defaults it
names:

    apiVersion: desktop.deskos.org/v1alpha1
    kind: GnomeProfile
    metadata:
      name: example-desktop
    spec:
      defaults:
        windows:
          buttons: [close]
        appearance:
          wallpaper:
            light: ../assets/example-org.svg
          loginLogo: ../assets/example-org-login-logo.svg
        session:
          idle:
            blankAfter: 5m
          lock:
            enabled: true
            delay: 0s
        dock:
          enabled: true
          position: bottom
          behavior: intellihide
          iconSize: 40
          showTrash: false

Boot appearance:

    apiVersion: system.deskos.org/v1alpha1
    kind: BootProfile
    metadata:
      name: deskos-core
    spec:
      splash: graphical
      quiet: true
      watermark: ../assets/deskos/deskos-splash-watermark.png

The organization's workstation then composes Core, its baseline and a
role on a platform; profile order is not precedence:

    apiVersion: core.deskos.org/v1alpha1
    kind: Workstation
    metadata:
      name: example-devops-centos10
    spec:
      displayName: Example Org DevOps Workstation
      platformRef: centos-stream-10
      profiles:
        - deskos-core
        - example-baseline
        - example-devops

`deskosctl plan ./resources ./examples/example-org --workstation
example-devops-centos10` shows the composed result, including which layer
won each setting.

## Composition

A workstation is composed from explicit semantic layers:

    foundation
        <
    organization
        <
    role
        <
    workstation

DeskOS Core provides defaults.

Organizations intentionally override them.

Conflicting settings at the same semantic layer are errors.

File order is never used as an implicit precedence mechanism.

## DeskOS Core

DeskOS Core is the reusable workstation foundation maintained by the
project.

The public reference workstation uses CentOS Stream 10.

Organizations that require RHEL can compile the same DeskOS Core
semantics directly onto the official RHEL 10 bootc base.

The RHEL artifact is not derived from the CentOS image.

## Software model

DeskOS deliberately avoids becoming a universal package manager.

The managed baseline can use the delivery mechanism appropriate to the
software:

    distribution RPM
    official vendor RPM
    verified upstream binary
    system Flatpak
    future managed web applications

User/project environments such as mise, SDKMAN, Homebrew, language
version managers, dotfiles and personal Toolboxes are outside the DeskOS
managed baseline.

## GNOME

DeskOS exposes administrator intent rather than dconf implementation
details.

An administrator should be able to describe things such as:

- window controls;
- wallpapers and branding;
- fonts;
- icon and cursor themes;
- ordered favorites;
- dock behavior;
- idle timeout;
- lock behavior;

without knowing which GNOME schema or dconf key implements them.

DeskOS Core ships reasonable defaults, but organizations can override
those defaults declaratively.

## CLI

The artifact-factory CLI is:

    deskosctl

The initial workflow is:

    deskosctl validate ...
    deskosctl plan ...
    deskosctl render ...
    deskosctl version

`validate` checks resources and composition.

`plan` shows the effective workstation before anything is built.

`render` produces a deterministic build context.

`version` prints the release version and commit (`dev` and the Git
revision for local builds).

## Releases

Pushing a tag `vX.Y.Z` (or `vX.Y.Z-<prerelease>`) runs the CI checks and
creates a GitHub Release with `deskosctl-vX.Y.Z-linux-amd64` and
`SHA256SUMS` (`.github/workflows/release.yml`). Only linux/amd64 is built.
The binary is not signed.

    git tag -a vX.Y.Z -m "deskosctl vX.Y.Z"
    git push origin vX.Y.Z

To download and verify `v0.1.0`, the current release:

    gh release download v0.1.0 --repo deskosproject/deskos-core
    sha256sum -c SHA256SUMS
    chmod +x deskosctl-v0.1.0-linux-amd64

or without `gh`:

    base=https://github.com/deskosproject/deskos-core/releases/download/v0.1.0
    curl -fL -O "$base/deskosctl-v0.1.0-linux-amd64" -O "$base/SHA256SUMS"
    sha256sum -c SHA256SUMS

## Development

Requires Go 1.26 or later (the oldest supported Go release, and the
minimum of the `golang.org/x/text` dependency).

Build the CLI:

    go build -o bin/deskosctl ./cmd/deskosctl

Run tests:

    go test ./...
    python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
    python3 -m unittest discover -s tests/rhel # RHEL factory harness, same

Validate the included resources:

    ./bin/deskosctl validate ./resources ./examples/example-org

Inspect the public CentOS reference workstation:

    ./bin/deskosctl plan ./resources \
      --workstation deskos-core-centos10

Add `--format json` to print the canonical plan.

### Build the image yourself

Contributors build the image from their checkout instead of pulling it.
Render the build context:

    ./bin/deskosctl render ./resources \
      --workstation deskos-core-centos10 \
      --backend containerfile \
      --output ./dist/deskos-core-centos10

and build it with rootful Podman, so bootc-image-builder can read it:

    sudo podman build -t localhost/deskos-core-centos10 ./dist/deskos-core-centos10

The QCOW2 and ISO commands in [Getting DeskOS](#getting-deskos) work
unchanged with `localhost/deskos-core-centos10` as the image reference.
[tests/vm/README.md](tests/vm/README.md) boots the disk and checks the
GNOME session against the plan; `tests/rhel/` does the same for RHEL on
an entitled host.

An example organization (`example-org`) with a RHEL 10 developer
workstation is included separately to prove organization- and
role-specific composition. It lives in its own
resource root and reuses DeskOS resources without copying them:

    ./bin/deskosctl plan ./resources ./examples/example-org \
      --workstation example-devops-rhel10

    ./bin/deskosctl render ./resources ./examples/example-org \
      --workstation example-devops-rhel10 \
      --output ./dist/example-devops-rhel10

RHEL builds require appropriate Red Hat access and must not be publicly
redistributed.

The `Makefile` wraps these commands for convenience (`make check`,
`make plan-example`, ...).

## Reference artifacts

DeskOS Core for CentOS Stream 10 is published at
`quay.io/deskos/deskos-core` by `vm-bootcheck.yml` run with `publish` on
`main`: the job pushes the exact image that passed its boot and session
checks, as `:<commit>` and `:latest`. Derived QCOW2 and ISO images will be
distributed from S3-compatible object storage, not from GitHub; until then
they are built locally (see [Getting DeskOS](#getting-deskos)). The CI
builds QCOW2 disks only as test input and does not upload them. The
`deskosctl` binary is published through GitHub Releases (see Releases).

## Design principles

- Intent is data.
- Compilation is deterministic.
- File order does not define precedence.
- Providers emit typed IR, not arbitrary shell.
- DeskOS Core provides capabilities and defaults.
- Organizations provide policy and content.
- DeskOS owns the artifact, not the running machine.
- CI is an adapter, not the product.
- AI may assist development, but it is not part of compiler semantics.
- Existing Linux technologies should be composed rather than reinvented.

## Project direction

The immediate goal is not feature breadth.

It is to prove the resource model on real workstation artifacts.

Future work may include identity enrollment, corporate trust,
OpenSCAP/compliance evidence, update UX, firmware integration, disk-image
release automation, supply-chain attestations and deeper automated
workstation testing.

Those features should be added only after the core composition model has
proved itself.

See [docs/architecture.md](docs/architecture.md),
[docs/roadmap.md](docs/roadmap.md), the [ADRs](docs/adr/) and the
[research notes](docs/research-notes.md).
