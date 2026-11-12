# ADR 0031: one build and release contract for Core and an organization

**Status:** accepted (2026-11-12)

## Context

Core publishes reference workstations from this repository. An organization
publishes its own workstation from a repository that pins Core resources and
adds organization profiles. The two pipelines answer the same questions (what
was built, what booted, what was scanned, what a person approved) with
different stages, names and failure rules.

Measured on 2026-11-12:

- Core scans inside `image-publish`, before anything is pushed.
- The organization publishes a boot-checked commit and an `:edge` tag before
  the scan, then the scan moves `:candidate` and `:latest`.
- Core builds the installer in a reusable `installer-build` workflow, called
  after the image publish. The organization builds the ISO inside the image
  job, so an ISO failure also stops the scan and the image channel.
- The workflow names carry an organization prefix in one repository and not in
  the other, and the tag set differs by one tag.

Two pipelines mean two mental models, two review checklists and two ways for a
release to fail. The user asked for one contract: same process, same names, same
tag vocabulary.

## Decision

**Both repositories use five stages, with these names and this order.**

| Stage | Workflow | What it does | Moves |
|---|---|---|---|
| 1 | `image-publish[-<platform>].yml` | Build; declared and installed SBOMs; compare; boot, session and first-boot checks; push the immutable commit; sign it | `:edge` |
| 2 | `image-scan[-<platform>].yml` | Grype and the reviewed exceptions against the published digest | `:candidate`, `:latest` |
| 3 | `promotion.yml` | Verify signature, evidence and approval record | `:stable` |
| 4 | `installer-build.yml` | Build the ISO from the published digest and sign it | `:candidate`, `:latest`; follows `:stable` |
| 5 | `installer-test.yml` | Install unattended in QEMU, on demand | none |

- **Names.** Workflows are `<stage>[-<platform>].yml`, without an organization
  prefix. Jobs are `prepare`, `build`, `verify`, `scan`, `publish`, `installer` and
  `promote`; the installer workflow's own job is `iso`. Repository names
  stay `<project>-<variant>-<platform>`.
- **Steps.** The boot checks are named “Boot check”, “Session check” and
  “First-boot check”; the scan step is “Scan (Grype)”; the signing step is
  “Publish, sign and attest”; the installer steps are “Build the installer ISO”
  and “Publish and sign the installer”.
- **Tags.** `:edge` is image only and means “the build boot-checked and signed
  this digest, before the scan”. `:latest` means “the scan passed”; the
  installer follows it for the same digest. `:stable` means “the evidence and
  the approval were verified”. `:candidate` is the source a promotion checks.
- **Failure rules.**
  - A failed scan leaves `:latest` and `:stable` where they were; `:edge` does
    not move back.
  - A failed ISO does not stop the image: the image keeps its channels and the
    installer waits at `:candidate`.
  - A failed promotion leaves `:latest` moved and `:stable` behind.
  - `installer-test` never gates publication; it is a separate signal.
- **Publication.** Core publishes the platforms it owns and never RHEL. An
  organization publishes its own images, and its RHEL artifacts stay private.
  This contract changes the stages, not who may publish what.

## Consequences

Core must split the scan out of `image-publish`, publish the commit before the
scan and add `:edge`. That changes what exists before the gate: an unscanned
commit and `:edge` become visible. The channels that a machine follows do not
change: `:latest` and `:stable` still require the scan and the promotion.

An organization must extract the ISO from its image job into `installer-build`,
rename the workflows, and let a failed ISO stop only the installer. Its scan and
promotion already match stages 2 and 3.

Neither repository tests the published ISO’s install path today. Core’s
`installer-test` builds a test-only ISO; an organization has no installer test.
Until stage 5 exercises the published ISO, both advertise an installer whose
install path is unverified. That gap is named here rather than hidden.

The migration is staged: the names and the ISO split first, the Core scan split
next, and the installer smoke test last. Existing release records and tags stay
readable; no digest is rebuilt to satisfy the contract.
