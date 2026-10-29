# Migrating between releases

What to **edit** when you move an existing resource root or machine to a newer
DeskOS. A release note says what changed; read the section for the version you come
from, then the ones after it. Verify every download before you use it:
[check what you download](https://deskos.org/verify/).

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
minutes into a build with `curl: not found` (ADR 0019). The platforms this
repository ships declare it, so a workstation built on one needs no change.
