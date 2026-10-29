# ADR 0014: Platform base ownership and support tiers

**Status:** accepted (2026-10-15)

## Context

ADR 0004 requires each workstation to compile Core onto its platform's **official** bootc base, so a RHEL workstation
is RHEL, not a derivative. That rule has no answer for a platform whose vendor publishes **no** bootc base: Canonical
publishes `ubuntu` OCI container images, not bootc operating systems, and the only available bases are community
recipes. The one exercised in the 2026-10-15 spike failed to build at HEAD, destroyed the `/var` skeleton `snapd` and
`apparmor` install into, and shipped Flatpak 1.16.6 without `flatpak preinstall`. Read strictly, ADR 0004 says Ubuntu
is unsupported; an organization may still own the base.

## Decision

1. **The official-base rule stands wherever an official base exists.** A RHEL workstation still starts from
   `registry.redhat.io/rhel10/rhel-bootc`, and RHEL-derived images are still never published or redistributed.
2. **A platform with no vendor base may be added only as an organization-owned base**, and only with all of:
   - **A named owner**: a person or team accountable for the base, its fixes and its updates, recorded in the
     `Platform` resource. An unnamed owner is not a tier; it is an omission.
   - **Recorded provenance**: the upstream recipe and its pinned commit, its license, and every local change with
     the measured reason for it.
   - **A declared support tier**, for example `vendor` or `organization`; a tier of `organization` must be visible
     wherever the platform is described and must never be presented as vendor support.
   - **Bounded validation claims**: what was verified, how, and what was not (invariant 10). "Builds and boots" is
     not "supported".
   - **A maintenance policy**: who moves the pins (base digest, `bootc` commit, package set), how quickly, and who
     responds when an update breaks a workstation.
3. **The base lives in its own repository.** Its recipe, pins and provenance are reviewed there; this repository
   consumes a base reference and the tier its owner declared.
4. **This changes no compiler behaviour.** Adding a platform is still adding a `Platform` resource (ADR 0004), and
   this ADR introduces no kind.

## Consequences

- Ubuntu becomes expressible as a platform without pretending that Canonical supports the result. An owned base is an
  owned liability, and the `Platform` resource names who carries it.
- A base whose owner is unnamed cannot be added. "We will figure it out later" is not a support tier.
- The rule that protects RHEL is untouched, because it is stated as the general case rather than as a RHEL exception.
- **Out of scope here:** which software and look choices belong to a platform or an organization.
  `resources/desktop/deskos-core.yaml` sets no `dock.enabled`, wallpaper, login logo or favorites (the look comes from
  the `deskos-look` profile), and `resources/software/deskos-core.yaml` carries the package set without the desktop
  choices. Composition offers no removal (ADR 0003), so anything Core chose could not be subtracted by an
  organization.
