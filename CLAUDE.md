# CLAUDE.md: DeskOS engineering rules

## Mission

DeskOS compiles a workstation from organizational intent. "Workstation" is the
composition kind an organization authors; it compiles into a workstation *image*,
not a distribution edition.

## Hard boundary

DeskOS owns the artifact and does not reconcile running endpoints. Never add fleet
controllers, reconciliation or endpoint enforcement to solve a problem that
belongs to Ansible, Satellite, Foreman, MDM or EDR.

## Compiler flow

    resources -> validation -> reference resolution -> deterministic composition
      -> typed IR (Plan) -> backend -> build context

Never shortcut this to YAML → Containerfile templating. Providers emit typed IR
and execute no shell during planning.

## Resource API

`apiVersion` / `kind` / `metadata` / `spec`, groups under `deskos.org`, current
version `v1alpha1`. Kubernetes-inspired envelope, no Kubernetes runtime
dependencies, controllers, CRDs or etcd.

## Composition

File order is never precedence. Layers: `foundation < organization < role <
workstation`. A higher layer may override a lower scalar on purpose; different
values at one layer are errors. Sets union deterministically. Preserve provenance
so a conflict names every contributor.

## Core

Core is a semantic foundation, not the CentOS image, and must serve CentOS Stream
10 and RHEL 10 alike. A RHEL workstation starts from the official RHEL bootc base,
never from the CentOS DeskOS image. No editions (Developer, Enterprise, Gaming,
KDE, LTS, HWE); organizations compose `Platform + Core + organization + role`.

## Managed baseline

Manage machine-wide state that any authorized user needs; never a personal
environment. No Homebrew, Brewfiles, mise, asdf, SDKMAN, version managers,
dotfiles or personal Toolbox contents in the core model.

## Software acquisition

Prefer distribution RPM; official vendor repository; verified upstream binary;
system Flatpak; later a managed web application. No curl-pipe-shell. Binaries need
cryptographic checksums.

## GNOME

Expose semantic administrator concepts; do not make users write raw dconf keys for
supported features. Core supplies defaults that organization and role profiles
override by layer. A **default** can be overridden by the user, `local`/`site` or
runtime configuration management; a **lock** enforces the value in the image and
beats `local` and `site`. Core never locks. No GNOME or Mutter forks, Shell
patches, random COPRs or theme hacks.

## Scripts

Bash is not the architecture. Containerfile steps generated from typed operations
are normal; user-supplied shell in resources is not. Never add `script`,
`postInstall` or `command` escape hatches.

## Determinism

Identical semantic inputs produce byte-identical plans and build contexts: no
timestamps, no random identifiers, explicitly sorted output. Say "deterministic
compilation and rendering"; never claim a bit-for-bit reproducible image until
bases, repository content and keys are pinned.

## Credentials

Registry credentials, RHSM credentials and activation keys, signing keys, join
secrets and enrolment tokens never enter resources, artifacts or Git history.
Factory credentials are external inputs to CI.

## RHEL

RHEL-derived bootc images are never publicly redistributed, and rendering a RHEL
context without credentials must stay possible. RHEL evidence is exactly what
`docs/architecture.md` ("Validation status") records: composition and rendering
are tested here; the entitled 10.2 build (`0e65bf4b`) and its VM checks are
historical, and that validation now runs in a downstream organization's
repository. There is no RHEL job here and no E2E test. Do not claim more until a
build shows it, and do not weaken the architecture because this machine cannot
run an authenticated build.

## Scope discipline

Ask of every feature: does this help an organization define and compile its
workstation artifact? If not, it does not belong in Core. Prefer one abstraction
proved by a real case over several half-implemented kinds.

## Kinds

Fourteen public kinds: `Platform`, `Profile`, `Workstation`, `PackageSet`,
`RpmRepository`, `AptRepository` (ADR 0015), `BinaryArtifact`, `FlatpakRemote`,
`FlatpakSet`, `GnomeProfile`, `BootProfile` (ADR 0006), `UpdatePolicy` (ADR 0007),
`TrustAnchor` (ADR 0009), `Theme` (ADR 0013). Do not expand the list casually.

## Out of scope for the compiler

Enrolment, configuration management, reconciliation, fleet work, OpenSCAP and
OSCAL execution, lockfiles, CUE, KCL, external provider loading, firmware
management, PWA resources, CI-provider adapters, multi-architecture builds and
LLM integration. Evidence, SBOM and signing live around the artifact in the
pipeline, never inside compiler semantics
([ADR 0010](docs/adr/0010-artifact-evidence-and-supply-chain.md)). Document
extension points instead of implementing them.

## AI

AI may propose source changes; it is not part of compiler semantics. The compiler
stays deterministic and usable with no model and no network.

## Testing

Every semantic behaviour has focused tests: loading, schema validation, unknown
GVK rejection, reference resolution, composition, conflict detection,
profile-order independence, deterministic plan JSON and render output, and the
CentOS and RHEL golden plans. Test architecture, not coverage.

## External references

Verify current authoritative upstream documentation for bootc, RHEL, CentOS,
GNOME and Flatpak. Write-ups of other factories are useful for architecture
lessons; they are not the resource-model specification.

## Working style

Inspect before rewriting. Keep dependencies small; prefer boring, readable Go; no
premature repository splits or speculative framework code. Never publish
artifacts or change remote services without explicit human instruction. End
substantial work with what changed, why, tests run, assumptions, known gaps and
the next milestone.

## Repository map

- `cmd/deskosctl`: the factory CLI; `cmd/deskos`: the runtime companion.
- `internal/model`: envelope, GVK, layers, provenance.
- `internal/loader`: YAML roots to resources, no kind knowledge.
- `internal/schema`: embedded JSON Schema validation (`schemas/`).
- `internal/registry`: GVK to provider dispatch.
- `internal/providers/{core,software,desktop,system}`: typed decoding,
  contributions and lowering; `internal/providers/assets`: reads confined to the
  resource root.
- `internal/compose`: reference resolution and layered composition.
- `internal/plan`: the typed Plan.
- `internal/backends/containerfile`: Plan to build context.
- `internal/sbom`: declared CycloneDX SBOM ([supply-chain.md](docs/supply-chain.md)).
- `internal/textplan`: the text form of `deskosctl plan`.
- `internal/compiler`: pipeline wiring and the architecture, golden and
  composition tests. Refresh goldens with `go test ./internal/compiler -update`
  and review the diff.
- `tests/vm`: QEMU boot and instrumented session checks
  (`python3 -m unittest discover -s tests/vm` for their unit tests).
- `.github/workflows`: `ci.yml`, `vm-bootcheck.yml`, `supply-chain.yml`,
  `promotion.yml`, `iso.yml` and `release.yml`.
