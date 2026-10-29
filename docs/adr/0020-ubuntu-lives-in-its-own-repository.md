# ADR 0020: Ubuntu lives in its own repository

**Status:** accepted (2026-10-18)

## Context

ADR 0004 requires a vendor's official bootc base, and ADR 0014 allows one exception: an organization-owned base
with a named owner and a declared support tier. Ubuntu 26.04 is that exception, because Canonical publishes
container images, not an installable and updatable operating system, so Project DeskOS builds and maintains the
base in `deskosproject/deskos-ubuntu-base`. Keeping it in Core cost more than a platform normally does: a base
without `curl` produced the `downloads` fact (ADR 0019), no `/etc/dconf/profile/user` produced
`gnome.keepsVendorDefaults` and the `deskos-gnome` split (ADR 0016), and Flatpak 1.16.6 a failure path.

## Decision

1. **The Ubuntu workstation lives in `deskosproject/deskos-ubuntu`**: its `Platform` resource, workstations,
   profiles and assets, example, vendored test harness, checks and publishing. Its base keeps a repository of its
   own, renamed `deskosproject/deskos-ubuntu-bootc`.
2. **Core keeps the language and the backend.** `deb`, the apt rendering and the `AptRepository` kind (ADR 0015)
   stay: Core can still *express* a Debian-family platform, and an organization can declare one from its own root.
   What leaves is an implementation, not a capability.
3. **Core stops building, booting and publishing Ubuntu.** The VMs, the publish leg, the goldens, the example and
   the platform resource move; `docs/` keeps the platform table's row only to say where it went.
4. **Core says only what it still verifies.** It composes and renders a Debian-family workstation, which is tested
   here, and does not claim to build or boot one; the validation matrix states exactly that.
5. **The claim lives where the work is.** `deskos-ubuntu` runs its full path (build, boot, session check,
   first-boot check) on its own pull requests and when it adopts a Core release, and a scheduled run against Core's
   `main` opens an issue here when Core breaks it. Core keeps the APT unit tests and a synthetic Debian golden.
6. **The platform's facts travel with it.** `ubuntu-2604.yaml` declares what its base does and does not do, with
   the measurements, and moves unchanged.

## Consequences

- **Ubuntu's cadence stops holding back Core's**, and Core's release stops carrying experimental platform media.
- **Core becomes slightly less exercised in one direction**: the apt path keeps its unit tests and synthetic golden
  but loses a machine that boots it before merge, which is why the scheduled run against `main` exists.
- **Ubuntu's base is no longer pinned by a Core release**: a new point release is a pull request in
  `deskos-ubuntu`.
- **The published images move to new names**: the reference a machine updates from is a deliberate change on that
  machine, and old repositories keep the digests published in them.
- **The signature identity changes** with the repository: images published by `deskos-ubuntu` are signed by its
  workflow, so anything verifying against `deskos-core`'s identity has to be told the new one.
- **"Experimental" becomes visible in the right place**: the repository that owns the base says so on its front
  page, rather than Core's README carrying a caveat about a platform most users never build.
