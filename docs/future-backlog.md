# Future backlog

> **Dated review, 2026-10-20: definitions, not status.** Statuses hold for that
> date only. What closed, and with which records, is the closeout table in
> [roadmap.md](roadmap.md#closeout-of-the-p0p1-items-2026-10-28). Do not update
> statuses here.

Strategic review of checkout `171e8cc` (CI snapshot `5532f31`), for which no image
was built. Repository records establish implementation and test scope; uncommitted
code does not count.

**Proposed** means eligible for selection, not scheduled. **Research** means a
bounded investigation whose output is a decision. **In progress**, **Blocked** and
**Done** require records. Scope **S/M/L** is a focused document or probe / several
connected changes / multiple reviewable slices, not a time estimate.

Activate one bounded slice when a maintainer records: a reproducible defect or a
validated organizational need; a named owner with review capacity; the upstream
alternatives and non-goals; a test or record plan; the compatibility and security
impact; human agreement. Never a whole tier. Research may end with no action.

## What the review found

| State | Boundary |
|---|---|
| Deterministic compiler, fourteen kinds | architecture, composition and golden tests; does not establish image reproducibility or API stability |
| Organization example | separate root, CentOS and private RHEL targets; `virt-manager` explicitly unmet (P1-03) |
| Supply-chain pipeline | builds, scans, boots, signs, verifies, moves `latest` by digest; promotion and consumer enforcement were absent at review |
| Declared vs installed SBOM | comparison checks declared RPM names, not every component; the signed statement referenced bytes that expired in 14 days |
| Runtime companion | `report`, `check`, `update`; `bootc` owns status and rollback; no lifecycle E2E at review |
| Local keys, APT, Ubuntu split | ADRs 0015, 0019, 0020; Ubuntu and RHEL build downstream |
| CI and input updates | workflows and Renovate exist; coverage and authorization still need review |
| Fedora boots in CI (closed, #91) | initramfs fix; run `39462806315`; nothing published, nothing installed on hardware |
| Participation policies | absent at review: `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `SUPPORT.md`, governance |

## P0: trustworthiness and operational correctness

<a id="p0-01"></a>
### P0-01: Make failed checks actionable
**Proposed · M.** Audit the required checks and their KVM, disk and RAM needs; keep
first-attempt artifacts, since the publish workflow uploads summaries, not
diagnostics; relate local image ID, scan input, disk and registry manifest; one
publisher, not two.
**Done when:** infrastructure and image failures are distinguishable from serial,
journal, results and frames kept on failure, a passing retry stays visible, and the
scheduled run has a demonstrated result.
[Harness](../tests/vm/README.md) · [Lint warnings](architecture.md#lint-warnings)

<a id="p0-02"></a>
### P0-02: Prove upgrade, rollback and state recovery
**Proposed · L.** Take one machine through successive deployments on CS10: state →
B → reboot → identity and state → rollback to A → verify, plus an interrupted
upgrade and a candidate that cannot boot, recovered through the previous entry.
Fault injection stays in the lab.
**Done when:** a CI recipe shows A → B → A and a recovery, with journals,
deployment identities, unchanged and edited `/etc`, and a `$HOME`-free `/var`
change; `/var` and Flatpaks are recorded as not rolled back.
[Operations](operations.md) · [bootc rollback](https://bootc.dev/bootc/man/bootc-rollback.8.html)

<a id="p0-03"></a>
### P0-03: Establish consumer-side enforcement
**Research · M.** Make a machine, not CI, reject an unauthorized image. Evaluate the
signature policy of the bootc and containers-image versions in use at install,
switch and upgrade, with the exact workflow identity, not a repository-wide regex.
**Done when:** authorized, unsigned, wrong-identity, tampered, other-digest,
rotation and lost-trust cases all behave, and the path for a new trusted policy to
reach a compromised machine is written down.
[ADR 0018](adr/0018-the-runtime-companion.md) · [policy format](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md)

<a id="p0-04"></a>
### P0-04: Bound build authority, rehearse incidents
**Proposed · M.** Document the PR → privileged build → records → publish
boundaries; review branch protection, tokens, OIDC scopes and action pins; run a
exercise (compromised base, workflow, signing identity or registry) that freezes
channels, finds affected digests and publishes a recovery.
**Done when:** a reviewed threat model names an owner per control or gap, and the
exercise produced a working freeze and a recovery path.
[Threat model](threat-model.md) · [ADR 0011](adr/0011-agentic-contribution-layer.md)

<a id="p0-05"></a>
### P0-05: Preserve digest-indexed records
**Proposed · L.** SBOM bytes expired after 14 days and attestations bound only a
reference. Choose durable storage (OCI referrers or an immutable store), then an
index joining commit, workflow, base digest, compiler version, Plan and context
hashes, image digest, both SBOMs, scanner and database, exceptions, tests and
authorization.
**Done when:** a historical digest's records can be retrieved and verified after
workflow artifacts expire, and missing or mismatched records fail.
[ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md) · [supply chain](supply-chain.md)

<a id="p0-06"></a>
### P0-06: Promote one tested digest
**Proposed · M.** Build once, promote by digest, with per-platform references,
immutable records and a recorded authorization; define entry and exit checks, stale
scans, withdrawal, channel rollback and a failure halfway through.
**Done when:** a reviewed design traces one digest through promotion without a
rebuild, verifies records at each step, and records `:<commit>` as a naming
convention, not immutability.
[Release model](architecture.md#release-model)

<a id="p0-07"></a>
### P0-07: Review released-image risk over time
**Proposed · M.** The weekly rebuild does not rescan every published digest.
Inventory supported digests, rescan those images, prioritise base re-pins and assess High findings for
reachability, keeping raw findings beside dated OpenVEX decisions.
**Done when:** selecting an old digest, recording scanner versions and routing a new
finding to revalidation, exception or withdrawal is a procedure someone else can run.
[ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md) · [supply chain](supply-chain.md)

<a id="p0-08"></a>
### P0-08: Make upstream inputs deliberate
**Proposed · M.** Inventory bases, repositories and keys, binaries, extensions, the
companion, builders, scanners and actions; document propose → human review → resolve
and hash → render diff → build, boot and scan; audit Renovate against it.
**Done when:** one base re-pin and one version/URL/checksum update have reviewable
records and each input is mapped to owner, trigger and check.
[tracking.md](design/tracking.md) · [ADR 0019](adr/0019-a-platform-says-whether-its-base-can-fetch.md)

## P1: product stability and usability

<a id="p1-01"></a>
### P1-00: Install image mode at scale over the network
**Proposed · M.** The first DeskOS shipped by booting from the network: DHCP and
PXE handed the machine to Anaconda, and the desktop installed itself. Image mode
keeps that path: Satellite or Foreman and Katello provision a bootc machine from a
trimmed kickstart whose `ostreecontainer` (or the newer `bootc`) command names the
image, with `inst.stage2` in the kernel options and registry credentials in `%pre`
(`/etc/ostree/auth.json`). Nothing in Satellite needs to change; what is missing is
ours: a kickstart template a maintainer can drop in, the image named by **digest**
rather than tag, and a page that says which layer owns what (architecture.md,
"What each layer owns").
**Done when:** the template and the page exist, and a machine provisioned that way
boots into the same session the ISO does, with the digest recorded.
Red Hat's recipe: <https://developers.redhat.com/articles/2026/08/06/image-mode-for-rhel-with-anaconda-and-satellite>

<a id="p1-01"></a>
### P1-01: Define the compatibility contract
**Proposed · M.** `v1alpha1` has had deliberate breaks, and schemas and goldens pin
behaviour, not a policy. Define compatibility for resources, Plan, CLI output and
exit codes, deprecation and migration, and the criteria for beta and 1.0.
**Done when:** a documented policy and fixture matrix separate compatible, rejected
and migratable inputs, and an outside use case exists before more kinds.
[compatibility.md](compatibility.md)

<a id="p1-02"></a>
### P1-02: Bound platform support and retirement
**Proposed · M.** The matrix separates composition, build, boot and publication but
promises nothing about maintenance. Add maintainers, security maintenance, upgrade
paths, exit criteria and consumer notices.
**Done when:** every platform and downstream boundary has a support statement and a
retirement process, with no LTS inferred from a vendor lifecycle.
[platforms.md](platforms.md) · [ADR 0014](adr/0014-platform-base-ownership.md)

<a id="p1-03"></a>
### P1-03: One public adoption journey
**Proposed · M.** Extend the CentOS example guide through obtaining a matching
`deskosctl`, validating, customizing, inspecting Plan and SBOM, rendering, building a
UEFI QCOW2, creating a first user, updating and recovering.
**Done when:** an unfamiliar engineer completes it without private steps, recording
versions, digests and prerequisites, and the gaps they hit are recorded.
[WALKTHROUGH](../examples/organization-and-role/WALKTHROUGH.md)

<a id="p1-04"></a>
### P1-04: Correct guidance, make recovery discoverable
**In progress · M.** Audit stale claims (downloads, AppIndicator, Ubuntu-era VM
steps, example counts) against code and ADRs; add symptom → record → remedy pages for
install, first boot, registry, update and recovery, linked from the README.
**Done when:** a platform status change needs one matrix edit, and documented and
experimental paths are recorded per platform.
[docs index](README.md) · [installer ISO](installer-iso.md)

<a id="p1-05"></a>
### P1-05: Diagnostics useful to outside authors
**Proposed · M.** Walk representative failures through `validate`, `plan`, `render`,
`sbom` and the companion; improve resource, file and layer context and the next
action; machine-readable codes only against a real consumer.
**Done when:** missing resources, same-layer conflicts, invalid references,
unavailable packages, missing download capability and absent runtime records each
have an actionable diagnostic with positive and negative examples.
[diagnostics.md](diagnostics.md)

<a id="p1-06"></a>
### P1-06: Test what vendor updates break
**Proposed · L.** Extend the harness incrementally: favorite launches, harness
browser content, GDM login and Wayland, logo and wallpaper, extension loads and the
watcher, offline login, Flatpak after reconnect, screen locking, accessibility probes
where they help.
**Done when:** a behaviour matrix ties each promoted image's claims to positive and
negative probes, separated into instrumented and unmodified boots.
[Harness](../tests/vm/README.md) · [ADR 0016](adr/0016-vendor-desktop-and-app-menu.md)

<a id="p1-07"></a>
### P1-07: Physical-hardware validation
**Proposed · M.** With volunteered hardware and an owner, record one UEFI laptop and
one desktop across available Intel and AMD graphics: Wi-Fi, Bluetooth, suspend,
displays, audio, camera and encryption where configured.
**Done when:** results name device class, firmware, digest and installer path without
personal identifiers, and untested combinations are labelled.
[platforms.md](platforms.md)

<a id="p1-08"></a>
### P1-08: Predictable update behaviour
**Proposed · M.** Document and test staged visibility, ordinary versus explicit
reboot, timer and companion concurrency, failures, offline and reconnect, Flatpak
independence.
**Done when:** a scenario table shows the actual behaviour and logs for automatic off
and on, battery skip, AC return, metered, offline and a failed download.
[operations.md](operations.md) · [ADR 0007](adr/0007-updatepolicy.md)

## P2: community and enterprise readiness

<a id="p2-01"></a>
### P2-01: Publish participation entry points
**Proposed · S.** `CONTRIBUTING.md`, `SECURITY.md` and concise support and conduct
pages, with a maintainer-confirmed private reporting route; link the agent rules
rather than writing another AI policy.
**Done when:** an outsider reproduces the checks and submits a bounded patch without
private instructions, and knows where to report a vulnerability.
[AGENTS.md](../AGENTS.md)

<a id="p2-02"></a>
### P2-02: Governance and continuity
**Proposed · M.** Define reviewer and maintainer duties, appointments, decision and
disagreement paths, RFC and ADR triggers, release approval and transfer; map
ownership of compiler, platforms, tests, docs and supply chain.
**Done when:** a dry run of the absence shows who can review, stop a release, communicate
and recover infrastructure, or records that nobody can.
[governance.md](governance.md)

<a id="p2-03"></a>
### P2-03: Validate independent adoption
**Proposed · M.** Two or three teams author and maintain their own roots. Compare a
bounded baseline change in their Containerfiles and CI against DeskOS profiles:
duplication, effort, conflicts, provenance, portability, cost.
**Done when:** two external environments document onboarding, update, recovery and
maintenance, and a ranked follow-up list exists before any new abstraction.
[Organization example](../examples/organization-and-role/README.md)

<a id="p2-04"></a>
### P2-04: Make releases and findings followable
**Proposed · S.** Concise release and migration notes, retirement notices, one
proposal and support venue, decisions in ADRs, and concrete upstream topics drawn
from measured work.
**Done when:** a user can identify what a release changes for them and what to do,
and proposed versus committed work is explicit.
[release.yml](../.github/workflows/release.yml)

<a id="p2-05"></a>
### P2-05: Prepare a conference proposal
**Proposed · S.** Draft the problem, the typed Plan and composition, a CS10
organization demo, the records, the limitations and a collaboration request as a
25-minute outline.
**Done when:** an abstract and a reproducible demo outline are ready to submit;
submission is a separate, explicitly approved action.
**CFP (verified 2026-10-20):** closes 2026-12-04 23:59 Europe/Brussels; the event is
2027-01-28/29 in Brussels. Recheck before submitting.

<a id="p2-06"></a>
### P2-06: External enrolment and restricted networks
**Research · M.** With a pilot, scope one FreeIPA or AD/SSSD/Kerberos lab: client
packages and CA trust in the artifact, per-machine enrolment, emergency access and
credentials in existing provisioning tools; mirrors only if that lab needs them.
**Done when:** artifact, provisioning and enrolment ownership are separated, and
offline first boot, credential expiry, server outage and emergency access are
covered.
[ADR 0005](adr/0005-managed-baseline-boundary.md)

<a id="p2-07"></a>
### P2-07: Map compliance to existing records
**Research · M.** Pick one real requirement and map artifact properties to
OpenSCAP/SSG, CycloneDX, OpenVEX and provenance; keep build-time and
installed-machine checks distinct.
**Done when:** a scoped workflow names control, producer, subject digest, limitation
and verification step, including missing records.
[ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md)

<a id="p2-08"></a>
### P2-08: Portable factory outputs
**Research · M.** Specify the minimum contract (compiler and resources version,
Plan, rendered context and manifest, OCI subject digest, installed SBOM, provenance,
tests and authorization), and probe a policy engine against GitHub-generated records
before considering another platform.
**Done when:** an independent verifier checks identity, provenance, SBOM association
and required tests, with negative cases, and the adapters needed are recorded.

## P3: strategic research, no commitments

Each needs a sponsor, a bounded question and a maintenance comparison.

- **P3-01 Alternative build infrastructure · M.** Compare the same context and record
  contract on GitHub and Konflux: isolation, policy, cost, QEMU/KVM, privileged
  workloads, staffing. Done when there is a written keep/adopt/reject with measured
  costs.
- **P3-02 Watch upstream bases and GNOME · S.** A source-linked checklist for new
  bases and desktop changes, with risks fed into P1-06. Done when each watch item has
  an adoption threshold; "no action" is a valid result.
- **P3-03 Secure-installation responsibility · M.** Map Secure Boot, TPM2, LUKS
  recovery, measured boot and factory provisioning to firmware, vendor, hardware and
  organization. Done when a design and lab plan state unsupported combinations.
- **P3-04 aarch64 · M.** Inventory compiler assets, bases, packages, inputs, runners,
  builders and hardware. Done when the assessment names users, an owner and an
  executable build/boot matrix, or declines.
- **P3-05 Reproducibility gaps · M.** Compare two controlled builds and inventory what
  differs: snapshots, bases, keys, binaries, runtimes, tools, timestamps. Done when
  the gaps that matter to audit or recovery are named, and lockfiles are proposed only
  for demonstrated gaps.
- **P3-06 Agent assistance and UX · S.** Pick one explanation or UX problem from pilot
  findings and compare manual with agent-assisted use. Done when a measurable obstacle
  maps to a bounded improvement that passes the ordinary offline validator and human
  review.

## Deferred decisions and non-goals

- **Not planned:** fleet management, an Ansible/Satellite/MDM replacement, endpoint
  reconciliation, a DeskOS identity service (ADR 0001).
- **Not planned:** shell hooks, raw dconf escape hatches, templating, personal
  development environments, `$HOME` management (ADRs 0002/0005).
- **Not planned:** maintaining GNOME, Mutter or distribution package forks; unlimited
  editions; a customization race.
- **Not planned:** AI-dependent compilation, a proprietary model requirement, custom
  agent orchestration in Core, autonomous release authority (ADR 0011).
- **Not planned:** a permanent dependency on any CI provider.
- **Not planned:** unsupported LTS guarantees, automatic platform expansion,
  publishing RHEL derivatives, or bringing Ubuntu builds back into Core
  (ADRs 0004/0014/0020).
- **Deferred:** `TrustPolicy`, `ComplianceProfile`, `TimeSyncPolicy`,
  `PerformancePolicy`, `FirmwarePolicy`, `IdentityProvider`, `WebApplication`,
  external providers, a custom update orchestrator. External need, a proven minimal
  abstraction and an approved ADR come first.
- **Deferred:** third-party theme import, session `desktopLogo`, Activities-button
  recolouring.
- **Deferred:** prebuilt disk or ISO hosting, registry mirrors, a second simultaneous
  Fedora release, NVIDIA and hardware-specific support.

## Decisions reserved for human review

Humans choose consumer trust and rotation (P0-03/P0-04), record retention and signing
of release assets (P0-05), publication authority (P0-06), API promotion (P1-01),
support promises (P1-02), reporting contacts and delegation (P2-01/P2-02). A
vulnerability threshold change conflicts with ADR 0012 until a future ADR accepts it.
New kinds, metered-update behaviour beyond ADR 0007, a boundary change or a new
platform ownership model need architectural review first.
