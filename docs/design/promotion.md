# Promotion of one tested digest

**Status: implemented, first automatic move 2026-10-28** (`sha256:871e59cc…`,
record `approvals/deskos-core-centos-stream-10/stable/20261028T161205Z-871e59cc….json`).
The weekly build publishes by itself, and `promotion.yml` runs when it succeeds:
it verifies the digest and its records, stores an approval record and
moves `:stable` and the ISO channel by digest. A dispatch still promotes a digest
built earlier.

Every move names a digest, its records and who approved it: `automatic (gate)`
with the run that produced it. The first promotion of all was a dispatch on
2026-10-26 and moved `sha256:96fe80d3…` to `:stable` from `:latest` (run
`40317904401`, because `:candidate` starts with the next weekly build); its record is
`approvals/deskos-core-centos-stream-10/stable/20261026T115001Z-96fe80d3….json`.
No canary or pilot channel exists.

P0-06 asks for build-once promotion: one digest moves through stages without a
rebuild. The weekly build scans and boots an image and moves `:candidate` and
`:latest` to that digest; `promotion.yml` then moves `:stable` to the same digest.

## What is true today

- The publish job builds **one** image per platform leg, boots that same `$IMAGE`,
  pushes `:<commit>`, and moves `:latest` and `:candidate` by digest. It never
  touches `:stable`.
- `:<commit>` is a naming convention, not registry-enforced immutability. A
  digest is the exact identity.
- Records are a 14-day workflow artifact (attempt-suffixed) and, for a published
  digest, a digest-keyed index under `evidence.deskos.org` (P0-05, implemented
  2026-10-26).
- `promotion.yml` is the only writer of `:stable`: it verifies the keyless
  signature (identity `supply-chain.yml` on `main`) and every object the index
  names, records the approval, moves the tag by digest, and verifies the tag
  resolves to the same digest.
- Machines follow the reference they were installed from; `:latest` is mutable, a
  digest is not. `deskos update` stages whatever that reference now serves.
- Publication does not wait for a person: the weekly build publishes and
  promotion follows, with no second reviewer or environment approval by default
  (P0-04).

## Model

A **channel** is a registry tag that points at one digest. It is not an image and
does not imply a rebuild.

| Channel | Meaning | Entry | Exit |
|---|---|---|---|
| `candidate` | built and checked by the weekly build, not recommended for general use | the weekly build, or a `publish=true` dispatch, by digest | evidence index complete, scan/boot green, the build succeeded |
| `canary` | **not used**; documented for completeness | a named owner and opted-in machines | an agreed window with no blocking regression |
| `stable` | the channel the page and documentation recommend | `promotion.yml`, by digest, with an approval record | replaced by a later digest, or withdrawn |

At this project's scale the path is `candidate → stable`, with no canary stage.

### Tag map

| Tag | Role | Written by |
|---|---|---|
| `:<commit>` | build record; immutable by convention | the weekly build, by digest |
| `:candidate` | the digest the weekly build produced; the source a promotion checks | the weekly build, by digest |
| `:latest` | fast lane: the same digest as `:candidate`, for whoever wants the newest build | the weekly build, by digest |
| `:stable` | recommended for machines and the download page | `promotion.yml` only |

## Promotion is a move, not a build

1. Resolve the source tag to a digest and verify the image signature and the
   evidence index for that digest.
2. Check the stage's entry conditions: scan result, boot/session/first-boot
   results, SBOM/Plan hashes, exception record and review dates.
3. Write an approval record (digest, from-channel, to-channel, approver, time,
   previous stable digest, evidence index hash and a reason) under
   `approvals/<repository>/<channel>/…` in the public evidence bucket, never
   inside a digest's immutable prefix.
4. Move the destination tag **by digest**, never by rebuilding; `:<commit>`
   remains as the build record.
5. Verify from a clean machine that the destination tag resolves to the same
   digest, that its signature verifies, and that its records are retrievable.

Caches and already-staged machines keep what they have until they fetch. The
approval record is the source of truth.

## Failure partway

- If the move succeeds but a consumer cannot fetch, the registry is the problem;
  do not rebuild.
- If verification fails after the move, move the tag back to the previous digest
  and record the failed approval; the previous digest's records must remain.
- If records are missing, the promotion does not start. A 404 is not "no known
  issues".
- If the **installer** failed, the image is still promoted: promotion depends on
  the `publish` job (scan, boot, signature, records), and the installer is a
  separate workflow whose failure leaves the ISO channel where it was; `iso.yml`
  follows the image channel whenever it succeeds, or on a dispatch for that
  digest.
- Withdrawal is an explicit tag move to the previous digest plus an advisory; it
  does not revoke cached images or signatures (see the threat model).

## Machine references

| Machine follows | Effect on promotion |
|---|---|
| `:stable` | sees the promoted digest on its next update |
| `:candidate` or `:canary` | sees the stage it opted into |
| `@sha256:…` | never moves; an intentional reference change is required |

No channel switch is automatic. A machine that is offline, metered or on battery
still follows its own update policy; promotion is not a fleet action (ADR 0001,
no controller).

## Decisions (2026-10-26)

- Channels exist per platform repository: `candidate` and `stable`. `:latest`
  stays the **fast lane**, not a second stable; retiring it is not planned.
- A move is authorized by the weekly build's success (2026-10-28) through the
  `publish` job, or by a dispatch. Both write the same record; the automatic path
  names the run, the dispatch the actor.
- The ISO channel moves with the image: `promotion.yml` finds the ISO tagged
  `sha256-<image digest>`, verifies it and moves the ISO `:stable`. A platform
  with no ISO leaves that channel alone; `iso.yml` tags the ISO `:stable` when
  the installer appears after the promotion.
- **One person approves, deliberately (2026-10-28).** An environment approval on
  `evidence-write` would pause every weekly promotion. The automatic path moves
  only the digest its triggering run published (the records name that `run_id`),
  after the keyless signature verifies against `supply-chain.yml` on `main` and
  every object matches its hash. A dispatch records its actor. What is left to a
  person is upstream: a merge to `main` under branch protection and the
  `validate` check. Revisit with a second maintainer, or under an SLA.
- No canary stage: it needs opted-in machines and a named owner.
- The previous stable digest is recorded in the approval record and stays
  retrievable by its `:<commit>` tag and by digest.
- The approval record is a JSON object in the evidence bucket, not a signed
  attestation; its `evidence_index_sha256` binds it to the records.

## Open questions

- Whether to sign the approval record, and with which identity.
- How OCI referrers and the evidence store interact if both are kept.

## Non-goals

- No fleet controller, no automatic rollout, no percentage-based canary.
- No rebuild to promote, and no mutable tag used as a record.
- No claim that a promoted digest is secure; promotion carries records, it does
  not replace review.
