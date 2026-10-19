# ADR 0014: Platform base ownership and support tiers

**Status:** accepted (2026-10-15)

## Context

ADR 0004 requires each workstation to compile Core onto its platform's
**official** bootc base, so that a RHEL workstation is RHEL and not a
derivative that cannot be supported as RHEL. That rule has no answer for a
platform whose vendor publishes **no** bootc base.

Ubuntu is that case today. Canonical publishes `ubuntu` OCI images, which are
container root filesystems, not bootc operating systems; no Canonical Ubuntu
bootc base was found in the official image sources checked. The only available
bases are community recipes, and the one exercised in the 2026-10-15 spike was
broken in ways that matter:

- its build failed at HEAD because a floating dependency (`selinux-sys` via
  bindgen) needed `libclang`, which the builder stage did not install, while
  `shared/build.sh` cloned `bootc` from HEAD with no pin;
- its `rm -rf /var` destroyed the directory skeleton that packages install
  there (`snapd`'s `void` at mode `0111`, `apparmor/snap-confine`,
  `ubuntu-advantage`), which broke `apparmor.service` on first boot and made
  `snap run firefox` fail;
- the archive's Flatpak is 1.16.6, which has no `flatpak preinstall`, so a
  `FlatpakSet` could not materialize without a newer source.

Read strictly, ADR 0004's answer is "Ubuntu is not a supported platform". That
is a defensible answer, but it is not the only one: an organization may be
willing to own the base itself, and the spike showed that such a base can be
built, installed, booted, updated and rolled back, with the platform's own
experience (Ubuntu session, snap Firefox) working. This ADR decides how that
may happen without weakening the rule that protects RHEL.

## Decision

1. **The official-base rule stands wherever an official base exists.** A RHEL
   workstation still starts from `registry.redhat.io/rhel10/rhel-bootc`, and
   RHEL-derived images are still never publicly published or redistributed.

2. **A platform with no vendor base may be added only as an
   organization-owned base**, and only with all of the following:
   - **A named owner**: a person or team accountable for the base, its fixes
     and its updates, recorded in the `Platform` resource. An unnamed owner is
     not a tier; it is an omission.
   - **Recorded provenance**: the upstream recipe and its pinned commit, its
     license, and every local change with the measured reason for it.
   - **A declared support tier**, for example `vendor` or `organization`. A
     tier of `organization` must be visible wherever the platform is described
     and must never be presented as vendor support.
   - **Bounded validation claims**: what was verified, how, and what was not
     (invariant 10). "Builds and boots" is not "supported"; the spike's own
     record states which claims were measured and which were not.
   - **A maintenance policy**: who moves the pins (base digest, `bootc`
     commit, package set), how quickly, and who responds when an update breaks
     a workstation.

3. **The base lives in its own repository.** Its recipe, pins and provenance
   are reviewed there; this repository consumes a base reference and the tier
   its owner declared. DeskOS Core does not grow a copy of another project's
   build.

4. **This changes no compiler behaviour.** Adding a platform is still adding a
   `Platform` resource (ADR 0004), and this ADR introduces no kind: the
   package-kind question (`RpmRepository` is intrinsically RPM) remains a
   separate decision.

## Consequences

- Ubuntu becomes expressible as a platform without pretending that Canonical
  supports the result, and the cost lands where it belongs: an owned base is an
  owned liability, and the `Platform` resource now names who carries it.
- A base whose owner is unnamed cannot be added. "We will figure it out later"
  is not a support tier.
- The rule that protects RHEL is untouched, because it is stated as the general
  case rather than as a RHEL-specific exception.
- **Out of scope here:** which software and look choices belong to a platform or
  to an organization. Core's foundation no longer chooses them:
  `resources/desktop/deskos-core.yaml` sets no `dock.enabled`, no wallpaper, no
  login logo and no favorites — the look is selected separately by the
  `deskos-look` profile — and `resources/software/deskos-core.yaml` carries the
  package set without the desktop choices. Composition unions sets and offers no
  removal (ADR 0003), so anything Core chose could not be subtracted by an
  organization: that is why those choices left Core, and why the layering this
  ADR assumes is now meaningful.
