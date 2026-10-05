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

## Requires Ricardo's explicit approval

- Publishing or pushing images, tags or commits, and creating releases.
- Changing the license (Apache-2.0).
- Handling credentials or changing shared infrastructure.

## Checks

    make check        # gofmt, go vet, go test, validate
    python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
    go test ./internal/compiler -update   # refresh goldens, then review the diff
