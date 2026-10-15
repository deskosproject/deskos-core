# ADR 0015: A second package manager, and where the package database lives

**Status:** accepted (2026-10-15)

## Context

ADR 0014 allows a platform whose vendor publishes no bootc base to be added as an
organization-owned base. Ubuntu is the case that motivated it, and Ubuntu is not
RPM: `PackageSet` means RPM packages and Platform-mapped RPM groups, and
`RpmRepository` and `rpmFiles` are intrinsically RPM.

The 2026-10-15 spike measured what that costs, and produced two facts that decide
the design:

- **A derived image can install, but blind.** The Ubuntu base follows upstream in
  running `rm -rf /var`, so its dpkg database is gone: `dpkg -l` lists nothing.
  `apt-get install` still works, but it treats the whole dependency closure as
  missing. Installing `curl` would install 35 packages of which **34 are already
  on disk**. Nothing is corrupted by that, but every derived image re-downloads
  and re-installs its own system, and conffile and postinst behaviour stops being
  predictable.
- **The bootc convention keeps the database in the image, not in `/var`.** A
  CentOS Stream 10 bootc image has no `/var/lib/rpm`, an almost empty `/var`, and
  its database at `/usr/lib/sysimage/rpm`, where `rpm -qa` reports **488
  packages**. `/usr` is image content, so a derived image inherits it and installs
  incrementally. That is what makes the "`FROM` the base, then add packages" model
  work at all.

## Decision

1. **A platform declares its package manager.** The `Platform` resource records
   which manager the base uses, and the group and availability facts it already
   carries become manager-neutral rather than RPM-shaped. The compiler keeps one
   package model and one composition path; only the rendering differs.

2. **The package database is image content, not state.** A base that a derived
   image installs into must ship a complete, usable package database inside the
   image, the way the RPM family puts it in `/usr/lib/sysimage/rpm`. `/var` stays
   free of state, as the bootc convention requires. For Ubuntu this means
   relocating dpkg's administration directory into the image; `deskos-ubuntu-base`
   is the repository that must do it, and a derived image must be able to install
   a package without re-installing what is already there.

3. **APT repositories become their own kind.** `RpmRepository` stays exactly what
   it is, and a new public kind expresses a signed APT source: suite, components,
   architectures, and the vendor key. A repository of one package manager is not a
   repository of another, and renaming the existing kind would break every
   existing resource for no gain. The public API therefore moves from thirteen
   kinds (ADR 0013) to **fourteen**, and the count test moves with it.

4. **The Containerfile backend gains an APT path next to the dnf one.** It renders
   signed APT sources and keys, runs a checked package transaction, and keeps the
   same guarantees the RPM path has: caches and credentials out of the image, no
   RHSM code on a platform that has no RHSM, and the same deterministic output for
   the same inputs. Compilation stays offline and model-free (invariants 2 and 11);
   the build, not the compiler, needs the network.

5. **Capabilities stay explicit.** A platform declares what it can do, and a
   request it cannot satisfy fails with a reason (invariant 9) instead of being
   silently substituted. The measured case: Ubuntu 26.04 ships Flatpak 1.16.6,
   which has no `flatpak preinstall`, so a system `FlatpakSet` cannot materialize
   there without a newer source, and the platform must say so rather than pretend.

## Consequences

- A second package manager is expressible without weakening the first: RPM
  platforms keep their kind, their groups and their rendering.
- The public API has fourteen kinds, and the count test in
  `internal/compiler/architecture_test.go` must say so.
- `deskos-ubuntu-base` gains a requirement it does not meet yet: a usable package
  database inside the image. Until it does, an Ubuntu derived image is technically
  buildable but blind, and this ADR says that is not good enough.
- Package-level determinism keeps the shape it has today: inputs are pinned and
  the plan records what was requested, but a package transaction is resolved by the
  distribution's archive, exactly as the RPM path is resolved by its repositories.
- The browser, the firewall and the application channel remain platform or
  organization choices (ADR 0014), not foundation ones.
