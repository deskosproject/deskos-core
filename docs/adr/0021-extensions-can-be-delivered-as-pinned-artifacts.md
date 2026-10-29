# ADR 0021: An extension the platform's repositories do not carry as a pinned artifact

**Status:** accepted (2026-10-20)

## Context

ADR 0017's `gnome.extensions[]` catalog assumes the platform's repositories carry the extension; EL10's do not
(measured 2026-10-20). `gnome-shell-extension-appindicator` is absent from baseos, appstream, extras and CRB, and EPEL
10's `61-1.el10_2` is uninstallable on CentOS Stream 10, because its `libappindicator-gtk3` needs CRB libraries RHEL
does not support. Upstream publishes a self-contained zip: v66 `appindicatorsupport@rgcjonas.gmail.com.zip`, sha256
`f2b17c7c5590f729853cd353f20a9e59b6e0c32e9c0a561976e09f5a97487730`, `shell-version` 45-51; EL10 has 49.5.

## Decision

1. **A catalog entry names one delivery.** `Platform.gnome.extensions[]` keeps `name` and `uuid` and requires exactly
   one of `package` (the platform's own package, as on Fedora 45) or `source` (a pinned artifact: `version`, an https
   `url` and a `sha256`). The catalog stays a platform fact and enablement a profile decision; no new public kind.
2. **The Plan carries the delivery**: an `artifact.extensions[]` entry `{uuid, version, url, sha256, provenance}`,
   never a package and never a binary. Provenance is the catalog entry plus the enabling decision.
3. **The build verifies the artifact, not the declaration.** It downloads the zip, verifies its sha256, extracts it
   rejecting duplicate names, symlinks, absolute paths and `..` with bounded member count and size, then requires the
   extracted `metadata.json` to carry the catalogued UUID and a `shell-version` containing the major version read from
   `gnome-shell --version` in the image (49 on EL10, 51 on Fedora 45), a mismatch failing the build naming both;
   compiles the schemas from XML with `glib-compile-schemas`, ignoring the zip's compiled file; and installs into
   `/usr/share/gnome-shell/extensions/<uuid>` with explicit modes, failing if it already exists.
4. **A platform that cannot deliver still says so.** `unavailablePackages` records that the RPM is not in the
   supported repositories; a resource asking for the package by name keeps failing at composition, and a profile that
   enables the UUID gets the artifact.
5. **The proof is the load and the watcher, not a screenshot.** The session check asks `gnome-extensions` about every
   enabled UUID, and the behavior probe asserts that the session bus carries the extension's
   `org.kde.StatusNotifierWatcher` service and accepts a registration. That proves it runs and answers, not that an
   icon is drawn; documentation says "loads and serves the watcher".

## Consequences

- EL10 gets tray icons without EPEL, without CRB, without a package fork and without a build toolchain: one pinned zip
  whose bytes and license are recorded.
- Fedora keeps the distro package; the cross-platform test asserts that difference instead of hiding it.
- The extension's license is its own: upstream declares GPL2 in `meson.build`, the zip is redistributed unmodified,
  and its source URL travels in the declared SBOM.
- The zip is a new network input at build time, which is why the platform must still declare `downloads` (ADR 0019)
  and why the URL joins the CI check that pinned inputs are still served.
- Refreshing the extension is a reviewable change to a version, a URL and a sha256; a GNOME release outside the zip's
  declared range fails the build here, with both versions named.
- Two changes the Fedora boot check forced are recorded in
  [research-notes.md](../research-notes.md#fedoras-rebuilt-initramfs-dropped-ostree-and-bootc-2026-10-20): the backend
  now builds a generic initramfs (`dracut --no-hostonly`), and the Fedora check builds an ext4 disk.
