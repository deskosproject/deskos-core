# The release record

One question: **what exactly did a release promote, and what was checked before
it moved?** The answer is a single JSON object per promotion, stored beside the
evidence and named by the digest it describes.

## What it is, and what it is not

A release record is *derived*. It is built from two things that already exist:

- the **evidence index** for the digest, which lists every evidence object and
  its SHA-256;
- the **approval record**, which says who moved the digest, from which channel,
  to which one, why, and which evidence index hash they checked.

It cannot claim a check, a digest or an evidence object that those two do not
hold. It is not a build log, not a promise of reproducibility, and not a fleet
inventory: machines report their own state, and this repository never collects
it.

## Stages

| Stage | What it produces | Where |
|---|---|---|
| Build | the image digest | the registry, `:<commit>` |
| Gate | scan, boot, session and first-boot results, SBOMs | the evidence index |
| Publish | `:latest` and `:candidate` | the registry |
| Approval | the approval record | `approvals/<repository>/stable/` |
| Release | this record, then `:stable` | `releases/<repository>/stable/` |

The record is written after the approval and after the installer move, so the
`iso` field can name the installer digest that was actually promoted.

## Schema

`schema` is `https://deskos.org/supply-chain/release/v1`.

```json
{
  "schema": "https://deskos.org/supply-chain/release/v1",
  "repository": "deskos-core-centos-stream-10",
  "image": "quay.io/deskos/deskos-core-centos-stream-10",
  "digest": "sha256:…",
  "channel": { "from": "candidate", "to": "stable", "previous": "sha256:…" },
  "iso": { "digest": "sha256:…" },
  "signature": { "scheme": "keyless", "identity": "…/image-publish.yml@refs/heads/main" },
  "evidence": { "index_sha256": "…", "objects": [{ "name": "…", "sha256": "…" }] },
  "approval": { "…": "the complete approval record" },
  "time": "2026-11-05T01:02:03Z"
}
```

- **digest** is the image the release is about; every other digest refers to it.
- **channel.previous** is the stable digest the move replaced, or `null` for the
  first move.
- **iso** is `null` for a platform that publishes no installer. A platform whose
  installer repository does not exist is not a failed promotion.
- **signature.scheme** is `keyless` for the public Core images and `key-based`
  for the private organization; the scheme is a property of the audience, not a
  stage.
- **evidence.objects** is copied from the index, so a verifier can compare the
  record with the index without trusting the record alone.
- **approval** is the full record, including the reason and the approver, so one
  document answers "what" and "why".

## Where it lives

The record is uploaded to the evidence bucket under
`releases/<repository>/stable/<timestamp>-<hex>.json` and kept as a workflow
artifact for 14 days. It never rewrites a digest's immutable evidence index:
adding the release to that index would change the bytes the approval hashed.

`tests/evidence/release.py` writes and uploads it; `test_release.py` covers the
schema, the digest and channel checks, the dry run and the key format. The
promotion workflow calls it; nothing else writes the path.

## Verifying one

1. Fetch the record and its approval.
2. Check `digest` against the image manifest the channel serves.
3. Hash the evidence index and compare it with `evidence.index_sha256`.
4. Verify the signature named by `signature` (keyless identity or `cosign.pub`).
5. Verify the installer digest, when `iso` is not null.

Anything that does not match is an error, never a partial pass. That is the same
rule the evidence verifier and the scan gate use.
