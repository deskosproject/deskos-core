# CentOS Connect 2027 — proposal draft

**Status: draft, not submitted.** Submission is a separate action the maintainer
takes, after approving the abstract. Nothing here is announced.

- **Event:** CentOS Connect, 28–29 January 2027, Brussels.
- **CFP:** <https://cfp.fedoraproject.org/centos-connect-2027/cfp>, closes
  **2026-12-04 23:59 Europe/Brussels** (22:59 UTC). Verified 2026-10-29; recheck
  before submitting.
- **Format:** the organizers prefer 25 minutes. This outline is 25.

## Title

Building Managed Linux Workstations with bootc: From Organizational Intent to
Auditable Images

## Abstract

Most "managed workstation" stories start with an agent and a fleet. This one
starts with a repository, a typed plan and a container image.

DeskOS compiles declared intent — a platform, an organization baseline, a role —
into a deterministic Plan, a generated Containerfile and one bootc image that is
scanned, booted, signed and published with the evidence of what was checked. There
is no controller and nothing to reconcile: a machine installs an image and later
moves to a newer one. The talk walks a CentOS Stream 10 organization/role example
from resources to a booted UEFI machine, shows what the plan says and why it is
auditable, and ends with what the model does not solve — hardware, enforcement and
lifecycle edge cases included.

## Outline (25 minutes)

| Min | Segment | Content |
|---|---|---|
| 3 | **The problem** | A workstation that is "configured" drifts, and the usual fix is an agent that runs forever. What if the unit of change were the image, and the intent were reviewable before it becomes one? |
| 5 | **The model** | intent → validate → deterministic composition → typed Plan → Containerfile → image → disk. Four semantic layers, sets union, keyed definitions must match, scalars take the highest layer. Compilation is offline and model-free; the same inputs produce the same Plan. |
| 8 | **Demo: the organization example** | `deskosctl validate` and `plan` on `examples/organization-and-role`; the Plan shows the baseline and the role merged, with the provenance of the value that won; `render`; `podman build`; boot the image. One command at a time, with the output on screen. |
| 4 | **The evidence** | The image is scanned, its SBOM is compared against what the build declares, it is signed keyless, and the evidence is keyed by digest. A verifier checks the ISO's checksum, the installer's signature and the digest inside it. Show the two commands, not a slide of badges. |
| 3 | **Lessons and limits** | A platform that cannot meet a requirement fails composition with the reason instead of substituting. What is not solved: bare-metal hardware validation, consumer-side enforcement, the interactive installer, RHEL builds that stay inside an organization. |
| 2 | **Collaboration** | What we would like reviewed upstream, with the evidence attached: a bootc host without `/run/ostree-booted`, a first-boot wizard suppressed by an existing account, and a desktop tool whose only EL10 copy lives in CRB. |

## Demo plan

- A laptop with rootful Podman and KVM, and the CentOS Stream 10 base image pulled
  beforehand.
- A pinned `deskosctl` release (a real tag and its sha256) and the example root.
- A **recorded fallback** of the same demo, captured against a known image digest,
  in case the projector, the network or the registry is not there. The recording
  names the digest it shows.
- No live registry writes, no signing during the talk.

## What this talk does not claim

- No full lifecycle E2E, no consumer-side enforcement, no Fedora repair: those are
  not finished, and the outline says so rather than implying them.
- No "secure" claim: a clean scan is not proof of a secure image.
- No new product: it is a CentOS Stream 10 example, and the RHEL variant stays
  inside the organization that builds it.

## Submission checklist (maintainer)

- [ ] Approve the abstract and the title as spoken.
- [ ] Fill the speaker name and bio in the CFP form.
- [ ] Attach the outline and the demo requirements.
- [ ] Record the fallback and note the digest it shows.
- [ ] Recheck the deadline and the talk length on the CFP page.
- [ ] Submit — and only then is this no longer a draft.

## References for the talk

- [the example walkthrough](../../examples/organization-and-role/WALKTHROUGH.md)
- [the Plan and the pipeline](../architecture.md)
- [what each check proves](../checks.md)
- [what a verifier can check](https://deskos.org/verify/)
