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

What Milestone 0 has shown:

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
  identify screens or test applications.

RHEL 10: one entitled build of an earlier revision of
`example-devops-rhel10` passed `bootc container lint`, confirming the comps
group, Dash to Dock, the vendor repositories and `oc`, with every layer
free of build-host subscription-manager state; its QCOW2 was booted once,
manually. The current example is not built on RHEL, and there is no
automated boot or E2E test on RHEL. See
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

`validate` checks resources and composition.

`plan` shows the effective workstation before anything is built.

`render` produces a deterministic build context.

## Development

Requires Go 1.26 or later (the oldest supported Go release, and the
minimum of the `golang.org/x/text` dependency).

Build the CLI:

    go build -o bin/deskosctl ./cmd/deskosctl

Run tests:

    go test ./...

Validate the included resources:

    ./bin/deskosctl validate ./resources ./examples/example-org

Inspect the public CentOS reference workstation:

    ./bin/deskosctl plan ./resources \
      --workstation deskos-core-centos10

Add `--format json` to print the canonical plan.

Render it:

    ./bin/deskosctl render ./resources \
      --workstation deskos-core-centos10 \
      --backend containerfile \
      --output ./dist/deskos-core-centos10

and build it locally with Podman:

    podman build -t localhost/deskos-core-centos10 ./dist/deskos-core-centos10

An example organization (`example-org`) with a RHEL 10 developer workstation is included separately to
prove organization- and role-specific composition. It lives in its own
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

The intended public OCI namespace for DeskOS Core is:

    quay.io/deskos/deskos-core

A future mirror may be published to GitHub Container Registry.

Future releases are expected to provide OCI plus derived QCOW2 and ISO
artifacts.

Publication is not part of the initial compiler milestone.

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
