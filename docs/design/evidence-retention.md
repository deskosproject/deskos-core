# Durable evidence for a published digest

**Status: implemented and exercised, 2026-10-26.** The R2 buckets,
`evidence.deskos.org`, the public upload token and the uploader are in place, and
the first real digest is stored and verified: 16 objects for
`sha256:96fe80d33023bdd4b674b45288e13a0e4aaedb935b09e154bc4adbc132f3fab6`.

P0-05 asks that the records for a historical image digest survive workflow
artifact expiry.

## What is true today

- Workflow records (declared and installed SBOMs, Grype report, Plan, boot and
  session results, digests, signature check) are a GitHub artifact with
  `retention-days: 14`, attempt-suffixed since P0-01.
- The image carries a signed attestation naming the **SHA-256 of the SBOM**; the
  SBOM bytes are not attached.
- The exception record is in Git and attested; the raw scan and its effective
  exceptions are only in the artifact.
- Nothing maps `sha256:<image digest>` to its records, and no command fails when
  a record is missing.

## Requirements

1. **Keyed by image digest**, not by workflow run or tag.
2. **Immutable**: an object for a digest cannot be silently replaced.
3. **Integrity**: every object is addressed by its own SHA-256; a missing or
   changed object fails retrieval.
4. **Access split**: Core records public, RHEL records private to the
   organization, no credential in the repository.
5. **Bounded cost**: a written deletion and GC policy.
6. **Verify without the build**: a person checks a digest's records against the
   image signature and the stored hashes.
7. **No recursion**: records are not embedded in the image after the digest is
   computed.

## Options

| Option | Strengths | Problems |
|---|---|---|
| **OCI 1.1 referrers** on the image registry (quay for Core, zot for RHEL) | travels next to the digest; consumers already use `oras`/`cosign`; no second store | referrers support and GC differ per registry and are unverified for quay; deleting the subject may take the records with it; large SBOMs are unusual registry blobs |
| **Object store** (R2 or an organization bucket) keyed `sha256/<digest>/…` | immutable keys, cheap, already used by the download site; public and private buckets are simple | needs a scoped upload credential in CI and a public-read decision; no automatic link from the image |
| **Git repository** for JSON records | reviewable history, no new infrastructure | SBOMs and scan output are large; binary in Git is a poor fit; every upload is a commit |
| **GitHub Release assets** | durable enough and easy to fetch | tied to a release tag, not every build; release authority is heavier than the weekly build |
| **GitHub artifacts** | already produced | 14 days; explicitly not durable |

## Decision (2026-10-25)

Use an **R2 object store keyed by image digest**, with a per-digest `index.json`
listing every record object and its SHA-256. The attestation still names the SBOM
hash; the index resolves that hash to bytes.

- Core records are **public** (`deskos-evidence-public`); organization records
  are **private**, in a bucket that organization owns.
- The key is `<namespace>/<repository>/sha256/<hex>/`, for example
  `deskos/deskos-core-centos-stream-10/sha256/<hex>/index.json`, independent of
  the registry host, so moving an image does not move its records.
- The upload token is scoped to the bucket or prefix and staged as a secret,
  never in `argv` or the repository.
- **Every published digest is retained.** GC may delete only temporary or
  unpublished objects.
- The index starts **unsigned**; the attested SBOM SHA-256 makes a replaced
  index or object fail the comparison.

The index is not authoritative by itself: a missing object or a differing hash
**fails** verification, and a 404 is not "no known issues". The two scripts are
[`tests/evidence/index.py`](../../tests/evidence/index.py) and
[`tests/evidence/verify.py`](../../tests/evidence/verify.py): one builds the
sorted index and refuses symlinks, the other checks the index, rejects traversal
and duplicate names, and hashes every named object.

## CI configuration

The publish job uses the GitHub environment `evidence-write`. Non-sensitive
configuration is environment variables; only the credential is a secret.

**Variables**

```text
EVIDENCE_S3_ENDPOINT
EVIDENCE_S3_REGION
EVIDENCE_S3_BUCKET
EVIDENCE_PUBLIC_BASE_URL
```

**Secrets**

```text
EVIDENCE_S3_ACCESS_KEY_ID
EVIDENCE_S3_SECRET_ACCESS_KEY
```

The endpoint contains the account ID; the bucket name and public base URL are
public, and the region is `auto`. The access key ID is kept secret so GitHub
masks it in logs; the secret access key is the credential.

## Retrieval procedure

```bash
digest=sha256:…
base=https://evidence.deskos.org/deskos/deskos-core-centos-stream-10/sha256/${digest#sha256:}
curl -fsS "$base/index.json" -o index.json
jq -r '.objects[] | "\(.sha256)  \(.name)"' index.json > SHA256SUMS
# Fetch each named object, then:
sha256sum -c SHA256SUMS
cosign verify <image@$digest> …       # identity policy from supply-chain.md
# Compare the SBOM hash named by the attestation with the object fetched here.
```

The public bucket and `evidence.deskos.org` are defined in
`deskos-infrastructure`; the private bucket and token owner are to provision.

## Repair and deletion

An upload can fail after the image is published. The workflow artifact keeps the
records for 14 days, so `evidence-maintenance.yml` can **backfill** the digest
from that run's artifact and check the stored copy; the artifact names its own
digest, and a mismatch is refused.

`tests/evidence/prune.py` lists and deletes one digest prefix. It is a dry run
until `--apply`, `--confirm` must repeat the exact prefix, and a prefix holding
an `index.json` is refused unless `--force` is given.

That path ran on 2026-10-26: `prune` removed the 332 objects of a failed upload
(`sha256:36824c3e…`, which indexed the whole checkout and never wrote
`index.json`), and `backfill` published the 16 objects from run `40244202449`'s
artifact. Fetching them from `evidence.deskos.org` and running `verify.py`
reported `verified 16 object(s)`. The records for `sha256:96fe80d3…` were not
touched.

## Open decisions

- Organization private bucket owner and name.
- Who owns and rotates each upload token, and which environment secret carries
  it.
- Whether to sign `index.json` later, and with which identity.
- Whether to add OCI referrers as a convenience; do not make both paths
  authoritative.
- A signed source-to-binary attestation for release assets is a separate gap.

## Non-goals

- No telemetry or fleet inventory; this is a record about an artifact, not about
  who runs it.
- No post-build SBOM embedded in the image, which would change the digest.
- No SLSA level claim; this design only makes existing records durable and
  retrievable.
