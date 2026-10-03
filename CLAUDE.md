# CLAUDE.md: DeskOS Engineering Rules

## Mission

DeskOS is an open workstation artifact factory.

Its central thesis is:

> A workstation is compiled from organizational intent.

DeskOS deterministically compiles declarative workstation definitions
into Linux workstation artifacts.

## Hard boundary

DeskOS is **not configuration management**.

DeskOS owns the artifact.

It does not continuously reconcile running endpoints.

Do not add fleet controllers, runtime reconciliation, or endpoint
configuration enforcement to solve problems that belong to Ansible,
Satellite, Foreman, MDM, EDR or similar systems.

## Core architecture

The required compiler flow is:

    resources
        ->
    validation
        ->
    reference resolution
        ->
    deterministic composition
        ->
    typed IR / Plan
        ->
    backend
        ->
    artifact build context

Never shortcut this to:

    YAML -> Containerfile template

Providers emit typed IR.

They do not execute shell commands during planning.

## Resource API

Public DeskOS resources use:

    apiVersion
    kind
    metadata
    spec

API groups live under `deskos.org`.

Current API version is `v1alpha1`.

The resource envelope is inspired by Kubernetes API design.

Do not add Kubernetes runtime dependencies, controllers, CRDs or etcd.

## Composition

File ordering is never precedence.

Profile semantic layers are:

    foundation < organization < role < workstation

Higher layers may intentionally override lower-layer scalar defaults.

Conflicting values at the same layer are errors.

Set-like state such as package lists composes by deterministic union.

Preserve provenance so conflict errors can identify the contributing
resources.

## DeskOS Core

DeskOS Core is a semantic workstation foundation, not the CentOS image.

The same Core intent must be usable with:

- CentOS Stream 10;
- RHEL 10.

A RHEL workstation must start from the official RHEL bootc base rather
than inheriting from the CentOS DeskOS image.

## No flavors

Do not create DeskOS editions such as Developer, Enterprise, Gaming, KDE,
LTS or HWE.

Organizations create concrete Workstation resources by composing:

    Platform
    + DeskOS Core
    + organization profiles
    + role profiles

## Managed baseline

DeskOS manages organization-required machine-wide baseline state.

DeskOS does not attempt to manage every user's development environment.

Do not introduce Homebrew, Brewfiles, mise, asdf, SDKMAN, language
version managers, dotfiles or user-specific Toolbox contents into the
core resource model.

If the organization requires a tool for every authorized user, it may
belong to system baseline state.

If it belongs to an individual project workflow, it is outside DeskOS.

## Software acquisition

Prefer, in order:

1. distribution RPM;
2. official vendor RPM repository;
3. verified official upstream binary/archive;
4. system Flatpak for suitable desktop applications;
5. future managed PWA when appropriate.

Do not use curl-pipe-shell installers.

Binary artifacts require cryptographic checksums.

## GNOME

Expose semantic administrator concepts.

Do not make users author raw dconf/GSettings keys for normal supported
features.

Core should provide reasonable defaults.

Organization and role profiles may override those defaults through
explicit semantic precedence.

Defaults and locks mean different things:

- defaults are artifact-supplied values that users, the `local`/`site`
  dconf databases and runtime configuration management may override;
- a lock is artifact-level enforcement. A lock in the DeskOS (`distro`)
  database also wins over `local` and `site`, so runtime tools cannot
  change the value.

DeskOS Core never locks. Lock a setting only when the organization wants
the image to enforce it, never for settings that Ansible, Satellite or
similar tools are expected to manage.

Avoid GNOME forks, Shell patches, Mutter patches, random COPRs and
unsupported theme hacks.

## Scripts

Bash is not the architecture.

Containerfile RUN commands generated from typed operations are normal.

Arbitrary user-supplied shell in resource YAML is not allowed.

Do not solve a missing abstraction by adding generic `script`,
`postInstall`, or `command` escape hatches.

## Determinism

Identical semantic inputs must produce byte-identical plans and rendered
build contexts.

No timestamps.

No random identifiers.

Sort map/set outputs explicitly.

Say "deterministic compilation/rendering" or "reproducible workstation
definition". Do not claim bit-for-bit reproducible images until base
images, repository content and keys are pinned.

## Credentials

Never place:

- registry credentials;
- RHSM usernames/passwords;
- activation keys;
- signing keys;
- domain join secrets;
- enrollment tokens;

inside workstation resources, generated artifacts or Git history.

Factory credentials are external inputs to CI/build environments.

