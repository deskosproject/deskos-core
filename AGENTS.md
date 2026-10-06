# AGENTS.md

Durable rules for any coding agent in this repository. `CLAUDE.md` has
the full engineering rules and `docs/adr/` the decisions behind them; if
anything here disagrees with an ADR, the ADR wins and this file is wrong.

## Invariants

1. **Artifact, not fleet.** DeskOS compiles workstation artifacts. No
   controllers, agents, reconciliation loops or runtime enforcement
   (ADR 0001).
2. **Typed Plan boundary.** Resources → validation → deterministic
   composition → typed Plan → backend. Backends read only
   `internal/plan`; never template YAML into a Containerfile (ADR 0002).
3. **Semantic layers, no last writer wins.** `foundation < organization <
   role < workstation`. Sets union, keyed definitions must match, scalars
   take the highest layer and conflict within one layer. File, root and
   profile order never change output (ADR 0003).
4. **Exactly twelve public kinds** in v1alpha1: Platform, Profile,
   Workstation, PackageSet, RpmRepository, BinaryArtifact, FlatpakRemote,
   FlatpakSet, GnomeProfile, BootProfile (ADR 0006), UpdatePolicy
   (ADR 0007), TrustAnchor (ADR 0009). A test enforces the count; adding a
   kind needs an explicit decision.
5. **No escape hatches.** No `script`, `command`, `postInstall` or other
   shell fields, no raw kernel arguments or Plymouth script themes, no
   templating, and nothing that manages a user's `$HOME`
   or personal tooling (ADR 0005).
6. **RHEL starts from the official base.** A RHEL workstation is built
   `FROM registry.redhat.io/rhel10/rhel-bootc`, never from a CentOS-derived
   DeskOS image. RHEL-derived images are never publicly published or
   redistributed (ADR 0004).
7. **Core and organizations stay separate.** DeskOS Core carries no
   organization or role software; organizations extend it from their own
   resource roots without copying it.
8. **Defaults are not locks.** Defaults may be overridden by users and by
   runtime configuration management. A lock is image-level enforcement
   that also overrides `local`/`site` dconf; Core never locks.
9. **Platform gaps are reported, not hidden.** A requirement the platform
   cannot meet fails composition with a reason; never substitute silently.
10. **Say only what was verified.** Compilation is deterministic; images
    are not yet bit-for-bit reproducible. RHEL 10.2 has one clean entitled
    build of an earlier revision of the example and one manual,
    owner-observed boot; there is no automated boot or E2E evidence on RHEL
    (see `docs/architecture.md`, "Validation status").
11. **AI is not part of compiler semantics.** Compilation must work with
    no model, no network and no SaaS API, and identical inputs must keep
    producing identical plans and build contexts. No resource kind,
    provider, Plan IR or backend may depend on a model or generate
    nondeterministic output (CLAUDE.md, "AI").
12. **Human-gated automation.** Automation may propose; a human approves.
    Agents never self-merge, never push to `main`, never touch signing,
    secrets, releases or registries, and never weaken a check to go green.

## Requires Ricardo's explicit approval

- Publishing or pushing images, tags or commits, and creating releases.
- Changing the license (Apache-2.0).
- Handling credentials or changing shared infrastructure.

## Checks

    make check        # gofmt, go vet, go test, validate
    python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
    go test ./internal/compiler -update   # refresh goldens, then review the diff

Do not run `go test ./internal/compiler -update` merely to make a red
check pass: a golden diff is a change under review, not a fix for a
failing test.

## Agentic contribution

DeskOS is compiled from intent. A patch may be authored or assisted by an
agent, under exactly the same rules as any other patch: review, tests and
a human decision. The value of automation here is review capacity and
evidence, not unattended change.

**The split.** Agents operate on the repository, CI, the issue tracker,
documentation and tests. They do **not** operate on compiler semantics:
no resource kind, provider, Plan IR or backend behaviour may depend on a
model (invariant 11). Keep the compiler deterministic and offline.

**Start with low-risk work.** Suitable first tasks are triage and
labelling, documentation, corrections under `resources/**`, flaky-test
fixes, and pin bumps that carry their checksum. Architecture, new kinds,
signing, secrets, releases and registry operations are out of scope for
automation.

**Workflow.**

- Work from a scoped, labelled issue; if it is unclear, ask instead of
  guessing.
- Branch from `main`; never commit or push to `main`.
- Open a PR. `make check` and the golden diff must be green before review.
- A human reviews and merges. An agent does not self-merge, and an agent
  review never substitutes for a maintainer's approval.
- Trailer: `Assisted-by: <agent> <model>`. Never `Co-authored-by:`.

**Evidence, not telemetry.** Reports and feedback are user-owned and
opt-in, the same principle as the artifact factory: DeskOS never phones
home and never aggregates user data centrally.

**Reuse before building.** Prefer an existing orchestrator — for example
Hive, part of KubeStellar (CNCF Sandbox) — over an agent runner written
into this repository. The orchestrator is infrastructure around DeskOS,
not part of it, and must never become a compiler dependency.

This section is the operating procedure of
[ADR 0011](docs/adr/0011-agentic-contribution-layer.md).
