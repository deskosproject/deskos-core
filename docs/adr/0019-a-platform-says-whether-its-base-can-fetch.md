# ADR 0019: A platform says whether its base can fetch

**Status:** accepted (2026-10-18)

## Context

A `BinaryArtifact` is a file DeskOS downloads at build time and verifies
against its declared `sha256` before it lands in the image. `RpmFile` is the
same shape for an RPM the image takes from a URL. Both are fetched by the
containerfile backend, which uses `curl` — a program the backend names, and
which is therefore an assumption about a base rather than a fact about it.

That assumption was wrong once, in the quiet way this kind of thing goes
wrong. Ubuntu's base ships no `curl`: the EL and Fedora bases carry it because
their package manager needs it, and Ubuntu's does not. The build did not say
so. It fetched and died with `/bin/sh: 1: curl: not found`, which is a true
statement about a base and a useless one about a plan: it arrives minutes into
a build, names no resource, and reads like a defect in the artifact instead of
a gap in the platform.

The gap was easy to close in the one place it existed (the Ubuntu platform
lists `curl` among the packages its workstation group installs), but closing
it that way leaves the assumption in place. The next base without a fetcher
fails the same way, and the platform that could have declared the gap has no
field in which to declare it. Invariant 9 asks for the opposite: a requirement
the platform cannot meet fails composition, with a reason, instead of being
substituted or discovered late.

Two things make this a platform fact rather than a backend detail:

- **Which bases carry a fetcher is a property of the base.** It varies by
  vendor and by whether that vendor's package manager depends on one. It is
  measured on a base, one line in a resource, exactly as `flatpak.preinstall`
  and `gnome.keepsVendorDefaults` are.
- **The package that provides one is the platform's name, not Core's.** Core
  knows neither `curl` nor `ca-certificates` nor `wget`, and must not: ADR 0004
  keeps platform names in the platform. A field that names the package is the
  same shape as `iconCache.package` and `flatpak.package`.

## Decision

1. A platform declares how its base gets a verified artifact onto the image:

   ```yaml
   downloads:
     baseFetches: false   # this base ships no fetcher
     package: curl        # and the image installs this one
   ```

   `baseFetches: true` is the common case and names no package: the base
   carries it. `baseFetches: false` requires `package`, because a platform that
   cannot fetch and does not install a fetcher is not a platform that can build
   an image containing a download.

2. **Composition fails on a plan that downloads from a platform that says
   nothing.** A plan with a `BinaryArtifact` or an `RpmFile` on a platform with
   no `downloads` fails with the platform named and the resource that asked
   for the download; a plan that downloads nothing composes anywhere.

3. **The composition installs the fetcher when the base cannot provide it.**
   For a `baseFetches: false` platform, the package joins the plan's installed
   packages, carrying the provenance of the download that needed it. The plan
   therefore answers "why is this installed" without the reader consulting a
   platform resource, and the image installs nothing for a plan that downloads
   nothing.

4. **The backend keeps naming its own tool.** This ADR does not make the
   fetch command a platform fact: the backend fetches with `curl` and its
   flags, and what the platform declares is that a base can satisfy it. A
   command line in a resource would be a per-platform copy of backend
   knowledge, and the copies would drift.

5. **A fetch retries transient failures and bounds the attempts that never
   answer.** The download step retries a connection error, a 5xx and a timeout,
   bounded in time; a 404 or a checksum that disagrees still fails at once,
   because neither is transient. The bytes the plan pins are what decides, and
   the retry never softens that.

## Consequences

- Ubuntu declares `baseFetches: false` with `package: curl`, measured on its
  base; the EL and Fedora platforms declare `baseFetches: true`, measured on
  theirs; RHEL's base cannot be pulled here (no subscription), and its
  declaration says so and rests on the EL bases that can.
- A new platform cannot be silent about this. Composing a workstation that
  downloads onto one fails at composition, with the platform named, instead of
  failing on a missing command minutes into a build.
- The plan's package list becomes the record of what the image installs to be
  able to fetch, and `deskos check`'s scope already reports installed packages
  as outside its remit.
- The `curl` in the backend remains Core's own choice, and this ADR is what
  keeps it from being an unchecked assumption: the platform states the base
  can satisfy it, or states the package that makes that true.
- This does not cover what else a base may or may not provide. It covers the
  one capability a build cannot proceed without, and leaves the rest to be
  added the same way, when one of them bites.
