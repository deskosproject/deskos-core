# Who decides, and what happens if they cannot

DeskOS is small. This page says who can do what, how a decision is taken, and
what the project does when the person who can is not there. It records the gaps
instead of pretending they are covered.

## The decisions that exist

| Decision | Taken by | Recorded in |
|---|---|---|
| What DeskOS is for, and what it refuses | the maintainer | [mission.md](mission.md), the invariants in [AGENTS.md](../AGENTS.md) |
| A new resource kind, backend or public model change | the maintainer, after discussion | an ADR under `adr/` |
| A change to compiler, resources, tests or docs | the maintainer, by reviewing and merging a pull request | the pull request, Conventional Commits |
| Publishing a newer digest | the weekly gate, by itself | the gate run and the evidence index |
| Moving `:stable` | the promotion, on the gate's success, with the approval record | `approvals/…json` |
| Stopping publication | the maintainer, setting `PUBLISH_FREEZE` | the variable and the run that skipped |
| Rotating a credential, a key or an account | the maintainer | outside the repository (see below) |

An agent proposes; it never merges, never pushes to `main`, never touches
signing, secrets, releases or registries, and never weakens a check to go green
(ADR 0011).

## Review, merge and release

`main` is protected: a pull request and a green `validate` check are required,
administrators included. One person reviews and merges; there is no second
approver, and this is a recorded decision for build authority
([promotion.md](design/promotion.md)) rather than an oversight. A release is not a
separate act: the gate publishes and the promotion moves `:stable` with its
record, so "who released this" is answered by the evidence, not by a meeting.

## Ownership

| Area | Owner | Backup |
|---|---|---|
| Compiler, schema, Plan, backends | the maintainer | none |
| Platforms and their pins | the maintainer | none |
| Tests and the VM harness | the maintainer | none |
| Documentation and ADRs | the maintainer | none |
| Supply chain, signing, evidence | the maintainer | none |
| Registry and bucket credentials | the maintainer | none |

The table is honest: there is one owner for everything and no backup. It is named
here so a reader does not infer a team that does not exist.

## If the maintainer is unavailable

The absence scenario, walked:

- **Publication stops.** No new digest is promoted; `:stable` keeps serving the
  last good one, and machines that already staged a newer one keep it until they
  roll back. Nothing breaks, and nothing moves silently.
- **The artifact keeps working.** A published image does not need the project to
  boot or update from its own reference.
- **Reviews stop.** Pull requests wait; the branch protection does not expire.
- **Recovery needs the credentials**, and they are not in this repository: the
  GitHub organisation, the quay robot, the R2 tokens for the evidence and the
  download bucket, the evidence environment, and the DNS. They are held by the
  maintainer in a password manager. **No backup custodian exists**, so a
  successor would have to recover them through the provider's account recovery —
  that is the real continuity risk, and this page does not pretend otherwise.

## Credentials and backups, without values

- Nothing secret is in the repository or in the Terraform state
  ([infrastructure README](https://github.com/deskosproject/deskos-infrastructure)).
- The evidence bucket is the durable record: deleting a tag or a signature does
  not revoke a digest, and the index is what a verifier reads.
- The Terraform state lives on the maintainer's machine with a copy; the bucket
  for it exists and moving it there is the remaining step (the infrastructure
  README has the procedure).
- **No restore has been tested from a backup.** That is a gap, not a plan.

## Becoming a maintainer

By demonstrated competence and an explicit appointment: someone who has had
patches reviewed, found a real defect, or carried a platform through a release.
Not by a contributor count and not by a committee vote. Privileges are granted
separately and explicitly: review, then merge, then signing and registry access,
each its own decision.

## What is not here

No committee, no steering group, no elections, and no response-time promise. A
project this size with a governance structure larger than its code is a costume.
