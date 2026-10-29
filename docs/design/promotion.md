# Promotion of one tested digest

**Status: implemented, first automatic move 2026-10-28** (`sha256:871e59cc…`,
record `approvals/deskos-core-centos-stream-10/stable/20261028T161205Z-871e59cc….json`).
The gate publishes weekly, by itself, and `promotion.yml` runs when that gate
succeeds: it verifies the digest and its evidence, stores an approval record and
moves `:stable` and the ISO channel by digest. A dispatch still promotes a digest
built earlier.

Every move names a digest, its evidence and who approved it — `automatic (gate)`
with the run that produced it. The first promotion moved `sha256:96fe80d3…` to
`:stable` from `:latest` (run `40317904401`, because `:candidate` starts with the
next gate run); its record is
`approvals/deskos-core-centos-stream-10/stable/20261026T115001Z-96fe80d3….json`.
No canary or pilot channel exists.

P0-06 asks for build-once promotion: one image digest moves through stages
without being rebuilt. The gate builds, scans and boots an image and moves
`:candidate` and `:latest` to that digest; `promotion.yml` then moves `:stable`
to the same digest.

## What is true today

- The publish job builds **one** image per platform leg, boots that same
  `$IMAGE`, pushes `:<commit>`, and moves `:latest` and `:candidate` by digest.
  `:stable` is not touched by the gate.
- `:<commit>` is a naming convention, not registry-enforced immutability. A
  digest is the exact identity.
- Evidence is a 14-day workflow artifact (attempt-suffixed) and, for a
  published digest, a durable digest-keyed index under `evidence.deskos.org`
  (P0-05, implemented 2026-10-26).
- `promotion.yml` is the only writer of `:stable`: it verifies the keyless
  signature (identity: `supply-chain.yml` on `main`) and every object the
  digest's index names, records the approval, moves the tag by digest, and
  verifies the tag resolves to the same digest.
- Machines follow the reference they were installed from; `:latest` is mutable,
  a digest is not. `deskos update` stages whatever that reference now serves.
- Publication does not wait for a person: the weekly gate publishes and the
  promotion follows its success. There is no second reviewer or environment
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

### Tag map

| Tag | Role | Written by |
|---|---|---|
| `:<commit>` | build record; immutable by convention | the gate, by digest |
| `:candidate` | the digest the gate produced; the source a promotion checks | the gate, by digest |
| `:latest` | fast lane: the same digest as `:candidate`, for whoever wants the newest build | the gate, by digest |
| `:stable` | recommended for machines and the download page | `promotion.yml` only |

`candidate` names what the gate produced and is the tag `promotion.yml` checks;
`latest` is the fast-lane name, not a second stable.

## Promotion is a move, not a build

1. Resolve the source tag to a digest and verify the image signature and the
   evidence index for that digest.
2. Check the stage's entry conditions: scan result, boot/session/first-boot
   results, SBOM/Plan hashes, exception record and review dates.
3. Write an approval record — digest, from-channel, to-channel, approver, time,
   previous stable digest, evidence index hash and a reason — under
   `approvals/<repository>/<channel>/…` in the public evidence bucket, never
   inside a digest's immutable prefix.
4. Move the destination tag **by digest**, never by rebuilding; `:<commit>`
   remains as the build record.
5. Verify from a clean machine that the destination tag resolves to the same
   digest, that its signature verifies, and that its evidence is retrievable.

Caches and already-staged machines keep what they have until they fetch. The
approval record is the source of truth.

## Failure partway

- If the move succeeds but a consumer cannot fetch, the registry is the problem;
  do not rebuild.
- If verification fails after the move, move the tag back to the previous digest
  and record the failed approval; the previous digest's evidence must remain.
- If evidence is missing, the promotion does not start. A 404 is not "no known
  issues".
- If the **installer** failed, the image is still promoted: the promotion is
  gated on the `publish` job (scan, boot, signature and evidence), and the
  installer is a separate workflow whose failure leaves the ISO channel where it
  was; `iso.yml` follows the image channel whenever it succeeds, or on a
  dispatch for that digest.
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
- A move is authorized by the gate's success (2026-10-28) — the `publish` job —
  or by a dispatch. Both write the same record; the automatic path names the
  run, the dispatch the actor.
- The ISO channel moves with the image: `promotion.yml` finds the ISO tagged
  `sha256-<image digest>`, verifies it and moves the ISO `:stable`. A platform
  with no ISO leaves that channel alone; `iso.yml` tags the ISO `:stable` when
  the installer appears after the promotion.
- **One person approves, deliberately (2026-10-28).** An environment approval on
  `evidence-write` would pause every weekly promotion. The automatic path moves
  only the digest its triggering run published — the evidence names that
  `run_id` — after the keyless signature verifies against `supply-chain.yml` on
  `main` and every evidence object matches its hash. A dispatch records its
  actor. What is left to a person is upstream: a merge to `main` under branch
  protection and the `validate` check. Revisit with a second maintainer, or under
  an SLA.
- No canary stage: it needs opted-in machines and a named owner.
- The previous stable digest is recorded in the approval record and remains
  retrievable by its `:<commit>` tag and by digest.
- The approval record is a JSON object in the evidence bucket, not a signed
  attestation; its `evidence_index_sha256` binds it to the evidence.

## Open questions

- Whether to sign the approval record, and with which identity.
- How OCI referrers and the evidence store interact if both are kept.

## Non-goals

- No fleet controller, no automatic rollout, no percentage-based canary.
- No rebuild to promote, and no mutable tag used as evidence.
- No claim that a promoted digest is secure; promotion carries evidence, it does
  not replace review.
