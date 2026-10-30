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
gh workflow run mirror-fedora.yml --repo deskosproject/deskos-core --ref main
```

The job prints the source digest. Bumping the pin is a pull request: set
`spec.bootc.image` to the mirror and `spec.bootc.digest` to that digest.

Silverblue, not `quay.io/fedora/fedora-bootc`: measured on the minimal base (:44
and :45, 2026-10-17), it has no `gnome-shell`, no `gdm`, no `gnome-initial-setup`,
no Flatpak, no Plymouth, no `fedora-logos` and no `gtk-update-icon-cache`. A DeskOS
workstation on it would be a desktop DeskOS invented. Silverblue carries all of
them as distribution packages: `gnome-shell` 51.0, `gdm` 51.0, `nautilus` 51.0.1,
`gnome-software` 51.0 and `firefox` 158.0.1 (measured 2026-10-30), 1509 packages in
total.

## Repositories

The base enables `fedora`, `updates`, `updates-archive` and Fedora's openh264
repository, read from `/usr/share/dnf5/repos.d` rather than `/etc/yum.repos.d`, and
it does not enable `updates-testing`. The resource decides which compose a build
gets; the repositories follow the pinned image.

## What DeskOS adds

The GNOME base stays distribution packages; only an organization's extra
applications go to Flatpak (ADR 0005). The platform installs
`workstation-product-environment` to complete the desktop, removes GNOME Software
so Bazaar is the application store (ADR 0022), and declares the terminal, the
extensions and the boot arguments as facts, the same way the EL10 platforms do.

The workstation carries `deskos-look`, so Fedora boots with the DeskOS splash:
Plymouth's stock spinner frames from the vendor package, made the default theme
and copied into the initramfs, which the build asserts. Fedora's own wallpaper is
replaced by the DeskOS one, exactly as on CentOS Stream 10; a workstation that
wants only the boot behaviour and not the look takes `deskos-boot`.

## Status

| | |
|---|---|
| Composes and renders | validated by the compiler tests and `ci.yml` |
| Image built | CI, every pull request |
| Booted and checked | CI: GNOME, session check |
| Published image | not yet |
| System Flatpak | preinstall |

The package manifest (ADR 0028) is not declared yet: the group resolution has to be
measured on the mirrored digest and committed, which is the next step before
publication.
