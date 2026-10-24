# Durable evidence for a published digest

**Status: design proposal, 2026-10-24. No storage or credential is created by
this document.**

P0-05 asks that a historical image digest's evidence can still be retrieved and
verified after workflow artifacts expire. Today it cannot.

## What is true today

- Workflow evidence (declared and installed SBOMs, Grype report, Plan, boot and
  session results, digests, signature verification) is a GitHub artifact with
  `retention-days: 14` (attempt-suffixed since P0-01).
- The image carries a signed attestation that names the **SHA-256 of the SBOM**;
  the SBOM bytes themselves are not attached.
- The exception record is in Git and is attested; the raw scan and its effective
  exceptions are only in the artifact.
- Release binaries have checksums but no signed source-to-binary attestation.
- There is no index from `sha256:<image digest>` to its evidence and no
  verification command that fails when evidence is missing.

## Requirements

1. **Keyed by image digest**, not by workflow run or tag.
2. **Immutable**: an object for a digest cannot be silently replaced.
3. **Integrity**: every object is addressed by its own SHA-256; a missing or
   changed object fails retrieval.
4. **Access split**: Core evidence public; RHEL evidence private to the
   organization. No credential in the repository.
5. **Bounded cost and retention**: a deletion/GC policy is stated, not implied.
6. **Verify without the build**: a person can check evidence for a digest using
   the image's signature and the stored hashes.
7. **No recursive promise**: evidence is not embedded into the image after the
   digest is computed.

## Options

| Option | Strengths | Problems |
|---|---|---|
| **OCI 1.1 referrers** on the image registry (quay for Core, zot for RHEL) | travels next to the digest; consumers already use `oras`/`cosign`; no second store | registry referrers support and garbage collection differ and are unverified for quay; deleting the subject may take evidence with it; large SBOMs are unusual registry blobs |
| **Object store** (R2 or an organization bucket) keyed `sha256/<digest>/…` | immutable keys, cheap, already used by the download site; public/private buckets are straightforward | needs a scoped upload credential in CI and a public-read decision; no automatic link from the image |
| **Git repository** for JSON evidence | reviewable history, no new infrastructure | SBOMs and scan output are large; binary in Git is a poor fit; every upload is a commit |
| **GitHub Release assets** | durable enough and easy to fetch | tied to a release tag, not every build; release authority is heavier than the gate |
| **GitHub artifacts** | already produced | 14 days; explicitly not durable |

## Recommendation

Use an **object store keyed by digest**, with a small per-digest `index.json`
listing every evidence object and its SHA-256. Keep the existing attestation
naming the SBOM hash; the index resolves that hash to bytes. Publish Core
evidence in a public bucket and RHEL evidence in the organization's private
bucket. Upload from the publish workflow through a scoped token staged as a
secret, never in `argv`.

Start with **one digest** as a lab: upload the installed SBOM, scan report, Plan,
identity record and a README; fetch them from a clean machine; verify hashes and
the image signature. Only then decide retention, GC and whether OCI referrers
become the secondary path.

The index must not be authoritative by itself: if an object is missing or its
hash differs, verification **fails**. A 404 is not "no known issues". A verifier
for this shape exists at
[`tests/evidence/verify.py`](../../tests/evidence/verify.py); it checks the
index, rejects traversal and duplicate names, and hashes every named object.

## Retrieval procedure (to test with the first digest)

```bash
digest=sha256:…
base=https://evidence.deskos.org/$digest
curl -fsS "$base/index.json" -o index.json
jq -r '.objects[] | "\(.sha256)  \(.name)"' index.json > SHA256SUMS
# Fetch each named object, then:
sha256sum -c SHA256SUMS
cosign verify <image@$digest> …       # identity policy from supply-chain.md
# Compare the SBOM hash named by the attestation with the object fetched here.
```

The exact base URL, bucket name and visibility are decisions, not assumptions.

## Open decisions

- Core public bucket name/URL and who owns its token; RHEL private bucket owner.
- Whether the index is signed, and which key/identity signs it.
- Retention: keep every digest forever, keep promoted digests, or time-box with
  a published policy.
- Garbage collection: never delete a digest that is still referenced by a
  release, a channel or an exception record.
- Whether to add OCI referrers later as a convenience; do not make both paths
  authoritative.
- A signed source-to-binary attestation for release assets is a separate gap.

## Non-goals

- No telemetry or fleet inventory; this is evidence about an artifact, not about
  who runs it.
- No post-build SBOM embedded in the image (that would change the digest).
- No SLSA level claim; this design only makes existing evidence durable and
  retrievable.
