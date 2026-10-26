# Promotion of one tested digest

**Status: design proposal, 2026-10-25. No channel, registry tag or approval
process is created by this document.**

P0-06 asks for build-once promotion: one image digest moves through stages
without being rebuilt, carrying its evidence and a recorded human decision.
Today the gate builds, scans and boots an image, then moves `:latest` to that
digest on a `publish=true` dispatch. There is no candidate→stable path, and a
promotion is not recorded as an artifact.

## What is true today

- The publish job builds **one** image per platform leg, boots that same
  `$IMAGE`, pushes `:<commit>`, and moves `:latest` by digest after the checks.
- `:<commit>` is a naming convention, not registry-enforced immutability. A
  digest is the exact identity.
- Evidence is a 14-day workflow artifact (attempt-suffixed) and, for a
  published digest, a durable digest-keyed index under `evidence.deskos.org`
  (P0-05, implemented 2026-10-26). The attestation names the SBOM hash; the
  index resolves it to bytes.
- Machines follow the reference they were installed from; `:latest` is mutable,
  a digest is not. `deskos update` stages whatever that reference now serves.
- Publication requires a human dispatch; there is no second reviewer or
  environment approval by default (P0-04).

## Model

A **channel** is a registry tag that points at one digest. It is not an image
and it does not imply a rebuild.

| Channel | Meaning | Entry | Exit |
|---|---|---|---|
| `candidate` | built and gate-checked, not recommended for general use | publish dispatch | evidence index complete, scan/boot green, human authorization |
| `canary` | a small set of opted-in machines may update | candidate evidence plus a named canary owner | agreed observation window with no blocking regression |
| `stable` | the channel the page and documentation recommend | canary result plus human authorization | replaced by a later digest, or withdrawn |

All four stages are not mandatory at this project's scale. A platform may stop at
`candidate → stable`; the important part is that every move names a digest, the
evidence and a person.

## Promotion is a move, not a build

1. Resolve the source tag to a digest and verify the image signature and the
   evidence index for that digest.
2. Check the stage's entry conditions: scan result, boot/session/first-boot
   results, SBOM/Plan hashes, exception record and review dates.
3. Write an approval record: digest, from-channel, to-channel, person, time,
   evidence index hash and a reason. Store it with the evidence.
4. Move the destination tag **by digest** (a registry-side tag update), never
   by rebuilding. `:<commit>` remains as the build record.
5. Verify from a clean machine that the destination tag resolves to the same
   digest and that its evidence is retrievable.

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

## Open decisions

- Channel names and whether they exist per platform repository.
- Who authorizes each move, and whether a second person is required at this
  scale (P0-04).
- Canary selection without telemetry: opt-in machines and a named owner, or no
  canary stage at all.
- Retention: which digests/channels stay after a promotion; the previous stable
  digest should remain for rollback and evidence.
- Whether the approval record is a signed attestation, a file in the evidence
  index, or both.
- How OCI referrers and the evidence store interact if both are kept.

## Non-goals

- No fleet controller, no automatic rollout, no percentage-based canary.
- No rebuild to promote, and no mutable tag used as evidence.
- No claim that a promoted digest is secure; promotion carries the evidence, it
  does not replace review.
