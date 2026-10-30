# ADR 0028: the installed package set is a manifest the build verifies

**Status:** accepted (2026-10-30)

## Context

A platform installs its desktop from distribution groups:
`workstation-product-environment` on EL10. A group is the distribution's list, in
`comps.xml`, and it changes between minor releases without asking anyone. Measured
on the pinned CentOS Stream 10 base (2026-10-30): the group adds **845 packages**;
the eleven groups have **297 direct members** and the rest are dependencies. What
arrives beyond a session includes 18 desktop applications, 16 input-method packages
and 14 tools a workstation rarely needs.

The choices are a hand-kept package list (297 names, per platform, and a comps
change that adds a required dependency would be missed), the group with exclusions
(what exists, blind to additions), or something that keeps the group but makes a
change visible.

## Decision

**The platform declares a manifest of installed package names, and the build fails
when the installed set differs.**

- `Platform.spec.packageManifest` names a file beside the resource, one package
  name per line, comments and blank lines ignored, sorted and unique after parsing.
- The Plan carries the list. The backend writes it into the build context, copies it
  to `/tmp/deskos-manifest/packages.txt`, and after the **group transaction** (and
  before the explicit packages) compares it with `rpm -qa`:

  ```
  diff -u /tmp/deskos-manifest/packages.txt /tmp/deskos-actual.txt
  ```

  A difference fails the build with the added and removed names.
- The check sits between the two transactions on purpose: the groups are the
  distribution's list, the explicit packages are the profiles' and are reviewed in
  the goldens. The manifest covers the base image plus the groups, which is exactly
  what a comps change moves.
- `excludePackages` keeps working: it is passed to the group transaction, so the
  excluded packages never enter the set the manifest describes.

## Why not an explicit package list

The prior art settles it. Bluefin LTS installs the GNOME group with `exclude=` lines
and explicit upgrades; Bluefin Classic inherits Silverblue, whose package set is
maintained by Fedora. Nobody keeps 845 names, and a frozen list would miss a comps
change that adds a *required* dependency, which is the failure a session check
catches late and expensively.

A manifest gives the property without the list: nothing arrives unannounced, and the
review is the diff.

## Consequences

- A comps or base-image change fails the build until a person regenerates the
  manifest. Regenerating is a deliberate act: run the same group install on the new
  base digest and commit the new file, with the diff in the pull request.
- The manifest is per platform and per base digest. CentOS Stream 10 has one now;
  RHEL 10's comes from the organization's entitled build, because this repository
  cannot resolve RHEL repositories; AlmaLinux and Fedora follow when their platforms
  become first-class.
- The manifest travels in the Plan and the SBOM, so `deskos report` can say what the
  image was expected to carry, not only what it carries.
- A platform that declares no manifest gets no check, and that is visible in the
  resource rather than implied.