## RHEL

RHEL-derived bootc images must not be publicly redistributed.

Rendering RHEL build contexts without credentials must remain possible.

RHEL evidence is exactly what `docs/architecture.md`, "Validation
status", records: composition and rendering are validated; the entitled
RHEL 10.2 build of the example workstation (image `0e65bf4b...`, pinned
base) passed `bootc container lint` with every layer free of build-host
subscription state, and its QCOW2 passed `bootcheck.py` and
`sessioncheck.py` on the factory VM under nested KVM, run manually. There
is no RHEL CI, no E2E test, and Flatpak preinstall is not exercised on
RHEL. Do not claim more until an entitled build shows it.

Do not weaken architecture merely because the current developer machine
cannot perform an authenticated RHEL build.

## Scope discipline

Before adding a feature, ask:

> Does this help an organization define and compile its workstation
> artifact?

If not, it probably does not belong in DeskOS Core.

Prefer proving one abstraction with a real testcase over adding many
half-implemented resource types.

## Current milestone

Current public resource kinds are limited to:

- Platform
- Profile
- Workstation
- PackageSet
- RpmRepository
- BinaryArtifact
- FlatpakRemote
- FlatpakSet
- GnomeProfile
- BootProfile (ADR 0006)

Do not expand that list casually.

## Out of scope for current milestone

Do not implement unless explicitly requested:

- FreeIPA/AD enrollment;
- configuration management;
- endpoint reconciliation;
- Fleet;
- OpenSCAP execution;
- OSCAL;
- SBOM generation;
- SLSA;
- Cosign;
- Rekor;
- lockfiles;
- CUE;
- KCL;
- external provider loading;
- update CLI;
- firmware management;
- PWA resources;
- Azure DevOps adapters;
- Tekton adapters;
- GitLab mirrors;
- multi-architecture builds;
- AI/LLM integrations.

Document extension points instead.

## AI

AI may propose source changes.

AI is not part of DeskOS compiler semantics.

The compiler must remain deterministic and usable without any model or
SaaS API.

## Testing

Every semantic behavior must be represented by focused tests.

Important properties include:

- loading;
- schema validation;
- unknown GVK rejection;
- reference resolution;
- composition;
- conflict detection;
- profile-order independence;
- deterministic JSON plans;
- deterministic render output;
- CentOS Core golden plan;
- RHEL example-org golden plan.

Tests should verify architecture, not just code coverage.

## External references

When changing bootc, RHEL, CentOS, GNOME or Flatpak integration, verify
current authoritative upstream documentation rather than relying on old
examples or memory.

Bluefin's `THEPATTERN.md` is useful for factory architecture lessons.

It is not the DeskOS resource-model specification.

## Working style

Inspect before rewriting.

Keep dependencies small.

Prefer boring, readable Go.

Avoid premature repository splitting.

Avoid speculative framework code.

Do not publish artifacts or modify remote services without explicit
human instruction.

At the end of substantial work, explain:

- what changed;
- why;
- tests run;
- assumptions;
- known gaps;
- next recommended milestone.

## Repository map

- `cmd/deskosctl`: the factory CLI.
- `internal/model`: envelope, GVK, layers, provenance.
- `internal/loader`: YAML roots to resources; no kind knowledge.
- `internal/schema`: embedded JSON Schema validation (`schemas/`).
- `internal/registry`: GVK to provider dispatch.
- `internal/providers/{core,software,desktop,system}`: typed decoding,
  contributions and lowering per API group; `internal/providers/assets`:
  asset reads confined to the resource root.
- `internal/compose`: reference resolution and layered composition.
- `internal/plan`: the typed Plan (IR).
- `internal/backends/containerfile`: Plan to build context.
- `internal/textplan`: the text form of `deskosctl plan`.
- `internal/compiler`: pipeline wiring and the architecture, golden and
  composition tests. Refresh goldens with
  `go test ./internal/compiler -update` and review the diff.
- `tests/vm`: QEMU boot and instrumented session checks for built disks
  (`python3 -m unittest discover -s tests/vm` for their unit tests).
- `tests/rhel`: `factory.py`, the RHEL build, layer scan, boot and
  session validation run on an entitled host; it publishes only a commit
  status (`python3 -m unittest discover -s tests/rhel`).
- `.github/workflows`: `ci.yml` (checks on push and pull requests),
  `vm-bootcheck.yml` (manual CS10 build, boot and session checks) and
  `release.yml` (tag-driven `deskosctl` release).
