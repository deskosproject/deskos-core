> **Dated review, 2026-10-20 — kept as the definition, not as status.** This is
> the review that introduced the P0/P1 numbering, and the only place each item is
> defined. The statuses below are the status *at that date*. What is done, and
> with which evidence, is the closeout table in
> [roadmap.md](roadmap.md#closeout-of-the-p0p1-items-2026-10-28); do not update
> statuses here.

# Future backlog

Strategic review dated **2026-10-20**, reviewed through committed checkout
`171e8cc`, including the extension hardening and corrected Fedora claims that
landed during this review. The CI snapshot below is for `5532f31`; where a
finding was closed after it, the row says so and names the run.
No image was built or
tested for this review. Repository evidence establishes implementation and test
scope; historical run results remain historical. Uncommitted code is not evidence
of completed work.

Start with the [eight recommended actions](roadmap.md#strategic-direction-and-immediate-focus-2026-10-20).
This document holds the detailed backlog; [roadmap.md](roadmap.md) retains the
milestone history. [Platforms](platforms.md) remains the sole current platform
matrix, [architecture](architecture.md#validation-status) holds validation
evidence, and [ADRs](adr/README.md) govern design. This review does not expand
support or override an ADR.

**Navigate:** [Evidence](#what-the-review-found) ·
[P0](#p0--trustworthiness-and-operational-correctness) ·
[P1](#p1--product-stability-and-usability) ·
[P2](#p2--community-and-enterprise-readiness) ·
[P3](#p3--strategic-research-without-feature-commitments) ·
[Non-goals](#deferred-decisions) · [Human decisions](#decisions-reserved-for-human-review).

## Reading and activating the backlog

IDs are stable even if priorities change. **Proposed** means eligible for human
selection, not scheduled. **Research** means a bounded investigation with a
decision as its output, not a promised feature. **In progress** requires evidence
of existing work; **Blocked** requires an identified unmet prerequisite; **Done**
requires acceptance evidence. Dependencies below constrain implementation, but
need not prevent drafting a design. Scope **S/M/L** means a focused document or
probe / several connected changes / multiple reviewable slices; it is not a time
estimate. Split an L item before authorizing implementation.

Promote work into active development only when a maintainer records:

- A reproducible defect or validated external organizational need, and the
  expected improvement over the current path.
- A named owner with review capacity and an acceptable ongoing support cost.
- Existing upstream solutions evaluated, a bounded scope and explicit non-goals.
- A test or evidence plan, completion criteria and dependencies that can be met.
- Compatibility and security impact, including migration and recovery where needed.
- Human agreement; a future RFC/ADR first if an accepted decision would change.

Do not activate the entire priority tier. Select one bounded engineering slice,
review its evidence, then reconsider the next choice. Pilots may reorder Proposed
work. Research can end with **no action**. No engineering dates are implied.

## What the review found

| State | Evidence and remaining boundary |
|---|---|
| **Done: deterministic compiler foundation** | [Architecture tests](../internal/compiler/architecture_test.go), [composition tests](../internal/compiler/compose_test.go) and [goldens](../internal/compiler/golden_test.go) cover fourteen kinds, layering, provenance, offline planning and deterministic rendering. This does not establish image reproducibility or API stability. |
| **Done: organization extension example** | [Organization and role](../examples/organization-and-role/README.md) references Core from a separate root and offers a public CentOS target plus a private RHEL target. Repositories, binaries, trust and GNOME are present; `virt-manager` remains explicitly unmet. P1-03 completes the guided adoption path. |
| **Done: reference supply-chain gate** | [Publish workflow](../.github/workflows/supply-chain.yml) builds, scans, boots, session-checks and first-boot-checks the local image, publishes a commit tag, signs/attests its digest, verifies it, then moves `latest` by digest. Platforms publish independently. [Recorded publication evidence](architecture.md#validation-status) exists for CentOS and AlmaLinux. Channel promotion and consumer enforcement are absent. |
| **Done: declared/installed SBOM distinction and exception mechanism** | [SBOM code](../internal/sbom/sbom.go), [comparison tests](../tests/supply-chain/test_compare.py) and [exception tests](../tests/supply-chain/test_check_exceptions.py). The comparison gates declared RPM **names**, not every declared component or exact version. The signed installed-SBOM statement references bytes held in a 14-day artifact. |
| **Done: minimal runtime companion and staging policy** | [ADR 0018](adr/0018-the-runtime-companion.md), [companion tests](../cmd/deskos/main_test.go), [guard tests](../internal/evidence/evidence_test.go), [operations](operations.md). `report`, `check`, `update` exist; `bootc` owns status/rollback. Report admits no installed SBOM; check publishes its limited scope. Scheduled staging and a manual CentOS update were previously measured; no automated lifecycle E2E exists. |
| **Done: local repository keys, APT language and Ubuntu separation** | ADRs [0015](adr/0015-package-manager-and-database.md), [0019](adr/0019-a-platform-says-whether-its-base-can-fetch.md), [0020](adr/0020-ubuntu-lives-in-its-own-repository.md). Core keeps synthetic Debian composition/render tests; Ubuntu build/boot/publish belongs downstream. RHEL builds are also downstream, with one historical Core factory validation. |
| **Partially implemented: CI and updates to inputs** | [VM workflow](../.github/workflows/vm-bootcheck.yml), [artifact probes](../.github/workflows/artifacts.yml), [pin existence check](../.github/workflows/ci.yml), [Renovate workflow](../.github/workflows/renovate.yml) and [configuration](../renovate.json) exist. The old “deploy Renovate” entry is stale as an implementation description; operation, coverage and authorization still need review. No credential inspection was performed. |
| **Fedora boots and is checked in CI** (closed with #91) | The regenerated initramfs now adds the `ostree` and `bootc` dracut modules and asserts `ostree-prepare-root` ([research notes](research-notes.md#fedoras-rebuilt-initramfs-dropped-ostree-and-bootc-2026-10-20)). [Run 39462806315](https://github.com/deskosproject/deskos-core/actions/runs/39462806315) booted a **btrfs** disk to a graphical screen in 30 s and passed the session and first-boot checks, with the tray extension active and its watcher answering. No Fedora image is published, no Fedora disk has been installed on hardware, and both community images were republished with that initramfs (CentOS `sha256:30878af4…`, Alma `sha256:bde415d7…`). |
| **Absent from the tree: public participation policies** | No `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `SUPPORT.md` or governance document was found. Engineering and agent rules already exist; remote reporting settings and branch protections were not audited. |

The latest extension decision, [ADR 0021](adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md),
already permits verified, pinned upstream extension archives where vendor packages
are unavailable. Preserve that bounded delivery model; do not propose a packaging
fork. Its declared UUID/version checks and session load checks are useful starting
evidence, not proof that every application's tray behavior works.

## P0 — Trustworthiness and operational correctness

<a id="p0-01"></a>

### P0-01 — Make gate failures actionable and preserve the tested identity

**Status: Proposed · Scope: M.** Boot retries, privileged disk builds and scarce
runner resources make a green/red result hard to interpret. The splash retry is
bounded to one specific failure, but its cause remains unexplained. The publish
workflow uploads summaries rather than the full diagnostic directories.

**Action:** Audit required merge/release gates, path coverage and measured runner
KVM, disk, RAM and timeout needs. Classify harness/infrastructure, image defects
and unverified probes separately; preserve first-attempt results. Record the
relationship between local image ID, exported scan input, disk and registry
manifest, accounting for compression changing OCI digests. Reuse the existing
unified gate; do not create a second publisher.

**Acceptance:** Reproduce the splash failure or record a bounded investigation
with an explicit unresolved result; demonstrate distinct infrastructure and image
failures with serial/QEMU logs, guest journal, structured results and frames
retained even on failure. A passing retry remains visible. Required probe reads
cannot silently count as verified. Resolve the pending Fedora run before closing
its regression; retest btrfs separately after ext4 works. Record a lint-warning
disposition per measured vendor warning without patching packages to silence it.
Document actual required checks and prove tested/published identity continuity.
Verify scheduled versus dispatched gate behavior: the scheduled supply-chain job
reads `inputs.fail-on` without an explicit fallback in the workflow, so the
Critical baseline needs a demonstrated schedule-path result too.

**Dependencies:** None. **References:** [VM harness](../tests/vm/README.md),
[publish workflow](../.github/workflows/supply-chain.yml),
[lint warnings](architecture.md#lint-warnings), [Fedora investigation](research-notes.md#fedoras-rebuilt-initramfs-dropped-ostree-and-bootc-2026-10-20).

<a id="p0-02"></a>

### P0-02 — Prove upgrade, rollback and persistent-state recovery

**Status: Proposed · Scope: L.** Existing checks start independent overlays;
they do not exercise a persistent workstation through successive deployments.

**Action:** Start on CentOS Stream 10 with known-good image A. Create disposable
user data and system state, then upgrade to B, reboot, assert deployment identity
and state, roll back to A and verify again. Exercise an interrupted/rejected
upgrade and a deliberately non-booting candidate recovered through the previous
boot entry. Keep fault injection in the external test lab, never a resource hook.

**Acceptance:** A reproducible CI recipe shows a successful A → B → A cycle and
at least one meaningful boot recovery, with journals and deployment identities
for every stage. Include unchanged and locally edited `/etc` files, `/var` and a
synthetic application-data change. Specify deployment-specific `/etc` merge and
rollback behavior; `/var`, homes, system Flatpaks and application migrations are
not automatically undone. Demonstrate backup/restore or document an incompatible
data migration that requires operator recovery. Record manual versus automatic
recovery accurately; reuse downstream contracts before claiming other platforms.

**Dependencies:** P0-01. **References:** [Milestones 3–4](roadmap.md#milestone-3-vm-boot-and-desktop-tests),
[operations](operations.md#roll-back), [upstream rollback semantics](https://bootc.dev/bootc/man/bootc-rollback.8.html).

<a id="p0-04"></a>

### P0-04 — Bound build authority and rehearse supply-chain incidents

**Status: Proposed · Scope: M.** Manual dispatch and signatures alone do not
establish independent publication approval or recovery from stolen authority.

**Action:** Document PR → privileged build → evidence → publish trust boundaries.
Review branch/tag protection, required checks, untrusted PR execution, workflow
tokens/OIDC scopes, action commit pins, tool downloads, registry access and
human approval. Some actions use version tags today; actual remote protection
settings remain unverified. Separate development and publication authority where
practical for the current maintainer capacity.

**Acceptance:** A human-reviewed threat model and prioritized remediation list
identify owners and evidence for each control or gap. A tabletop exercise covers
compromised upstream base, workflow, signing identity or registry: freeze channels,
identify affected digests, notify users, restore trusted inputs and publish a
human-authorized recovery. Explain why deleting tags/signatures does not revoke
cached images or signatures. Document backup access and restore responsibilities
without recording secrets; any real credential or infrastructure change requires
separate explicit approval. Coordinate reporting with P2-01 and succession with
P2-02.

**Dependencies:** None for the model; P0-03/P0-06 consume its decisions.
**References:** [workflows](../.github/workflows),
[agent authority limits](../AGENTS.md), [ADR 0011](adr/0011-agentic-contribution-layer.md).

<a id="p0-03"></a>

### P0-03 — Establish consumer-side authenticity enforcement

**Status: Research · Scope: M.** CI's `cosign verify` does not demonstrate that
installers or installed workstations reject unauthorized images.

**Action:** Produce a threat model and disposable-lab prototype plan for the
actual vendor versions of bootc/containers-image. Evaluate signature-policy
enforcement at install, switch and subsequent upgrade, and whether the current
cosign signature/attestation storage is consumable by that path. Specify exact
workflow/ref/issuer authorization rather than treating a repository-wide identity
regex as release authorization. Specify installer or organization-image provisioning
of public trust roots; credentials remain external. Existing TLS `TrustAnchor`
is not an image-signature trust policy.

**Acceptance:** Document supported mechanisms, version constraints and executable
positive/negative cases: authorized, unsigned, wrong identity, tampered bytes,
evidence for another digest, identity rotation and loss of trust in a previously
valid release. Define how the next trusted policy reaches a machine on a
compromised release, emergency recovery and offline limits. Include both timer
and companion update paths; a manual verification command alone is insufficient.
Choose no new kind unless a working experiment proves a need and a human approves
a future ADR.

**Dependencies:** P0-04 threat model; P0-02 supplies the recovery lab.
**References:** [ADR 0018](adr/0018-the-runtime-companion.md),
[bootc switch](https://bootc.dev/bootc/man/bootc-switch.8.html),
[containers signature policy](https://github.com/containers/image/blob/main/docs/containers-policy.json.5.md).
These upstream mechanisms are candidates, not a verified DeskOS integration.

<a id="p0-05"></a>

### P0-05 — Preserve digest-indexed evidence and verifiable provenance

**Status: Proposed · Scope: L.** Installed SBOM bytes expire after 14 days;
current attestations bind an SBOM reference and exceptions, not the complete
source/build/test/authorization chain. Downloaded CLI/companion binaries have
checksums but no signed source-to-binary attestation.

**Action:** First choose durable evidence storage after testing OCI referrers and
an alternative immutable store against registry size, retention, garbage
collection, access and cost constraints. Then design a verifiable evidence index
joining source commit, workflow identity, base digest, compiler version, canonical
Plan/context hashes, produced image digest, declared and installed SBOMs, scanner
and database identity, exceptions, test results and publication authorization.
Evaluate GitHub Artifact Attestations and SLSA requirements; distinguish claims
made by the build itself from independently protected evidence generation.

**Acceptance:** A chosen storage design and lab retrieval/verification procedure
recover a historical digest's full evidence after workflow artifacts expire;
missing, changed or mismatched evidence fails verification. Demonstrate copying
associated evidence across promotion/mirror boundaries. Specify an authenticity
path for release binaries and the resources archive. Explain endpoint evidence
discovery under ADR 0010 stage 4: embedding a post-build SBOM changes the image
digest and requires a deliberate design, not a recursive digest promise. No SLSA
level claim without a requirement-by-requirement assessment.

**Dependencies:** P0-01 identity records and P0-04 trust boundaries; retention
design can begin first. **References:** [supply chain](supply-chain.md),
[ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md),
[release workflow](../.github/workflows/release.yml),
[OCI referrers](https://specs.opencontainers.org/distribution-spec/),
[GitHub attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations),
[SLSA requirements](https://slsa.dev/spec/v1.2/build-requirements).

<a id="p0-06"></a>

### P0-06 — Design promotion of one tested digest

**Status: Proposed · Scope: M.** The existing gate moves `latest` after checks;
candidate → canary → pilot → stable is still a design proposal.

**Action:** Define build-once promotion with per-platform references, immutable
evidence and recorded human authorization. Specify entry/exit checks, stale scan
handling, withdrawal, channel rollback and failure partway through publication.
Organizations own their channel names, timing and external rollout tools; no
fleet controller is required and all four stages need not be mandatory at this
project's scale.

**Acceptance:** A reviewed design traces one digest through promotion without
rebuild, verifies its evidence on each transition and records approvals. State
that `:<commit>` is a naming convention, not registry-enforced immutability.
Machines following mutable tags see future content on upgrade; digest-pinned
machines require an intentional reference change. Test reference behavior in the
lifecycle lab and define recovery when cached or already staged releases differ
from the current tag. Publication authority remains external to Core.

**Dependencies:** P0-02, P0-03 mechanism decision, P0-04, P0-05.
**References:** [Milestones 6–7](roadmap.md#milestone-6-compliance-and-supply-chain),
[publish workflow](../.github/workflows/supply-chain.yml),
[upstream reference behavior](https://bootc.dev/bootc/man/bootc-switch.8.html).

<a id="p0-08"></a>

### P0-08 — Make upstream-input changes deliberate and verifiable

**Status: Proposed · Scope: M.** Pin checks and Renovate exist, but coverage and
update responsibilities vary. RHEL cannot be checked anonymously; a version-only
binary bump is incomplete without the matching URL and checksum.

**Action:** Inventory bases, RPM repositories/keys, binary and extension archives,
embedded companion, builder, scanners and actions. Document proposal → human
review → resolve/hash → compose/render diff → build/boot/scan. Audit Renovate
coverage against that inventory; distinguish checks that prove availability from
ones that prove expected content, and credentials-required from missing digests.

**Acceptance:** One worked base re-pin and one version/URL/checksum update have
reviewable evidence and explicit failure messages for unavailable input. Map each
input to owner, trigger and verification; security updates receive explicit triage.
Fedora release detection/re-pin remains deliberate; RHEL checks stay downstream
with appropriate access. No automatic merge, new token scope or dependency on
network during compilation. Do not redeploy automation merely because an old
roadmap entry says it is missing.

**Dependencies:** P0-01; coordinate with P1-02 support policy.
**References:** [Renovate configuration](../renovate.json),
[artifact resolver](../tests/artifacts/resolve.py), [CI](../.github/workflows/ci.yml),
[ADR 0019](adr/0019-a-platform-says-whether-its-base-can-fetch.md),
[ADR 0021](adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md).

<a id="p0-07"></a>

### P0-07 — Review released-image risk throughout its useful life

**Status: Proposed · Scope: M.** Reviewed exceptions already expire and bind
specific content. The weekly supply-chain workflow rebuilds from the checkout;
that does not rescan every previously published digest.

**Action:** Extend the documented operating process to inventory supported release
digests, rescan those exact images, prioritize security base re-pins, validate
vendor backports and assess High findings for reachability. Keep raw findings
alongside dated OpenVEX/exception decisions; explain organizational responsibility
for stricter external acceptance policy and private RHEL evidence.

**Acceptance:** A reference procedure selects an old release by digest, records
scanner/database versions, detects a newly relevant finding, assigns an owner
and routes to revalidation, an expiring justified exception or emergency
replacement/withdrawal. Critical blocks, High is tracked and becomes blocking
when demonstrated reachable, as ADR 0012 states. No silent threshold change;
changing that baseline requires a future ADR. A clean scan is not proof of a
secure image.

**Dependencies:** P0-04 incident roles, P0-05 retention/index; P0-08 update path.
**References:** [ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md),
[exception checker](../tests/supply-chain/check_exceptions.py),
[scheduled workflow](../.github/workflows/supply-chain.yml).

## P1 — Product stability and usability

<a id="p1-01"></a>

### P1-01 — Define the public compatibility contract

**Status: Proposed · Scope: M.** `v1alpha1` has already had intentional breaking
changes; schemas and goldens pin today's behavior, not a compatibility policy.

**Action:** Define compiler/resources/Plan/companion compatibility, CLI output and
exit-status promises, deprecation/migration notices and pre-1.0 breaking-change
review. Require a validated outside use case before proposing more public kinds.

**Acceptance:** A documented policy and fixture matrix distinguish compatible,
rejected and migratable inputs; intentional golden changes receive human review.
Define measurable beta/stable promotion criteria: external use, bounded unresolved
API issues, documented migration and compatibility evidence. No automatic version
promotion and no expansion beyond fourteen kinds without an explicit decision.

**Dependencies:** None for policy; pilot evidence from P2-03 for API graduation.
**References:** [resource reference](resources.md), [goldens](../internal/compiler/golden_test.go),
[architecture tests](../internal/compiler/architecture_test.go).

<a id="p1-02"></a>

### P1-02 — Bound platform support, adoption and retirement

**Status: Proposed · Scope: M.** The canonical matrix separates composition,
build, boot and publication; it does not define a complete maintenance promise.

**Action:** Extend that single authority with maintainers, security maintenance,
hardware evidence, supported upgrade paths, exit criteria and consumer notices.
Evaluate demand, vendor bases, CI cost and ownership before adding a platform;
use Ubuntu's separation as a maintenance-cost precedent.

**Acceptance:** Every current platform and downstream boundary has an explicit
support statement and retirement process, with no LTS promise inferred from vendor
lifecycle. Preserve private RHEL builds, AlmaLinux's experimental vendor base,
Fedora's one-release policy and prerelease caveat, and downstream Ubuntu ownership.
Fedora publication requires the released-compose re-pin, successful relevant gates
and human approval; hardware validation is a separate claim. State how a lost
maintainer/vendor support changes status and how users change update references.

**Dependencies:** P0-01 findings; P0-02 before promising upgrade paths.
**References:** [platform matrix](platforms.md),
[ADR 0014](adr/0014-platform-base-ownership.md), [ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md).

<a id="p1-04"></a>

### P1-04 — Correct current guidance and make recovery discoverable

**Status: In progress · Scope: M.** Platform status already has one home, but
documentation restructuring is unfinished. Stale descriptions include unmodeled
downloads/AppIndicator in architecture, Ubuntu-era VM instructions, outdated
example counts and a release-asset list that omits the companion.

**Action:** Audit claims against current ADRs/code. Add symptom → evidence →
remedy guidance for install, first boot, registry, update and recovery failures;
link from README. Keep the resource reference navigable; split it only when that
improves finding information. Separate historical observations from current claims.

**Acceptance:** One platform status change needs one matrix edit and evidence
links rather than duplicated tables. Document verified versus experimental QCOW2,
ISO, UEFI and partitioning paths, first-user behavior and destructive installer
assumptions; record an installation/recovery walkthrough. Explain `/etc` merge,
shared `/var`, organization-owned baseline and user-owned state without promising
data rollback. Current commands/asset names match the release and ADR 0018;
links and reader navigation are checked. This roadmap update does not itself
complete those corrections.

**Dependencies:** None for corrections; P0-02/P1-03 provide measured walkthroughs.
**References:** [historical open work](roadmap.md#open-work-2026-10-11),
[docs index](README.md), [installer ISO](installer-iso.md), [ADR 0018](adr/0018-the-runtime-companion.md).

<a id="p1-03"></a>

### P1-03 — Complete one public organizational adoption journey

**Status: Proposed · Scope: M.** Examples already prove composition without
copying Core; a newcomer still crosses several documents and an unmet role need.

**Action:** Extend the CentOS organization/role example's guide through obtaining a
version-matched compiler, validate, customize baseline/role and update policy,
inspect canonical Plan/provenance and declared SBOM, render, build, make a UEFI
QCOW2, create a first user, inspect with `deskos`, then update and recover. Reuse
the existing repositories, trust and desktop definitions. Document optional private
registry integration with placeholders and external credentials.

**Acceptance:** An unfamiliar engineer follows one pinned reference exercise
without founder-only infrastructure or undocumented steps, recording versions,
digests and host/disk/RAM prerequisites. Explain public-key trust versus image
authenticity and local lab images versus reachable update references. Keep the
`virt-manager` gap visible until the example organization explicitly decides;
do not silently substitute Cockpit. Produce a sample provenance explanation, not
another copy of Core resources. Record gaps discovered by the exercise.

**Dependencies:** P1-01/P1-02 policy drafts, P1-04 corrections; P0-02 for a verified
recovery exercise. **References:** [example](../examples/organization-and-role/README.md),
[development](development.md), [installation](install.md), [operations](operations.md).

<a id="p1-05"></a>

### P1-05 — Make existing diagnostics useful to outside authors

**Status: Proposed · Scope: M.** Provenance and explicit capability errors exist;
their usefulness to new users and external tooling has not been established.

**Action:** Walk representative failures through `validate`, `plan`, `render`,
`sbom`, `deskos report/check/update`. Improve resource/file/layer context and next
actions where confusing; investigate machine-readable diagnostics or stable codes
only against concrete consumers. Distinguish artifact intent, observed state,
unsupported capability and an observation that could not be made.

**Acceptance:** Missing/unknown resources, same-layer conflicts, invalid profile
references, unavailable packages, missing download capabilities and absent runtime
evidence each have a documented, actionable diagnostic. Positive and negative
examples keep tested scope honest. Do not add commands merely to rename what
`bootc` or existing CLI output already explains.

**Dependencies:** P1-01 contract; P1-03/P2-03 user findings prioritize changes.
**References:** [text Plan](../internal/textplan), [composition tests](../internal/compiler/compose_test.go),
[companion tests](../cmd/deskos/main_test.go).

<a id="p1-06"></a>

### P1-06 — Test the workstation behavior vendor updates can break

**Status: Proposed · Scope: L.** Session defaults, enabled extensions and headless
Firefox already have probes. They do not prove interactive login, app launches,
Flatpak materialization or a complete desktop workflow.

**Action:** Expand the existing harness incrementally: declared favorite IDs and
launches; local harness-served browser content; GDM login/Wayland, logo, wallpaper
and first-boot behavior; extension loads and StatusNotifierWatcher; offline login,
Flatpak install after reconnection and screen locking. Select accessibility probes
where useful. Monitor vendor GNOME changes, not a DeskOS GNOME fork.

**Acceptance:** A recorded behavior matrix ties each promoted image's claims to
positive/negative probes, distinguishes instrumentation from an unmodified boot
and lists untestable behavior. Test AlmaLinux Flatpak/trust only before claiming
them exercised. Include an extension-incompatible vendor update failure. Assign
power/suspend/display behavior to hardware testing when emulation cannot prove it.

**Dependencies:** P0-01, P1-02; P0-06 later consumes results.
**References:** [VM harness](../tests/vm/README.md),
[ADR 0016](adr/0016-vendor-desktop-and-app-menu.md),
[ADR 0021](adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md).

<a id="p1-07"></a>

### P1-07 — Record a small physical-hardware validation matrix

**Status: Proposed · Scope: M.** VM evidence does not establish workstation
hardware support, and no universal compatibility promise is affordable.

**Action:** With volunteered hardware and an owner, select a generic UEFI laptop
and desktop spanning available Intel/AMD graphics. Record Wi-Fi/Bluetooth,
suspend/resume, external displays, audio/camera and encryption where actually
configured. Treat NVIDIA as a separately justified investigation.

**Acceptance:** Repeatable results record device class/model, firmware, image
digest, installer path and limitations without personal identifiers. Retest on a
meaningful kernel/vendor transition; label untested combinations explicitly.
No extra hardware support tier without a maintainer and retest capacity.

**Dependencies:** P0-02, P1-02/P1-03; donated hardware and operator availability.
**References:** [validation evidence](architecture.md#validation-status), [platforms](platforms.md).

<a id="p1-08"></a>

### P1-08 — Make update behavior predictable to users and operators

**Status: Proposed · Scope: M.** Staging and explicit companion updates exist;
battery-skipped runs wait for the next schedule, and metered handling is not
supported by ADR 0007. GUI notification expectations remain undefined.

**Action:** Document and test staged visibility, ordinary versus explicit reboot,
timer/companion concurrency, failures, offline/reconnected devices and system
Flatpak independence. Assess AC-connect catch-up and notifications using upstream
facilities only after a user need is demonstrated.

**Acceptance:** A scenario table covers automatic disabled/enabled, battery skip,
AC return, metered/offline and failed download, showing actual behavior and logs.
State what `deskos update` invokes and that it does not update Flatpaks or firmware
today. No unsolicited reboot or reconciliation. Any metered support or change to
staging semantics needs human review of ADR 0007 before implementation; proposal
and current behavior remain distinguishable.

**Dependencies:** P0-02, P0-03; P1-07 for power behavior that requires hardware.
**References:** [operations](operations.md), [ADR 0007](adr/0007-updatepolicy.md),
[update guard](../internal/evidence/guard.go).

## P2 — Community and enterprise-readiness

### Community and sustainable maintenance

<a id="p2-01"></a>

### P2-01 — Publish contribution, security, support and conduct entry points

**Status: Proposed · Scope: S.** Engineering rules exist, but newcomers lack a
public contribution path and confidential security-reporting instructions.

**Action:** Prepare `CONTRIBUTING.md`, `SECURITY.md`, and concise support/conduct
guidance with a maintainer-confirmed private reporting route. Cover scoped
proposals, tests (including separate Python harness suites), golden review, ADR
discussion and human-owned PRs. Link existing agent rules rather than inventing
another AI policy or execution framework.

**Acceptance:** An outside contributor can reproduce checks and submit a bounded
patch without private instructions. Users can distinguish security reporting,
support, bugs and collaboration concerns. State realistic supported versions and
response expectations, with no unstaffed response-time promise. Conduct rules
focus on respectful technical collaboration and safe participation. AI-assisted
output must be verified; human review/merge/release accountability and deterministic
model-free compilation remain authoritative. Confirm routes before publishing
them; this proposal does not enable remote features.

**Dependencies:** Maintainer chooses reporting/support contacts; no engineering
gate dependency. **References:** [AGENTS.md](../AGENTS.md),
[development](development.md), [ADR 0011](adr/0011-agentic-contribution-layer.md).

<a id="p2-02"></a>

### P2-02 — Define lightweight governance and maintainer continuity

**Status: Proposed · Scope: M.** Founder ownership and informal procedures are
fragile even if the compiler is small.

**Action:** Define reviewer/maintainer responsibilities, appointments, decision
and disagreement paths, RFC/ADR triggers, release approval and transfer of
maintainership. Map ownership of compiler, platforms, tests, docs and supply chain;
write onboarding and continuity runbooks, including backup access custodians.

**Acceptance:** A tabletop absence scenario identifies who can review, stop a
release, communicate and recover project/evidence infrastructure. Record gaps
honestly if a second maintainer is unavailable. Backups and access recovery have
a verification plan; never place credentials in documentation. Delegate by
demonstrated competence, not a contributor-count target, with separate explicit
approval for privileges. No committees needed.

**Dependencies:** P2-01; coordinate incident authority with P0-04.
**References:** [human approval rules](../AGENTS.md), [release workflow](../.github/workflows/release.yml).

<a id="p2-03"></a>

### P2-03 — Validate independent adoption and the compiler's value

**Status: Proposed · Scope: M.** Published reference images do not establish
that organizations can maintain their own workstation definitions economically.

**Action:** Invite two or three willing teams to author and maintain separate
resource roots. Compare a bounded baseline change in independent Containerfiles
and CI against DeskOS profiles: duplication, change effort, conflict detection,
provenance, vendor-base portability and maintenance cost. Collect user-owned,
explicitly opt-in reports; accept unfavorable results.

**Acceptance:** At least two external environments build their own definitions
and document onboarding, update/recovery and maintenance findings. Publish a
reproducible comparison method and results with permission; distinguish anonymous
summaries from approved public attribution. Convert observed obstacles into a
small ranked follow-up list before designing new abstractions. No telemetry.

**Dependencies:** P1-01/P1-02 draft contracts, P1-03, P2-01; P0 safety results before
suggesting production use. **References:** [organization example](../examples/organization-and-role/README.md),
[ADR 0005](adr/0005-managed-baseline-boundary.md).

<a id="p2-04"></a>

### P2-04 — Make releases, proposals and upstream findings easy to follow

**Status: Proposed · Scope: S.** Durable ADRs exist, but release notes are embedded
in a workflow and support/proposal destinations are not a public process.

**Action:** Define concise release/migration notes, platform retirement notices
and development summaries. Choose one proposal/support venue; evaluate Discussions
only if moderation capacity exists. Keep decisions in ADRs and roadmap documents.
Prepare concrete upstream discussion topics from measured work.

**Acceptance:** Users can identify affected installations, reference/identity
changes and required actions for a release. Proposed versus committed work and
abandoned experiments are explicit. Identify appropriate upstream venues for
bootc/install diagnostics, CentOS Alternative Images, AlmaLinux Atomic, Fedora
Image Mode/ELN and GNOME/Flatpak findings, using initramfs, first-boot or extension
evidence rather than announcing hypothetical integrations. Outreach or remote
feature changes require a separate human instruction.

**Dependencies:** P2-01/P2-02, P1-01/P1-02 for compatibility notices.
**References:** [release notes in workflow](../.github/workflows/release.yml),
[research archive](research-notes.md), [ADR index](adr/README.md).

<a id="p2-05"></a>

### P2-05 — Prepare the CentOS Connect 2027 proposal for human submission

**Status: Proposed · Scope: S.** A focused CentOS demonstration could attract
independent users and upstream review without expanding the product.

**Action:** Draft “Building Managed Linux Workstations with bootc: From
Organizational Intent to Auditable Images.” Prepare the problem, typed Plan and
composition explanation, a CentOS Stream 10 organization/role demo, evidence,
lessons, limitations and collaboration requests. Prefer a 25-minute outline.

**Acceptance:** A proposal, speaker-approved abstract and reproducible demo outline
are ready for a maintainer to submit. Capture a known-good digest and fallback recording;
do not present lifecycle E2E, consumer enforcement or Fedora repair as complete
without results. Submission is a separate explicitly approved action.

**Timing, verified 2026-10-20:** The [official CFP](https://cfp.fedoraproject.org/centos-connect-2027/cfp)
closes **2026-12-04 23:59 Europe/Brussels** (22:59 UTC). The
[event](https://www.centos.org/events/connect/) is **January 28–29, 2027, Brussels**;
the organizers prefer 25-minute talks with some 40-minute slots. Recheck before
submission. This is the only external calendar constraint in this backlog.

**Dependencies:** Existing CentOS example and recorded evidence suffice for a
proposal; no dependency on finishing P0. **References:** [example](../examples/organization-and-role/README.md),
[validation evidence](architecture.md#validation-status).

### Enterprise work gated by actual organizational demand

<a id="p2-06"></a>

### P2-06 — Validate external enrollment and restricted-network operation

**Status: Research · Scope: M.** Identity and disconnected-network requirements
are plausible adoption blockers, but no reference organizational lab is established.

**Action:** With a requesting pilot, scope one FreeIPA or AD/SSSD/Kerberos lab:
client packages and enterprise CA trust in the artifact; per-machine enrollment,
emergency local access and credentials in existing external provisioning tools.
Assess private/mirrored registries and package/Flatpak sources only to the extent
that the same lab needs them.

**Acceptance:** A design and isolated lab plan separate artifact, provisioning and
enrollment ownership. Cover first boot offline, credential expiry, identity-server
outage and emergency access. For restricted networks, list every network input,
how images/evidence/signatures are mirrored and verified, and how update references
and trust are provisioned externally. No embedded join secret, home management,
identity service or fleet controller; demonstrate one lab before generalizing.

**Dependencies:** A pilot sponsor from P2-03, P0-03/P0-05 trust and evidence contracts.
**References:** [Milestone 5](roadmap.md#milestone-5-identity-and-provisioning),
[provisioning boundary](architecture.md#provisioning-and-enrollment).

<a id="p2-07"></a>

### P2-07 — Map compliance requirements to established evidence

**Status: Research · Scope: M.** SBOMs and scans are useful evidence, not a
compliance certification. No requesting organization's control set is selected.

**Action:** Select one real requirement and map artifact properties to OpenSCAP/
SCAP Security Guide, CycloneDX/SPDX, OpenVEX and provenance evidence where useful.
Distinguish build-time checks from installed-machine checks owned by the
organization. Reuse standards; keep scanners external to compiler semantics.

**Acceptance:** A scoped reference workflow names control, producer, subject
digest, limitations and verification steps, including missing evidence. Do not
implement historical candidate `TrustPolicy`/`ComplianceProfile` kinds without
proof that external policy is insufficient and a future approved ADR.

**Dependencies:** Requesting organization, P0-05, P2-06 when identity controls matter.
**References:** [ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md), [supply chain](supply-chain.md).

<a id="p2-08"></a>

### P2-08 — Define portable factory outputs and evaluate policy independently

**Status: Research · Scope: M.** Compiler outputs are CI-neutral; the complete
image/evidence handoff is currently expressed by the GitHub workflows.

**Action:** Specify the minimum contract: versioned compiler/resources, canonical
Plan, rendered context/manifest, OCI subject digest, installed SBOM, provenance,
test results and authorization. Run a small Conforma (`ec`) evaluation against
GitHub-generated evidence before considering another build platform.

**Acceptance:** An independent verifier can check exact identity/digest, provenance,
SBOM association and required tests, with negative cases for missing or mismatched
evidence. Record which predicates Conforma can consume and what would need an
adapter; no claim of compatibility before the probe. The contract remains usable
by Tekton, Konflux, GitLab or internal CI without Kubernetes in Core. Demonstrate
one alternative runner only after a user justifies its maintenance cost.

**Dependencies:** P0-05/P0-06 contracts; independent policy experiment can inform
promotion design. **References:** [Conforma CLI](https://conforma.dev/docs/cli/),
[Conforma policies](https://conforma.dev/docs/policy/index.html),
[compiler outputs](architecture.md#containerfile-backend).

## P3 — Strategic research, without feature commitments

Each investigation needs a sponsor, bounded question and maintenance comparison
before it starts. These entries are not parallel work or a reason to delay P0/P1.

<a id="p3-01"></a>

### P3-01 — Compare Konflux with the existing factory at actual project scale

**Status: Research · Scope: M.** Alternative build infrastructure may offer stronger
boundaries, but migration itself creates operational cost.

**Action:** Compare the same rendered context/evidence contract on GitHub and
Konflux: provenance isolation, policy/promotion, performance/cost, developer
experience, QEMU/KVM, privileged workloads, OpenShift SCC and maintenance staffing.

**Acceptance:** A written keep/adopt/reject decision with measured advantages and
costs; any probe is external infrastructure. Adoption by Fedora/Red Hat alone is
insufficient. No Core migration or permanent service without a human decision.

**Dependencies:** P2-08 and a requesting user/maintainer.
**References:** [CI adapter boundary](architecture.md#design-principles),
[upstream Konflux documentation](https://konflux-ci.dev/docs/).

<a id="p3-02"></a>

### P3-02 — Watch upstream bases and GNOME changes against concrete risks

**Status: Research · Scope: S.** New bases and desktop releases may reduce gaps
or add costs; names and release plans are not usable platform evidence.

**Action:** Maintain a bounded watchlist for ELN/Stream 11/RHEL 11, official bootc
bases, Hummingbird and GNOME OS. Verify what each project actually is before
comparing ownership, image model, package/security lifecycle, bootc compatibility,
CI and staffing. Track vendor GNOME/Wayland, portals, accessibility, remote desktop,
HDR, authentication and extension changes only where a DeskOS behavior is at risk.

**Acceptance:** A source-linked prerequisite checklist and an explicit adoption
threshold: usable base, user need, owner and executable validation path. Feed
specific risks into P1-06; do not create Stream 11 resources, promise GNOME 50/51
backports or maintain a desktop fork. “No action” is a valid result. These names
are research questions, not claims of current suitability or future release dates.

**Dependencies:** P1-02 support/cost criteria; evaluate when relevant upstream
evidence changes. **References:** [official-base rule](adr/0004-core-is-platform-independent.md),
[owned-base rule](adr/0014-platform-base-ownership.md), [platforms](platforms.md).

<a id="p3-03"></a>

### P3-03 — Establish a secure-installation responsibility model

**Status: Research · Scope: M.** Disk encryption and boot-chain trust are
organizational requirements distinct from OCI publisher authenticity.

**Action:** For a real threat model, map UEFI Secure Boot, TPM2, LUKS recovery,
measured boot, installer verification, factory provisioning and replacement to
firmware, vendor bootc/installer, hardware and the organization. Prefer upstream
support over maintaining a signing/install stack.

**Acceptance:** A design and lab plan cover trust bootstrap, lost keys, failed
install, encrypted-device recovery and replacement; state unsupported combinations.
No implementation or key handling follows from this research entry.

**Dependencies:** P0-02/P0-03, requesting pilot and physical hardware owner.
**References:** [installer guide](installer-iso.md),
[provisioning boundary](architecture.md#provisioning-and-enrollment).

<a id="p3-04"></a>

### P3-04 — Assess aarch64 only with users and test ownership

**Status: Research · Scope: M.** Compiler portability alone does not establish
desktop image support on another architecture.

**Action:** Inventory compiler/companion assets, vendor bootc bases, packages,
binary/extension inputs, runners, disk builders and ARM workstation hardware.

**Acceptance:** Feasibility and maintenance assessment names requesting users,
maintainer and executable build/boot/desktop matrix; choose no expansion if any
prerequisite is absent. Distinguish CLI releases from bootable platform support.

**Dependencies:** P1-02, pilot demand, funded or available validation hardware.
**References:** [release architecture](../.github/workflows/release.yml), [platform resources](../resources/platforms).

<a id="p3-05"></a>

### P3-05 — Measure image reproducibility gaps before choosing lockfiles

**Status: Research · Scope: M.** Plans and contexts are deterministic; repository
resolution, build tools and metadata still prevent an image reproducibility claim.

**Action:** Compare two controlled builds; inventory repository snapshots/exact
packages, bases, keys, binaries, Flatpak runtime materialization, tools, timestamps
and environment differences. Separate image bytes from post-boot mutable apps.
Measure build time, image/disk size, boot and upgrade resource costs as diagnostic
baselines, not unsupported performance targets.

**Acceptance:** A gap analysis identifies which differences matter to audit or
recovery, and minimal controls with retention/maintenance costs. Propose resolved
input manifests or lockfiles only for demonstrated gaps; any language change
requires a future decision. Continue to say deterministic compilation/rendering
until repeat image digests are actually demonstrated under stated conditions.

**Dependencies:** P0-05 evidence and P0-08 input inventory; repeatable test host.
**References:** [supply-chain inputs](architecture.md#supply-chain-inputs),
[historical lockfile proposal](roadmap.md#later).

<a id="p3-06"></a>

### P3-06 — Evaluate external agent assistance and focused UX improvements

**Status: Research · Scope: S.** Better explanations and onboarding could help
administrators, including those using agents, without expanding compiler semantics.

**Action:** Use pilot findings to select a small explanation/UX problem: Plan diff,
why a value exists, validated proposed YAML, migration review or update visibility.
Study relevant Bluefin/Omarchy lessons in discoverability, coherent defaults and
accessibility only when they address that problem. Prefer existing CLI/JSON and
documentation over a custom agent platform or more themes.

**Acceptance:** A proposal ties a measurable user obstacle to a bounded improvement
and compares manual/agent-assisted use. Any generated resources pass the ordinary
offline validator and human review. No model in a provider/Plan/backend, proprietary
API requirement, autonomous production change, home management or telemetry.

**Dependencies:** P1-05, P2-03 findings; do not start from a model integration idea.
**References:** [ADR 0011](adr/0011-agentic-contribution-layer.md),
[managed baseline](adr/0005-managed-baseline-boundary.md).

<a id="deferred-decisions"></a>

## Deferred decisions and non-goals

- **Not planned:** general-purpose fleet management, an Ansible/Satellite/MDM
  replacement, endpoint reconciliation or a DeskOS identity service (ADR 0001).
- **Not planned:** shell hooks/raw dconf escape hatches, YAML-to-Containerfile
  templating, personal development environments or home-directory management
  (ADRs 0002/0005).
- **Not planned:** maintaining GNOME/Mutter/distribution package forks for newer
  desktop versions, unlimited editions or a desktop-customization race. Use vendor
  components and upstream fixes; the existing pinned extension delivery is bounded.
- **Not planned:** AI-dependent compilation, a proprietary model requirement,
  custom agent orchestration in Core or autonomous release authority (ADR 0011).
- **Not planned:** permanent dependency on Konflux or any CI provider; research
  must demonstrate a benefit before infrastructure changes.
- **Not planned:** unsupported LTS guarantees, automatic platform expansion,
  public redistribution of RHEL derivatives or bringing Ubuntu builds back into
  Core (ADRs 0004/0014/0020).
- **Deferred, no commitment:** `TrustPolicy`, `ComplianceProfile`, `TimeSyncPolicy`,
  `PerformancePolicy`, `FirmwarePolicy`, `IdentityProvider`, `WebApplication`,
  external providers and a custom update orchestrator. Existing milestones named
  candidates; they are not authority to add kinds. External need, a minimal proven
  abstraction and an approved future ADR must come first.
- **Deferred, lower impact:** third-party theme import, session desktop-logo and
  Activities-button recoloring. Existing Theme/applier functionality stays recorded
  in milestone history; finish lifecycle, trust and user adoption before new polish.
- **Deferred until justified:** prebuilt disk/ISO hosting, registry mirrors, a
  second simultaneous Fedora release and hardware-specific/NVIDIA support. Each
  creates a retention, trust or ongoing validation obligation; P1-02/P2-03 must
  supply the case. Restricted-network organizational mirroring is scoped in P2-06.

## Decisions reserved for human review

Choosing roadmap order is not authorization to implement security-sensitive work.
Humans must choose consumer trust/rotation and emergency recovery (P0-03/P0-04),
evidence retention and signing of release assets (P0-05), publication/channel
authority (P0-06), API promotion/deprecation (P1-01), support promises/retirement
(P1-02), reporting contacts and maintainer delegation (P2-01/P2-02).

Any vulnerability threshold change conflicts with ADR 0012 until a future ADR
accepts it. New kinds, metered-update behavior beyond ADR 0007, changes to the
artifact/runtime boundary or a new platform ownership model likewise need human
architectural review first. No such decision is made here. Security policy,
enrollment and enterprise verification should be proven externally before asking
the compiler to express another concept.
