# Threat model and publication authority

**Status: draft for human review.** This page separates what the repository
shows from what only GitHub or an operator can confirm. It records no secret
values and changes no infrastructure; every remediation that touches credentials,
runners, registries or branch protection needs its own approval (AGENTS.md,
"Requires maintainer approval").

## Assets

| Asset | Why it matters |
|---|---|
| Source and resource roots | The image's intent; a change here changes every later build |
| Privileged workflows and their secrets | RHEL entitlement, registry push, signing key, R2 token |
| Signing identity | Core uses keyless OIDC; DayTwo uses a key kept outside its repository |
| Published digests and tags | What machines pull and what users install |
| Evidence | SBOMs, scans, boot results, exceptions and approvals |
| Registry and bucket access | Push, pull and the installer's embedded read-only credential |

## Trust boundaries

1. **Untrusted pull request → unprivileged check.** Core's `ci.yml`,
   `artifacts.yml` and path-filtered `vm-bootcheck.yml` run without publish
   secrets on GitHub-hosted runners. DayTwo's `ci.yml` is different: it runs on
   the **self-hosted, entitled** runner for pull requests (see gaps).
2. **`main` → privileged build.** Core publishes only on a `publish=true`
   dispatch from `main`; DayTwo's build job is `main`-only. Secrets are staged
   into files, not passed in `argv`.
3. **Build host → image.** The Containerfile is generated from the Plan; no
   resource carries a script, command or post-install hook.
4. **Image → machine.** A machine follows its image reference; `deskos update`
   stages a new deployment and a person reboots.
5. **Registry/bucket → consumer.** Pull credentials, signatures and checksums;
   the installer carries a read-only registry credential as a secret file.
6. **Human → merge/publish.** Agents propose; a human reviews and merges; a
   human dispatches publication. No agent self-merges (AGENTS.md).

## Controls

| Control | State | Evidence |
|---|---|---|
| Pull-request checks get no publish secrets | verified in Core workflows | triggers and `permissions` in `.github/workflows/` |
| Publish is dispatch-only and `main`-only | verified | `supply-chain.yml` (`publish=true`), DayTwo `build-rhel-10.yml` |
| Workflow tokens default to read | verified in the workflow files | `permissions: contents: read`; `id-token: write` only where keyless signing runs |
| Third-party actions pinned by commit | verified for cosign and Renovate | `uses: …@<40-char sha> # version` |
| First-party actions pinned by commit | Core pinned; DayTwo still uses tags | `actions/checkout@<sha> # v7` and the same for setup/upload/download in Core; DayTwo is the remaining gap |
| Tools downloaded with a checksum or a digest | verified where recorded | BIB by digest; Syft/Grype installer commit + release checksums; cosign checksums |
| Signing | Core keyless OIDC; DayTwo key outside the repo, no transparency log | `supply-chain.yml`, DayTwo `docs/supply-chain.md` |
| Publishing needs a human dispatch | verified | no schedule can set `publish=true` |
| A second human approval before publish | **not configured** | Core dispatch is one person; DayTwo's environment has no reviewers |
| Branch and tag protection | **unverified here** | GitHub setting, not in the repository |
| Registry and GitHub audit-log retention | **unverified here** | external settings |
| Evidence survives a re-run | improved: artifacts carry the run attempt | `docs/checks.md`; `-attempt-${github.run_attempt}` |
| Evidence survives 14 days | **gap** | artifact retention and no durable index (P0-05) |

## Prioritized remediation

| Priority | Item | Owner | Evidence it is done |
|---|---|---|---|
| 1 | Move DayTwo pull-request validation off the self-hosted entitled runner (or isolate it from entitlement, secrets and the host) | DayTwo maintainer | `runs-on` for `pull_request` is GitHub-hosted or a hardened runner; a PR cannot read host secrets |
| 1 | Pin first-party actions by commit SHA | repository maintainer | Core workflows use `uses: owner/action@<sha> # vX`; DayTwo remains |
| 2 | Verify and record branch/tag protection and required checks | repository owner | exported GitHub settings or an audit screenshot attached to this page's issue |
| 2 | Add a second reviewer or environment approval for `publish=true` and signing, or record why one person is acceptable at this scale | maintainer | environment protection settings; a dispatch audit |
| 3 | Durable evidence index (P0-05) | maintainer | a historical digest's evidence retrievable after 14 days |
| 3 | Consumer-side authenticity enforcement (P0-03) | maintainer | a policy that rejects unsigned updates, tested |
| 4 | Rotate and back up the DayTwo signing key and registry/R2 credentials on a schedule | key holder | rotation record; no secret in the repository |

The remaining priority-1 item is the one that most changes the risk: a pull
request that runs on an entitled self-hosted host. Core's first-party actions are
now pinned by commit; DayTwo's are not yet.

## Tabletop

A serious finding or a compromised input reaches the release owner. Walk one of
these, record the date, the digests and the decisions, and do not publish a
recovery until a human authorizes it.

**A. Compromised upstream base.** Stop promoting `latest` and announce that the
affected digest is frozen. Re-pin to a known-good earlier digest or wait for the
vendor's rebuild; rebuild, scan, boot and publish a new digest. Machines that
already staged the old one keep it unless the user rolls back; there is no
remote switch.

**B. Compromised workflow or action.** Disable the affected workflow, rotate
tokens and the signing key as appropriate, and audit its recent runs and
artifacts. Signatures already issued under the old identity remain valid, which
is why the advisory names the affected digests rather than trusting deletion.

**C. Compromised signing identity.** Core's keyless certificate is short-lived,
but a signature it made remains verifiable; DayTwo's public key is in the
repository, so rotation means publishing a new key and telling verifiers. A new
signature over the old digest is not a fix; publish a new digest and a new
advisory.

**D. Compromised registry.** Freeze pushes, verify the served digests against
the recorded `digest.txt`/`image.txt` and signatures, and restore the registry
from a trusted push. Consumers may have cached or staged images; deleting a tag
does not revoke them.

**E. Compromised maintainer account.** Revoke sessions and tokens, review recent
merges, dispatches and releases, and treat any image published in the window as
suspect until rebuilt from a trusted commit. Branch protection and a second
approval reduce, but do not remove, this risk.

**Deleting a tag or a signature does not revoke an image.** Registries and
machines cache by digest; signatures can be copied; a transparency log is
append-only. Withdrawal means a new advisory, a new digest and a human-authorized
replacement, not a delete button.

## Known gaps and non-claims

- The repository cannot enumerate installed machines: there is no telemetry
  (ADR 0011). Affected machines are found from registry/CI records and user
  reports, not a fleet inventory.
- Branch protection, environment reviewers, runner isolation, registry audit
  retention and key custody are **not verified by this page**.
- Vulnerability gates and their exceptions are a different control; see
  [supply-chain.md](supply-chain.md).
- This model does not claim SLSA compliance, a security certification, or that
  a clean scan makes an image safe.
