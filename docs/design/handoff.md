# The handoff contract

What a consumer needs to verify a DeskOS artifact **without** this repository, its
workflows or GitHub. If a check needs something that is not in this list, the
handoff is incomplete.

## The contract

| Piece | What it is | Where it lives | How it is verified |
|---|---|---|---|
| **Compiler and resources** | the commit that produced the artifact | the image's provenance (commit in `resolvedDependencies`) | read the predicate; the commit is a public repository |
| **Plan** | the canonical, deterministic Plan the image was built from | evidence index, `plan.json` | sha256 from the index |
| **Rendered context** | `generated-manifest.json`, the files and layers the backend wrote | evidence index | sha256 from the index |
| **Subject digest** | the OCI manifest digest of the published image | registry, `:stable` or a digest | `cosign verify` with the workflow identity |
| **Installed SBOM** | the SBOM of what is actually in the image | evidence index, referenced by the attestation | the attestation names its sha256; the index resolves it to bytes |
| **Provenance** | SLSA v1 predicate: commit, base digest, builder, run | attached to the image | `cosign verify-attestation --type slsaprovenance1` |
| **Test results** | boot, session and first-boot summaries, the scan and the accepted risks | evidence index | sha256 from the index |
| **Authorization** | who moved the digest to `:stable`, when, against which evidence | `approvals/<repository>/stable/<stamp>-<digest>.json` in the evidence bucket | read the record; it names the evidence index hash |

The **evidence index** is the hub: a JSON object per digest naming every file and
its sha256, served from the public bucket. Everything above except the subject
digest and the signature is resolved through it.

## What an independent verifier does

No GitHub account, no workflow, no access to this repository:

```bash
# 1. the digest :stable serves
skopeo inspect --format '{{.Digest}}' docker://quay.io/deskos/deskos-core-centos-stream-10:stable

# 2. who built it
cosign verify <image>@<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity 'https://github.com/deskosproject/deskos-core/.github/workflows/supply-chain.yml@refs/heads/main'

# 3. what it says about itself
cosign verify-attestation --type slsaprovenance1 <image>@<digest> … | jq -r '.payload' | base64 -d

# 4. the evidence, byte for byte
curl https://evidence.deskos.org/deskos/deskos-core-centos-stream-10/sha256/<digest>/index.json
```

Step 4 is the one a consumer should automate: fetch the index, fetch each object,
compare the hash. `tests/evidence/verify.py` does exactly that and takes the index
and the directory.

## What is GitHub-specific, and what is not

- **Not:** the artifacts (registry), the evidence (bucket), the signatures
  (Sigstore's transparency log), and the approval records (bucket). A different CI
  could produce the same handoff.
- **Is:** the workflow files that produce it, the identity in the certificate
  (`…/supply-chain.yml@refs/heads/main`), and the run id in the provenance. A
  different builder would sign with its own identity, and a verifier pins that
  identity deliberately.

## Gaps, named

- **The evidence index is unsigned.** The image's attestation names the SBOM hash,
  and the promotion refuses an index that does not verify, but the index itself
  carries no signature. An attacker who could write the bucket could rewrite an
  index and its objects consistently. The mitigation today is that the promotion
  reads it before the move and the approval record pins its hash.
- **The approval record is unsigned.** `automatic (gate)` is a claim by the
  workflow; the record is only as trustworthy as the bucket.
- **The negative cases exist; the gap is different.** `tests/evidence/test_verify.py`
  proves the verifier refuses a missing object, changed bytes, a traversal path, a
  duplicate name and a wrong image digest. What is **not** refused — because
  nothing can refuse it — is a *consistent* forgery: an index and matching objects
  written by whoever can write the bucket. That is the unsigned-index gap above,
  stated twice because it is the one that matters.
- **An external policy engine was probed once.** Conforma `ec` verified the
  keyless signature through Rekor, found the SLSA v1 predicate and ran 155 checks;
  the release policy set then failed on the shape of the evidence it expects
  (Tekton build types, a PipelineRun attestation, CVE and SBOM attestations of
  their own). The result is in [conforma-probe.md](conforma-probe.md); what it
  would need is a policy for our build type, not a change to the compiler.

## What this contract is for

So that "verify the image" is a list of objects and commands rather than an act of
trust in a workflow someone cannot see — and so that a second build platform, if
one is ever justified, has a target to hit.
