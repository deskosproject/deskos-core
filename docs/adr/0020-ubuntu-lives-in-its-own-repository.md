# ADR 0020: Ubuntu lives in its own repository

**Status:** accepted (2026-10-18)

## Context

ADR 0004 requires a workstation to compile onto its platform **vendor's** official bootc
base. ADR 0014 allows one exception: a platform whose vendor publishes no bootc base may be
added as an organization-owned base, with a named owner and a declared support tier. Ubuntu
26.04 is that exception. Canonical publishes container images, not an installable and
updatable operating system, so the Project DeskOS builds and maintains the base itself, in
`deskosproject/deskos-ubuntu-base`.

Keeping that platform inside `deskos-core` has cost Core more than a platform normally does,
and the cost is the argument for this decision. Every gap in that base has become a change in
Core: a base without `curl` produced the `downloads` platform fact (ADR 0019); a `/usr/local`
that is a symlink into `/var` produced a fix in the Containerfile backend; a base with no
`/etc/dconf/profile/user` produced `gnome.keepsVendorDefaults` and split Core's GNOME defaults
into the `deskos-gnome` profile (ADR 0016); Flatpak 1.16.6, which predates `flatpak
preinstall`, produced a composition failure path; and a base that ships an account produced
the first-boot work. Core's release also carries it: a publish leg with its own disk path
(`bootc install to-disk`, no Anaconda), two virtual machines on every pull request, its
goldens, its resources and a page of documentation.

None of that is wrong. It is the shape of a platform whose base is ours: a base we build is a
base whose surprises are ours to find. But it means the cadence of an experimental,
organization-owned platform is coupled to the cadence of Core, in both directions — Core's
release waits on it, and its users wait on Core.

## Decision

1. **The Ubuntu workstation lives in `deskosproject/deskos-ubuntu`**: its `Platform` resource,
   its workstations, its profiles and assets, its example, its vendored test harness, its
   gates and its publishing. Its base keeps living in a repository of its own, renamed
   `deskosproject/deskos-ubuntu-bootc` so that the two repositories say which is which.
2. **Core keeps the language and the backend.** `deb`, the apt rendering and the
   `AptRepository` kind (ADR 0015) stay: Core can still *express* a Debian-family platform, and
   an organization can still declare one from its own root. What leaves is an implementation,
   not a capability.
3. **Core stops building, booting and publishing Ubuntu.** The two virtual machines, the
   publish leg, the goldens, the example and the platform resource move. `docs/` keeps the
   platform table's row only to say where it went.
4. **Core says only what it still verifies.** It composes and renders a Debian-family
   workstation — that is tested here — and it does not claim to build or boot one. The
   validation matrix states exactly that, and no other page claims more.
5. **The claim lives where the work is.** `deskos-ubuntu` runs its full path — build, boot,
   session check, first-boot check — on its own pull requests *and* when it adopts a Core
   release; and a scheduled run against Core's `main` opens an issue here when Core breaks it.
   Core keeps the APT unit tests and a synthetic Debian golden, so a change in how Core
   renders Debian is a reviewed diff without a machine.
6. **The platform's facts travel with it.** `ubuntu-2604.yaml` declares what its base does and
   does not do, with the measurements, and that resource moves unchanged.

## Consequences

- **Ubuntu's cadence stops gating Core's**, and Core's release stops carrying an
  experimental platform's media. A user of `deskos-ubuntu` gets fixes as that repository
  publishes them, on its own schedule.
- **Core becomes slightly less exercised in one direction.** The apt path keeps its unit
  tests and its synthetic golden; what it loses is a machine that boots it before merge. That
  is a real loss and it is the reason for the scheduled run against `main`: drift is found by
  a person reading an issue, later than a red check, instead of never.
- **Ubuntu's own base is no longer pinned by a Core release.** A new Ubuntu point release, or
  a fix in the base, is a pull request in `deskos-ubuntu`, not a Core release.
- **The published images move to new names**, and this is a consumer-visible change: the
  reference a machine updates from is a deliberate change on that machine, and the old
  repositories keep the digests published in them.
- **The signature identity changes** with the repository: images published by
  `deskos-ubuntu` are signed by its workflow, so anything verifying against
  `deskos-core`'s identity has to be told the new one. Digests already published keep the
  provenance they had.
- **"Experimental" becomes visible in the right place**: the repository that owns the base
  says so on its front page, rather than Core's README carrying a caveat about a platform
  most of its users will never build.
