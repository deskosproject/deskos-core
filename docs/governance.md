# Who decides, and what happens if they cannot

DeskOS is small. This page names who can do what, how a decision is taken, and
what the project does when that person is unavailable. It records the gaps.

## Decisions

| Decision | Taken by | Recorded in |
|---|---|---|
| What DeskOS is for, and what it refuses | the maintainer | [mission.md](mission.md), the invariants in [AGENTS.md](../AGENTS.md) |
| A new kind, backend or public model change | the maintainer, after discussion | an ADR |
| A change to compiler, resources, tests or docs | the maintainer, by reviewing and merging a pull request | the pull request |
| Publishing a newer digest | the weekly build, by itself | the run and the records |
| Moving `:stable` | promotion, on that build's success, with an approval record | `approvals/…json` |
| Stopping publication | the maintainer, with `PUBLISH_FREEZE` | the variable and the run that skipped |
| Rotating a credential, key or account | the maintainer | outside this repository |

An agent proposes. It never merges, never pushes to `main`, never touches signing,
secrets, releases or registries, and never weakens a check to go green (ADR 0011).

## Review and release

`main` is protected: a pull request and a green `validate` check, administrators
included. One person reviews and merges. There is no second approver, which is a
recorded decision about build authority in
[promotion.md](design/promotion.md), not an oversight. A release is not a separate
act: the weekly build publishes, promotion moves `:stable` with its record, and
"who released this" is answered by that record.

## Ownership

| Area | Owner | Backup |
|---|---|---|
| Compiler, schema, Plan, backends | the maintainer | none |
| Platforms and pins | the maintainer | none |
| Tests and the VM harness | the maintainer | none |
| Documentation and ADRs | the maintainer | none |
| Supply chain, signing, records | the maintainer | none |
| Registry and bucket credentials | the maintainer | none |

One owner for everything, no backup. The table says so rather than implying a team
that does not exist.

## If the maintainer is unavailable

- **Publication stops.** No new digest is promoted, `:stable` keeps serving the
  last good one, and machines that staged a newer build keep it until they roll
  back.
- **The artifact keeps working.** A published image boots and updates from its own
  reference without the project.
- **Reviews stop.** Pull requests wait; branch protection does not expire.
- **Recovery needs credentials that are not here**: the GitHub organisation, the
  quay robot, the R2 tokens, the `evidence-write` environment and DNS. The maintainer holds
  them in a password manager, and **no backup custodian exists**. That is the real
  continuity risk.

## Credentials and backups

- Nothing secret is in this repository or in the Terraform state.
- The records bucket is the durable part: deleting a tag or a signature does not
  revoke a digest, and a verifier reads the index.
- The Terraform state lives on the maintainer's machine with a copy; the bucket for
  it exists, and moving it there is the remaining step.
- **No restore has been tested from a backup.** That is a gap.

## Becoming a maintainer

By demonstrated competence and an explicit appointment: reviewed patches, a real
defect found, a platform carried through a release. Not by a contributor count and
not by a vote. Privileges are granted separately: review, then merge, then signing
and registry access, each its own decision.

## Not here

No committee, no steering group, no elections, no response-time promise. A
governance structure larger than the code would be a costume.
