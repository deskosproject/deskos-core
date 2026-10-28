# Promotion of one tested digest

**Status: implemented 2026-10-28; the first automatic move is the next gate
run.** The gate publishes weekly, by itself, and `promotion.yml` now runs when
that gate succeeds: it verifies the digest and its evidence, stores an approval
record and moves `:stable` and the ISO channel by digest. A dispatch still
promotes a digest built earlier. Until that run, the moves exercised by hand on
2026-10-26 remain the only ones with a record.

The trigger changed because a cadence that waits for a person is not a cadence.
What did not change is the record: every move names a digest, its evidence and
who approved it — now "automatic (gate)", with the run that produced it. The first promotion moved
`sha256:96fe80d3…` to `:stable` from `:latest` (run `40317904401`, because
`:candidate` starts with the next gate run) and its record is
`approvals/deskos-core-centos-stream-10/stable/20261026T115001Z-96fe80d3….json`.
No canary or pilot channel exists.

P0-06 asks for build-once promotion: one image digest moves through stages
without being rebuilt, carrying its evidence and a recorded decision. The gate
builds, scans and boots an image and moves `:candidate` and `:latest` to that
digest; `promotion.yml` then moves `:stable` to the same digest, and the approval
record names the digest, its evidence and the run that produced it.

## What is true today

- The publish job builds **one** image per platform leg, boots that same
  `$IMAGE`, pushes `:<commit>`, and moves `:latest` and `:candidate` by digest
  after the checks. `:stable` is not touched by the gate.
- `:<commit>` is a naming convention, not registry-enforced immutability. A
  digest is the exact identity.
- Evidence is a 14-day workflow artifact (attempt-suffixed) and, for a
  published digest, a durable digest-keyed index under `evidence.deskos.org`
  (P0-05, implemented 2026-10-26). The attestation names the SBOM hash; the
  index resolves it to bytes.
- `promotion.yml` is the only writer of `:stable`. It verifies the keyless
  signature (identity: `supply-chain.yml` on `main`), fetches and verifies every
  object the digest's index names, records the approval, moves the tag by
  digest, and verifies the tag resolves to the same digest.
- Machines follow the reference they were installed from; `:latest` is mutable,
  a digest is not. `deskos update` stages whatever that reference now serves.
- Publication does not wait for a person: the weekly gate publishes and the
  promotion follows its success. The record still names the approval —
  `automatic (gate)` and the run that produced the digest — and a dispatch
  remains for a digest built earlier. There is no second reviewer or environment
  approval by default (P0-04).

## Model

A **channel** is a registry tag that points at one digest. It is not an image
and it does not imply a rebuild.

| Channel | Meaning | Entry | Exit |
|---|---|---|---|
| `candidate` | built and gate-checked, not recommended for general use | the weekly gate, or a `publish=true` dispatch, by digest | evidence index complete, scan/boot green, the gate succeeded |
| `canary` | **not used**; documented for completeness | a named owner and opted-in machines | an agreed window with no blocking regression |
| `stable` | the channel the page and documentation recommend | `promotion.yml`, by digest, with an approval record | replaced by a later digest, or withdrawn |

At this project's scale the path is `candidate → stable`, with no canary stage.
The important part is that every move names a digest, the evidence and who
approved it.

### Tag map

| Tag | Role | Written by |
|---|---|---|
| `:<commit>` | build record; immutable by convention | the gate, by digest |
| `:candidate` | the digest the gate produced; the source a promotion checks | the gate, by digest |
| `:latest` | fast lane: the same digest as `:candidate`, for whoever wants the newest build | the gate, by digest |
| `:stable` | recommended for machines and the download page | `promotion.yml` only |

Keeping `:latest` next to `:candidate` is deliberate. `candidate` names what the
gate produced and is the tag `promotion.yml` checks; `latest` is the familiar
fast-lane name, and it is not a second stable. The reason is the method, not a
model borrowed from another project: one digest is built and tested once, a move
names it, and the record says what was approved and why.

## Promotion is a move, not a build

1. Resolve the source tag to a digest and verify the image signature and the
   evidence index for that digest.
2. Check the stage's entry conditions: scan result, boot/session/first-boot
   results, SBOM/Plan hashes, exception record and review dates.
3. Write an approval record: digest, from-channel, to-channel, approver, time,
   previous stable digest, evidence index hash and a reason. It is stored in the
   public evidence bucket under `approvals/<repository>/<channel>/…`, never
   inside a digest's immutable prefix. An automatic move records
   `automatic (gate)` and the run that produced the digest, so "who approved
   this" has an answer without a person having pressed a button.
4. Move the destination tag **by digest** (a registry-side tag update), never
   by rebuilding. `:<commit>` remains as the build record.
5. Verify from a clean machine that the destination tag resolves to the same
   digest, that its signature verifies, and that its evidence is retrievable.

A tag move is not magic: caches and already-staged machines keep what they have
until they fetch. The approval record is the source of truth for what was
promoted and why.

## Failure partway

- If the move succeeds but a consumer cannot fetch, the registry is the problem;
  do not rebuild.
- If verification fails after the move, move the tag back to the previous digest
  and record the failed approval; the previous digest's evidence must still be
  present.
- If evidence is missing, the promotion does not start. A 404 is not "no known
  issues".
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
no controller). There is no telemetry to prove who runs a canary.

## Decisions (2026-10-26)

- Channels exist per platform repository: `candidate` and `stable`. `:latest`
  stays as the **fast lane** next to `stable`, not as a second stable, and
  `:candidate` is the promotion source. Retiring `:latest` is not planned.
- A move is authorized by the gate's success (2026-10-28), and a dispatch still
  authorizes one by hand. Both write the same record; the automatic path names
  the run, the dispatch names the actor. A cadence that waited for a person was
  not a cadence, and the record does not need a button to be honest.
- The ISO channel moves with the image: `promotion.yml` finds the ISO tagged
  `sha256-<image digest>`, verifies its signature and moves the ISO `:stable` to
  it. A platform that published no ISO leaves the ISO channel alone.
- No canary stage. Adding one needs opted-in machines and a named owner, and the
  project has neither.
- The previous stable digest is recorded in the approval record and remains
  retrievable by its `:<commit>` tag and by digest.
- The approval record is a JSON object in the evidence bucket, not a signed
  attestation. Its `evidence_index_sha256` binds it to the evidence it was
  approved against.

## Open questions

- Whether to sign the approval record, and with which identity.
- How OCI referrers and the evidence store interact if both are kept.

## Non-goals

- No fleet controller, no automatic rollout, no percentage-based canary.
- No rebuild to promote, and no mutable tag used as evidence.
- No claim that a promoted digest is secure; promotion carries the evidence, it
  does not replace review.
