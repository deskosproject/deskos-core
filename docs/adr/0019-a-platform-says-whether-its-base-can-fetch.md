# ADR 0019: A platform says whether its base can fetch

**Status:** accepted (2026-10-18)

## Context

A `BinaryArtifact` is downloaded at build time and verified against its declared `sha256`, and `RpmFile` is the
same shape for an RPM taken from a URL; the Containerfile backend fetches both with `curl`. That makes `curl` an
assumption about a base, and it was wrong once: the Ubuntu base ships none, so the build died with `/bin/sh: 1:
curl: not found` minutes in, naming no resource. Invariant 9 asks the opposite, and this is a platform fact
because which bases carry a fetcher varies by vendor, and the package is the platform's name, not Core's — Core
knows neither `curl` nor `ca-certificates` (ADR 0004, the shape of `iconCache.package`).

## Decision

1. A platform declares how its base gets a verified artifact onto the image:

   ```yaml
   downloads:
     baseFetches: false   # this base ships no fetcher
     package: curl        # and the image installs this one
   ```

   `baseFetches: true` is the common case and names no package; `baseFetches: false` requires `package`.

2. **Composition fails on a plan that downloads from a platform that says nothing**, naming the platform and the
   resource that asked for the download; a plan that downloads nothing composes anywhere.
3. **The composition installs the fetcher when the base cannot provide it**: the package joins the plan's
   installed packages, carrying the provenance of the download that needed it, so the plan answers "why is this
   installed" without the reader consulting a platform resource.
4. **The backend keeps naming its own tool.** The backend fetches with `curl` and its flags; what the platform
   declares is that a base can satisfy it. A command line in a resource would be a per-platform copy of backend
   knowledge, and the copies would drift.
5. **A fetch retries transient failures and bounds the attempts that never answer**: a connection error, a 5xx
   and a timeout are retried within a bound; a 404 or a checksum that disagrees fails at once.

## Consequences

- Ubuntu declares `baseFetches: false` with `package: curl`, measured on its base; the EL and Fedora platforms
  declare `baseFetches: true`, measured on theirs; RHEL's base cannot be pulled here (no subscription), so its
  declaration rests on the EL bases that can.
- A new platform cannot be silent about this: composing a workstation that downloads onto one fails at
  composition, with the platform named, instead of failing on a missing command minutes into a build.
- The plan's package list becomes the record of what the image installs to be able to fetch, and `deskos check`
  already reports installed packages as outside its scope.
- `curl` remains Core's own choice, and the platform states the base can satisfy it, or the package that makes
  that true.
- This covers the one capability a build cannot proceed without, and leaves the rest to be added the same way.
