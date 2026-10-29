# ADR 0010: Artifact records and the supply-chain check

**Status:** accepted (2026-10-06)

## Context

Organizations that run DeskOS artifacts (an internal workstation image on RHEL, a published image on CentOS Stream)
are asked what the artifact is made of and to show it carries no known critical vulnerabilities. The compiler is
**deterministic, offline and model-free** (CLAUDE.md, "AI"; AGENTS.md invariant 11): it resolves no package versions
and scans no bytes, so only the **built OCI image** has the installed content and the resolved versions. An
**industrial factory** that produces hardened, attested artifacts is separable from the **agentic maintenance** layer
that proposes changes (AGENTS.md, "Agentic contribution").

## Decision

**Records and scanning live around the artifact, in the build/CI pipeline, never inside compiler semantics.** The
compiler may only *declare* intent, deterministically.

1. The compiler's contribution is `deskosctl sbom`: a **declared** SBOM (CycloneDX 1.5) of the inputs a Plan selects,
   with hashes where they exist and no timestamp or `serialNumber`, so identical resources render identical bytes.
2. The **installed** SBOM and the **CVE scan** run on the built image with a scanner (Syft/Grype) in CI. The declared
   SBOM is the ingredient list, the installed SBOM is the authoritative one, and a check compares them.
3. A digest is promoted only when it passes the checks and the scan and carries a signature, by digest and never by a
   mutable tag.

## Staged plan

| Stage | Deliverable | Status |
|---|---|---|
| 1 | Declared SBOM: `deskosctl sbom`, deterministic, tested | **done** (v0.8.0) |
| 2 | CI: render → build the CS10 image → Syft SBOM → Grype scan → upload artifacts, fail on critical; compare declared vs installed | **done**: `supply-chain.yml` runs it; the check passed on 2026-10-10 |
| 3 | Signing and provenance: `cosign sign` over the digest and an SBOM attestation; promotion by digest | **done**: keyless for CentOS Stream 10 in CI, key-based for RHEL in a downstream organization's pipeline (2026-10-14) |
| 4 | Records travel with the artifact: ship the SBOM beside `plan.json`; an endpoint command reports it | **half done**: `deskos report` describes the plan and says the SBOM is not installed ([ADR 0018](0018-the-runtime-companion.md)) |
| 5 | Compliance resources (`TrustPolicy`, `ComplianceProfile`) and OpenSCAP execution | later |

The how-to for stages 1-3 is [docs/supply-chain.md](../supply-chain.md).

## Consequences

- The compiler stays deterministic, offline and model-free; records are additive and optional for it.
- Scanning needs a **built image**: CentOS Stream 10 runs in CI (`vm-bootcheck` style); RHEL runs in a downstream
  organization's pipeline, which owns the entitled build host and holds the results.
- The tooling is vendor-neutral (CycloneDX, Syft/Grype/cosign), so the plan survives a change of pipeline.
- This is a **publication check**, not a controller: it decides which digest moves forward, not what a running machine
  does (ADR 0001).
- "Declared" and "installed" are distinct terms: the declared SBOM is never proof of image content.
