# The handoff contract

What a consumer needs to verify a DeskOS artifact **without** this repository, its
workflows or GitHub; anything missing from this list leaves the handoff
incomplete.

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
| **Authorization** | who moved the digest to `:stable`, when, against which records | `approvals/<repository>/stable/<stamp>-<digest>.json` in the evidence bucket | read the record; it names the evidence index hash |

The **evidence index** is the hub: a JSON object per digest naming every file and
its sha256, served from the public bucket. Everything above except the subject
digest and the signature resolves through it.

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

Step 4 is the one to automate: fetch the index, fetch each object, compare the
hash. `tests/evidence/verify.py` does that and takes the index and the directory.

## What is GitHub-specific, and what is not

- **Not:** the artifacts (registry), the records (bucket), the signatures
  (Sigstore's transparency log) and the approval records (bucket); a different CI
  could produce the same handoff.
- **Is:** the workflow files, the certificate identity
  (`…/supply-chain.yml@refs/heads/main`) and the provenance run id. A different
  builder signs with its own identity, which a verifier pins deliberately.

## Gaps, named

- **The evidence index is unsigned.** The promotion refuses an index that does not
  verify, but the index carries no signature: whoever can write the bucket can
  rewrite an index and its objects consistently. The approval record pins its
  hash.
- **The approval record is unsigned.** `automatic (gate)` is a claim by the
  workflow; the record is only as trustworthy as the bucket.
- **No negative-case suite.** `tests/evidence/verify.py` and the promotion refuse
  missing or mismatched records, but no test *proves* they refuse a tampered
  index, a wrong digest or an empty object.
- **No independent policy evaluation yet.** Whether Conforma/`ec` or an
  organization's own engine can consume these predicates has not been tried, so
  no compatibility is claimed.

## What this contract is for

So that "verify the image" is a list of objects and commands, not trust in a
workflow someone cannot see, and a second build platform has a target.
