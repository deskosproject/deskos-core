# Migrating between releases

What to **edit** when you move an existing resource root or machine to a newer
DeskOS. A release note says what changed; read the section for the version you come
from, then the ones after it. Verify every download before you use it:
[check what you download](https://deskos.org/verify/).

## v0.23 → the output of `deskos update`

The command no longer prints the guard's conditions and bootc's own output by
default. It prints what the machine booted, what is already staged, and one line
saying whether this run changed anything and what to do next; `--verbose` adds
every condition and the tool's output.

Nothing needs editing on the machine. A script that parsed the old text does:
the summary is now derived from the deployment before and after the command, and
the old `staged. The machine applies it on the next reboot` line appeared even
when nothing had been staged.

## v0.22 → the lock screen follows the wallpaper

`appearance.lockShield` is **removed**. GNOME 40 and later draw the lock screen
from a blurred copy of the background, so the field lowered to an
`org.gnome.desktop.screensaver picture-uri` that nothing read.

- Delete the field from every `GnomeProfile`. A root that keeps it fails
  validation with an unknown field rather than ignoring it.
- The lock screen shows `appearance.wallpaper`. The wallpaper's new `options`,
  `primaryColor`, `secondaryColor` and `colorShadingType` describe the
  background, not the lock screen.
- An installed machine keeps the old key in its dconf database until the image
  updates. Nothing reads it, and the next update removes it.

`power.keyAction` and `power.lidSwitch` are new in the same release and lower to
`/etc/systemd/logind.conf.d/50-deskos-power.conf`, not to dconf; a machine that
sets them needs the new image, and `spec.locks` does not reach them.

## v0.18 → `/opt` and `/usr/local` on Fedora

- Fedora 45's base symlinks `/opt` to `var/opt` and `/usr/local` to
  `../var/usrlocal`, so a package or artifact that installs there writes to `/var`
  and the image does not carry it. The platform now declares
  `imageOwnedDirs: [/opt, /usr/local]`, and the build replaces both symlinks with
  real directories before the first package transaction.
- On an installed machine, the switch leaves the old content where it was: `/var/opt`
  and `/var/usrlocal` keep whatever was there, and `/opt` and `/usr/local` start
  empty with the image's content. Move or reinstall what you need; DeskOS does not
  move a user's files.

## v0.12 or earlier → names

- `deskosctl render --workstation deskos-core-centos10` is now
  `--workstation deskos-core-centos-stream-10`; likewise `-almalinux10` is
  `-almalinux-10`, `-fedora44` is `-fedora-44` and `-ubuntu2604` is
  `-ubuntu-2604`.
- `examples/core-<platform>/` is now `examples/<platform>/`, the workstation
  inside is `example-<platform>`, and the Ubuntu example that adds an
  organization's own signed package source is `example-ubuntu-2604-org-packages`.
- Image repositories follow the workstation names:
  `quay.io/deskos/deskos-core-centos10` is now
  `quay.io/deskos/deskos-core-centos-stream-10`, and the same for
  `-almalinux-10` and `-ubuntu-2604`. The old repositories keep the digests
  already published in them and are not deleted, but **they stop receiving new
  images**: a machine whose reference points at one stops updating until its
  reference changes.

## v0.11 or earlier → the desktop defaults

- DeskOS's GNOME defaults moved to the `deskos-gnome` profile, which a workstation
  now selects (ADR 0016). A workstation that adds `deskos-gnome` keeps what it
  had; one that does not stops inheriting the session lock, the idle timeout and
  the software-update policy. Ubuntu workstations keep their vendor's desktop and
  select neither.
- The dock and every extension are decided by UUID in `shell.extensions` rather
  than by `dock.enabled` (ADR 0017).

## v0.13 → the `deskos` companion

Core's foundation declares the runtime, so an image that also declares
`BinaryArtifact/deskos` fails composition with `duplicate resource
BinaryArtifact/deskos`. Remove your declaration and let the foundation carry it:
two files cannot define one key and composition has no removal (ADR 0003). The
version an image gets is the one Core pins.

## v0.17 → the Ubuntu platform

Ubuntu 26.04 lives in its own repository (ADR 0020), and a reference to
`quay.io/deskos/deskos-core-ubuntu-2604` now points there. Nothing else changes:
Core still composes and renders a Debian-family workstation, on a platform an
organization declares.

## Declaring your own platform

A `Platform` resource says whether its base can fetch a verified artifact
(`downloads.baseFetches`, and `downloads.package` when the base cannot). A plan
that downloads anything (a `BinaryArtifact` or an `RpmFile`) on a platform that
declares nothing fails composition with the platform named, instead of failing
minutes into a build with `curl: not found` (ADR 0019).

The platforms this repository ships declare it, so a workstation built on one
needs no change.
