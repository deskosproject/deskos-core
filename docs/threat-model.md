# Threat model and publication authority

**Status: reviewed and merged 2026-10-24; the incident exercise ran on 2026-10-28, and the
priority-1 items still open are listed at the end.** This page separates what the repository shows from what only GitHub or
an operator can confirm. Every remediation touching credentials, runners,
registries or branch protection needs its own approval (AGENTS.md).

## Assets

| Asset | Why it matters |
|---|---|
| Source and resource roots | The image's intent; a change here changes every later build |
| Privileged workflows and their secrets | RHEL entitlement, registry push, signing key, R2 token |
| Signing identity | Core uses keyless OIDC; an organization uses a key kept outside its repository |
| Published digests and tags | What machines pull and what users install |
| Evidence | SBOMs, scans, boot results, exceptions and approvals |
| Registry and bucket access | Push, pull and the installer's embedded read-only credential |

## Trust boundaries

1. **Untrusted pull request → unprivileged check.** Core's `ci.yml`, `artifacts.yml`
   and path-filtered `vm-bootcheck.yml` run without publish secrets on
   GitHub-hosted runners. An organization's `ci.yml` runs on its self-hosted,
   entitled runner for pull requests (see gaps).
2. **`main` → privileged build.** Core publishes only from `main`, by the weekly
   run or a `publish=true` dispatch, under branch protection and the `validate`
   check; an organization's build job is `main`-only. Secrets are staged into
   files, not passed in `argv`.
3. **Build host → image.** The Containerfile is generated from the Plan; no
   resource carries a script, command or post-install hook.
4. **Image → machine.** A machine follows its image reference; `deskos update`
   stages a new deployment and a person reboots.
5. **Registry/bucket → consumer.** Pull credentials, signatures and checksums; the
   installer carries a read-only registry credential as a secret file.
6. **Human → merge/publish.** Agents propose, a human reviews and merges, and the
   weekly run then publishes by itself. Promotion moves the digest its run
   published, after verifying the signature and the records; a dispatch publishes or
   promotes on demand. No agent self-merges (AGENTS.md).

## Controls

| Control | State | Evidence |
|---|---|---|
| Pull-request checks get no publish secrets | verified in Core workflows | triggers and `permissions` in `.github/workflows/` |
| Publish is `main`-only and runs the reviewed workflow | verified | `supply-chain.yml` triggers (`schedule`, `workflow_dispatch`) and its `refs/heads/main` guards; an organization's `build-rhel-10.yml` |
| Workflow tokens default to read | verified in the workflow files | `permissions: contents: read`; `id-token: write` only where keyless signing runs |
| Third-party actions pinned by commit | verified for cosign and Renovate | `uses: …@<40-char sha> # version` |
| First-party actions pinned by commit | Core pinned; an organization still uses tags | `actions/checkout@<sha> # v7` and the same for setup/upload/download in Core; an organization is the remaining gap |
| Tools downloaded with a checksum or a digest | verified where recorded | BIB by digest; Syft/Grype installer commit + release checksums; cosign checksums |
| Signing | Core keyless OIDC with manual verification and no enforcement; an organization's key outside the repo, no transparency log, enforcing on update | `supply-chain.yml`, an organization's `docs/supply-chain.md`, [authenticity](design/authenticity.md) |
| Automatic publication is bounded | verified 2026-10-28 | the schedule publishes, but `promotion.yml` moves only the digest its triggering run published (`run_id` in the record), after the keyless signature and every record verifies, and records `automatic (gate)` with the run. See [promotion.md](design/promotion.md) |
| A second human approval before publish | **decided: one person** (2026-10-28) | an environment approval on `evidence-write` would stop the weekly automatic promotion; the bounds above stand in for it. Revisit with a second maintainer or an SLA. See [promotion.md](design/promotion.md) |
| Branch and tag protection | **unverified here** | GitHub setting, not in the repository |
| Registry and GitHub audit-log retention | **unverified here** | external settings |
| Evidence survives a re-run | improved: artifacts carry the run attempt | `docs/checks.md`; `-attempt-${github.run_attempt}` |
| Evidence survives 14 days | **done** | digest-keyed R2 evidence; the first real digest and 16 objects verified from `evidence.deskos.org` |

## Prioritized remediation

