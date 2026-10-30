# ADR 0015: A second package manager, and where the package database lives

**Status:** accepted (2026-10-15)

## Context

Ubuntu is not RPM: `PackageSet` means RPM packages and Platform-mapped RPM groups, and `RpmRepository` and `rpmFiles`
are intrinsically RPM. The 2026-10-15 spike measured two facts: a derived image installs **blind**, because the Ubuntu
base runs `rm -rf /var` and its dpkg database is gone (`dpkg -l` lists nothing), so installing `curl` installs 35
packages of which 34 are already on disk; and the bootc convention keeps the database in the image, because a CentOS
Stream 10 image has no `/var/lib/rpm`, its database is at `/usr/lib/sysimage/rpm`, and `rpm -qa` reports 488 packages.

## Decision

1. **A platform declares its package manager.** The `Platform` resource records which manager the base uses, and its
   group and availability facts become manager-neutral rather than RPM-shaped. The compiler keeps one package model
   and one composition path; only the rendering differs.
2. **The package database is image content, not state.** A base a derived image installs into must ship a complete,
   usable package database inside the image, the way the RPM family puts it in `/usr/lib/sysimage/rpm`, and `/var`
   stays free of state. For Ubuntu that means relocating dpkg's administration directory; `deskos-ubuntu-base` does
   it, and a derived image installs a package without re-installing what is present.
3. **APT repositories become their own kind.** `RpmRepository` stays as it is, and a new public kind describes a
   signed APT source: suite, components, architectures and the vendor key. The public API moves from thirteen kinds
   (ADR 0013) to **fourteen**, and the count test moves with it. A `PackageSet` names the sources it needs, typed for
   the same reason: `repositories` names `RpmRepository` resources and `aptRepositories` names `AptRepository`
   resources, and a set naming the other manager's kind fails at composition with that reason (invariant 9). A
   reference includes a source in the image; it does not pin which repository a package comes from, which stays the
   package manager's own resolution and priority order.
4. **The Containerfile backend gains an APT path next to the dnf one.** It renders signed APT sources and keys, runs a
   checked package transaction, and keeps the RPM path's guarantees: caches and credentials out of the image, no RHSM
   code on a platform without RHSM, and the same deterministic output for the same inputs. Compilation stays offline
   and model-free (invariants 2 and 11); the build, not the compiler, needs the network.
5. **Capabilities stay explicit.** A platform declares what it can do, and a request it cannot satisfy fails with a
   reason (invariant 9) instead of being silently substituted: Ubuntu 26.04 ships Flatpak 1.16.6, which has no
   `flatpak preinstall`, so a system `FlatpakSet` cannot materialize there without a newer source.

## Consequences

- A second package manager is expressible without weakening the first: RPM platforms keep their kind, groups and
  rendering. The count test in `internal/compiler/architecture_test.go` must say the
  new number.
- `deskos-ubuntu-base` keeps dpkg's administration directory in `/usr/lib/sysimage/dpkg` (**met** 2026-10-16: `dpkg
  -l` reports 1509 packages, and a derived `FROM` installing a package reports "already the newest version"), so an
  Ubuntu derived image is buildable and no longer blind.
- Package-level determinism keeps its shape: inputs are pinned and the plan records what was requested, but a package
  transaction is resolved by the distribution's archive, as the RPM path is resolved by its repositories.
- The browser, the firewall and the application channel remain platform or organization choices (ADR 0014).
