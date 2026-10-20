# ADR 0021: An extension the platform's repositories do not carry as a pinned artifact

**Status:** accepted (2026-10-20)

## Context

A GNOME Shell extension is a platform fact: ADR 0017 made the platform's
`gnome.extensions[]` a catalog of `{name, package, uuid}`, and a profile decides
which catalogued UUID to enable. The package name is the platform's, because Core
knows no package names (ADR 0004).

That model assumes the platform's repositories carry the extension. EL10 is the
case where they do not. Measured on 2026-10-20:

- `gnome-shell-extension-appindicator` is absent from baseos, appstream, extras
  and CRB on CentOS Stream 10 and AlmaLinux 10.
- EPEL 10 publishes `61-1.el10_2`, and it is **uninstallable on CentOS Stream 10**:
  it requires `libappindicator-gtk3`, whose `libdbusmenu-glib.so.4` and
  `libdbusmenu-gtk3.so.4` come from CRB, which CentOS Stream does not enable and
  RHEL does not support. The same transaction resolves on AlmaLinux 10, where CRB
  is enabled by default.
- Building the extension from its git repository (what Bluefin LTS does in
  `21-build-gnome-extensions.sh`) produces the same bytes upstream already
  publishes as a release zip, with a compiler toolchain in the build and a commit
  pin that is harder to verify than a pinned artifact.

Upstream publishes that zip per release: for v66,
`appindicatorsupport@rgcjonas.gmail.com.zip`, sha256
`f2b17c7c5590f729853cd353f20a9e59b6e0c32e9c0a561976e09f5a97487730`, whose
`metadata.json` declares `shell-version` 45–51. It is self-contained: JavaScript,
schemas XML, compiled locales. It does not need the `libappindicator` library the
RPM depends on; that dependency belongs to the packaging, not to the Shell side.
Measured on the CentOS Stream 10 base: `python3`'s `zipfile` extracts it,
`glib-compile-schemas` compiles its schemas, and no `unzip` package is needed.

Two more measured facts bound the decision. EL10 ships gnome-shell 49.5, so the
zip's declared support covers it; and `gnome-extensions` (which the session check
already uses) ships in the `gnome-shell` package, so the session can be asked
whether the UUID loads.

## Decision

1. **A catalog entry names one delivery.** `Platform.gnome.extensions[]` keeps
   `name` and `uuid`, and requires exactly one of:
   - `package`: the platform's own package, as today (Fedora 45); or
   - `source`: a pinned artifact — `version`, an https `url` and a `sha256`.

   The catalog stays a platform fact and enablement stays a profile decision
   (ADR 0017). No new public kind: this is a second shape of an existing entry,
   not a new concept.

2. **The Plan carries the delivery.** A composed extension becomes an
   `artifact.extensions[]` entry `{uuid, version, url, sha256, provenance}`,
   never a package and never a binary. Provenance is the platform catalog entry
   plus the profile decision that enabled it.

3. **The build verifies the artifact, not the declaration.** The backend
   downloads the zip, verifies its sha256, extracts it into a temporary directory
   rejecting duplicate names, symlinks, absolute paths and `..`, with bounded
   member count and size, and then:
   - requires the extracted `metadata.json` to carry the catalogued UUID and a
     `shell-version` that contains the major version read from
     `gnome-shell --version` in the image (49 on EL10, 51 on Fedora 45); a
     mismatch fails the build naming both;
   - compiles the extension's schemas from XML with `glib-compile-schemas` and
     ignores the zip's compiled file;
   - installs into `/usr/share/gnome-shell/extensions/<uuid>` with explicit modes
     and fails if that directory already exists.

   The version support is a claim by upstream; the check is what makes it a fact
   on this image.

4. **A platform that cannot deliver still says so.** `unavailablePackages` keeps
   recording that the RPM is not in the platform's supported repositories, with a
   reason that says DeskOS delivers the extension itself. A resource that asks for
   the package by name keeps failing at composition; a profile that enables the
   UUID gets the artifact.

5. **The evidence is the load and the watcher, not a screenshot.** The session
   check asks `gnome-extensions` about every enabled UUID, and the behavior probe
   asserts that the session bus carries the extension's
   `org.kde.StatusNotifierWatcher` service. That proves the extension runs and
   answers; it does not prove that a particular application's icon is drawn, and
   documentation says "loads and serves the watcher", not "works with every app".

## Consequences

- EL10 gets tray icons without EPEL, without CRB, without a package fork and
  without a build toolchain: one pinned zip whose bytes and license are recorded.
- Fedora keeps the distro package; the two platforms deliver the same UUID
  through different means, and the cross-platform test asserts exactly that
  difference instead of hiding it.
- The extension's license is its own. Upstream declares GPL2 in `meson.build`; the
  zip is redistributed unmodified inside the image, and its source URL travels in
  the declared SBOM.
- The zip is a new network input at build time, which is why the platform must
  still declare `downloads` (ADR 0019) and why the URL joins the CI check that
  pinned inputs are still served.
- Refreshing the extension is a reviewable change to a version, a URL and a
  sha256. A GNOME release outside the zip's declared range fails the build here,
  with the two versions named, rather than at a user's first login.
