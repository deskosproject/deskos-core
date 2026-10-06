# ADR 0010: Artifact evidence and the supply-chain gate

**Status:** accepted (2026-10-06)

## Context

Organizations that run DeskOS artifacts (an internal workstation image on
RHEL, a published image on CentOS Stream) are asked to state what the
artifact is made of and to show it carries no known critical
vulnerabilities. Today DeskOS compiles intent deterministically and
publishes an image, but ships no bill of materials and no scan.

Two facts constrain the design:

- The **compiler is deterministic, offline and model-free** (CLAUDE.md,
  "AI"; AGENTS.md invariant 11). It resolves no package versions, reads no
  host files and scans no bytes. Only the **built OCI image** has the
  installed content and the resolved versions.
- The industry push toward "agentic software factories" (Bluefin's Hive,
  Red Hat's Hummingbird) mixes two separable things: an **industrial
  factory** that produces hardened, reproducible, attested artifacts, and
  an **agentic maintenance** layer that proposes changes. The first is
  worth having on its own; the second is governed separately
  (AGENTS.md, "Agentic contribution").

## Decision

**Evidence and scanning live around the artifact, in the build/CI
pipeline, never inside compiler semantics.** The compiler may only
*declare* intent, deterministically.

1. The compiler's contribution is `deskosctl sbom`: a **declared** SBOM
   (CycloneDX 1.5) of the inputs a Plan selects, with hashes where they
   exist and no timestamp or `serialNumber`, so identical resources render
   identical bytes.
2. The **installed** SBOM and the **CVE scan** run on the built image with
   a scanner (Syft/Grype) in CI. The declared SBOM is the ingredient list;
   the installed SBOM is the authoritative one; a gate compares the two.
3. A digest is promoted only when it passes the checks and the scan, and
   carries a signature. Promotion is by digest, never by a mutable tag.

## Staged plan

| Stage | Deliverable | Status |
|---|---|---|
| 1 | Declared SBOM: `deskosctl sbom`, deterministic, tested | **done** (v0.8.0) |
| 2 | CI: render → build the CS10 image → Syft SBOM → Grype scan → upload artifacts, fail on critical; compare declared vs installed | **added**, first run pending |
| 3 | Signing and provenance: `cosign sign` over the digest and an SBOM attestation; promotion by digest | **started** (keyless in CI; key-based on the RHEL factory host) |
| 4 | Evidence travels with the artifact: ship the SBOM beside `plan.json`; an endpoint command reports it | planned |
| 5 | Compliance resources (`TrustPolicy`, `ComplianceProfile`) and OpenSCAP execution | later |

The how-to for stages 1–3 is [docs/supply-chain.md](../supply-chain.md).

## Consequences

- The compiler stays deterministic, offline and model-free; evidence is
  additive and optional for the compiler itself.
- Scanning needs a **built image**: CentOS Stream 10 runs in CI (`vm-bootcheck`
  style); RHEL needs the entitled factory host. No scan is claimed before
  one has run.
- The tooling is vendor-neutral: CycloneDX is the format, Syft/Grype/cosign
  are open source, so the plan survives a change of pipeline.
- This is a **publication gate**, not a controller: it governs which digest
  moves forward, not what a running machine does (ADR 0001).
- "Declared" and "installed" are distinct terms in this project; the
  declared SBOM must never be presented as proof of image content.
