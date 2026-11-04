# Fedora 45

A shipped DeskOS platform, compiled onto Fedora's own Silverblue bootc image.
GNOME 51 comes from the vendor, so the desktop is the one Fedora validates rather
than one DeskOS assembles from `fedora-bootc`.

## The base

`quay.io/deskos/fedora-silverblue:45`, a mirror of the vendor image, pinned by
digest in `resources/platforms/fedora-45.yaml`.

Fedora publishes only moving tags and stops serving an old digest within days, so
a commit that pins the vendor's digest stops resolving. The mirror copies the
current image into DeskOS's namespace with `--all --preserve-digests`, keeping the
vendor's index digest and every manifest. It is refreshed by a person, never on a
schedule:

```bash
gh workflow run mirror-fedora.yml \
  --repo deskosproject/deskos-core \
  --ref main
```

The job prints the source digest. Bumping the pin is a pull request: set
`spec.bootc.image` to the mirror and `spec.bootc.digest` to that digest.

Silverblue, not `quay.io/fedora/fedora-bootc`: measured on the minimal base (:44
and :45, 2026-10-17), it has no `gnome-shell`, no `gdm`, no `gnome-initial-setup`,
no Flatpak, no Plymouth, no `fedora-logos` and no `gtk-update-icon-cache`. A DeskOS
workstation on it would be a desktop DeskOS invented.

Silverblue carries all of them as distribution packages: `gnome-shell` 51.1,
`gdm` 51.0, `nautilus` 51.1, `gnome-software` 51.0 and `firefox` 159.0 (measured
2026-11-01), 1509 packages in total.

## Repositories

The base enables `fedora`, `updates`, `updates-archive` and Fedora's openh264
repository, read from `/usr/share/dnf5/repos.d` rather than `/etc/yum.repos.d`, and
it does not enable `updates-testing`. The resource decides which compose a build
gets; the repositories follow the pinned image.

Fedora 45 moved those definitions with
[Changes/RelocateRpmRepoConfigsToUsr](https://fedoraproject.org/wiki/Changes/RelocateRpmRepoConfigsToUsr).
`bootc-image-builder` still reads `/etc/yum.repos.d` when it builds the installer,
and stopped with `NoReposError: There are no enabled repositories` (run
`41140733589`).

The platform declares `repoConfigDirs: [/usr/share/dnf5/repos.d]`, so the image
carries the same files in both places and dnf5 keeps reading the new one. They are
copies, not links: BIB's installer depsolve reads the directory without the tree
the links point at and finds no repositories (run `41304158163`, where every
installer package answered `No match for argument`).

Silverblue keeps OSTree's strict layout: `/opt` is a symlink to `var/opt` and
`/usr/local` to `../var/usrlocal`, so a package or artifact that installs there
writes to `/var`, which an image does not ship. The platform declares
`imageOwnedDirs: [/opt, /usr/local]` and the build replaces both symlinks with
real directories before the first package transaction.

Content an installed machine had in `/var/opt` or `/var/usrlocal` stays there
and is not moved ([migrations](migrations.md)).

## What DeskOS adds

The GNOME base stays distribution packages; only an organization's extra
applications go to Flatpak (ADR 0005). Fedora's own `firefox` RPM is the
browser, not a Flatpak: it is already in the base, and naming it in
`PackageSet/firefox` keeps the same browser on every flavor.

The platform installs `workstation-product-environment` to complete the desktop,
removes GNOME Software so Bazaar is the application store (ADR 0022), and
declares the terminal, the extensions and the boot arguments as facts, the same
way the EL10 platforms do.

The workstation carries `deskos-look`, so Fedora boots with the DeskOS splash:
Plymouth's stock spinner frames from the vendor package, made the default theme
and copied into the initramfs, which the build asserts. Fedora's own wallpaper is
replaced by the DeskOS one ([desktop parity](desktop-parity.md)); a workstation
that wants only the boot behaviour takes `deskos-boot`.

The platform also names Adwaita Sans and Adwaita Mono, the families its base
already ships, so the desktop is drawn in the same families as CentOS Stream 10.

## Status

| Check | Today |
|---|---|
| Composes and renders | validated by the compiler tests and `checks.yml` |
| Image built | CI, pull requests that change the platform, its Core resources or the workflow |
| Booted and checked | CI: GNOME, session check |
| Published image | **published** (`quay.io/deskos/deskos-core-fedora-45`); Core publishes no installer for it |
| System Flatpak | preinstall |

The package manifest (ADR 0028) is `manifests/fedora-45.txt`, measured on this
base and checked by the build. A downstream organization builds the installer
from the published image.