| Priority | Item | Owner | Evidence it is done |
|---|---|---|---|
| 1 | Move organization pull-request validation off the self-hosted entitled runner (or isolate it from entitlement, secrets and the host) | organization maintainer | `runs-on` for `pull_request` is GitHub-hosted or a hardened runner; a PR cannot read host secrets |
| 1 | Pin first-party actions by commit SHA | repository maintainer | Core workflows use `uses: owner/action@<sha> # vX`; an organization remains |
| 2 | Verify and record branch/tag protection and required checks | repository owner | exported GitHub settings or an audit screenshot attached to this page's issue |
| 2 | Add a second reviewer or environment approval for `publish=true` and signing, or record why one person is acceptable at this scale | maintainer | **decided 2026-10-28: one person, recorded.** An environment approval would stop the weekly automatic promotion; the automatic move is limited to the digest its run published, after signature and record verification, and a dispatch records its actor. See [promotion.md](design/promotion.md) |
| 3 | Durable evidence index (P0-05) | maintainer | **done:** `evidence.deskos.org` stores and verifies the objects for a published digest |
| 3 | Consumer-side authenticity enforcement (P0-03) | Core maintainer; organization maintainer | **Core:** keyless signatures, manual verification, no enforcement (accepted 2026-10-25); **Organization:** key-based signing with the policy recipe tested on bootc 1.17.1; its policy, installer path and key rotation remain |
| 4 | Rotate and back up the organization's signing key and registry/R2 credentials on a schedule | key holder | rotation record; no secret in the repository |

The remaining priority-1 item most changes the risk: a pull request that runs on an
entitled self-hosted host. Core's first-party actions are pinned by commit; an
organization's are not.

## Incident exercise

**Run 2026-10-28** by the maintainer (deciding) and the agent (facilitating). A
serious finding or a compromised input reaches the release owner: walk one of these,
record the date, the digests and the decisions, and do not publish a recovery until
a human authorizes it.

### What the exercise decided

- **A, compromised upstream base.** Set `PUBLISH_FREEZE` with a sentence saying
  why: the weekly build, the installer, the promotion and the mirror stop, and
  `:stable` keeps serving the last good digest. Re-pin to a known-good earlier
  digest or wait for the vendor's rebuild, then rebuild, scan, boot and publish.
  Machines that already staged the old one keep it unless the user rolls back;
  there is no remote switch, and the advisory says so.
- **B, compromised workflow or action.** Disable the workflow, rotate tokens and
  the signing key as appropriate, and audit its recent runs and artifacts.
  Signatures already issued under the old identity remain valid, which is why the
  advisory names the affected digests rather than trusting deletion.
- **C, compromised signing identity.** Rotate the quay robot, disable the workflow,
  and publish an advisory that **names the affected digests**: deleting a signature
  revokes nothing, and a new signature over an old digest is not a fix. Core's
  keyless certificate is short-lived, but a signature it made remains verifiable;
  an organization's public key is in the repository, so rotation means a new key
  and telling verifiers.
- **D, compromised registry.** Freeze pushes, verify the served digests against the
  recorded `digest.txt`/`image.txt` and signatures, and restore from a trusted push.
  Consumers may have cached or staged images; deleting a tag does not revoke them.
- **E, compromised maintainer account.** Revoke sessions and tokens, review the
  merges, dispatches and releases in the window, and treat every image published in
  it as suspect until rebuilt from a trusted commit. Branch protection and a second
  approval reduce, but do not remove, this risk.

### What the exercise found

- **There was no freeze switch.** Publication could only be stopped by editing four
  workflows across three repositories during the incident. `PUBLISH_FREEZE` now
  does it from the repository settings, and the weekly build, the installer, the
  promotion and the mirror all check it.
- **One person holds every credential.** Revoking after a compromised account
  depends on that account being intact, and there is no second maintainer to do it.
  Accepted at this scale, named here, revisited when a second maintainer joins.

**Deleting a tag or a signature does not revoke an image.** Registries and machines
cache by digest; signatures can be copied; a transparency log is append-only.
Withdrawal means a new advisory, a new digest and a human-authorized replacement.

## Known gaps and non-claims

- The repository cannot enumerate installed machines: there is no telemetry
  (ADR 0011). Affected machines are found from registry/CI records and user
  reports, not a fleet inventory.
- Branch protection, environment reviewers, runner isolation, registry audit
  retention and key custody are **not verified by this page**.
- Vulnerability checks and their exceptions are a different control; see
  [supply-chain.md](supply-chain.md).
- This model does not claim SLSA compliance, a security certification, or that a
  clean scan makes an image safe.
