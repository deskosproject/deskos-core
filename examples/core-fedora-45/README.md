# Example (evaluation): DeskOS Core on Fedora 45

> [!WARNING]
> **Not an offered platform.** Fedora 45 was a prerelease when it was measured
> (2026-10-17) and its base enables Fedora's `updates-testing` repository. DeskOS
> offers **Fedora 44** only (`resources/platforms/fedora-44.yaml`); this root is
> the prerelease evaluation, it is not in the platform matrix of
> [docs/platforms.md](../../docs/platforms.md), and it has no reference
> workstation and no published image. No Fedora 45 image was built.

The point of the root is to find out whether the next Fedora release composes with
the same Fedora facts before anyone commits to it — so it carries **its own
Platform resource** (`platforms/fedora-45.yaml`) instead of shipping one, and
evaluating a release needs no change to what DeskOS offers:

```bash
deskosctl plan ./examples/core-fedora-45 \
  --workstation example-core-fedora-45
```

| Resource | Contents |
|---|---|
| `fedora-45` (platform, in this root) | Fedora 45 facts on the vendor's bootc desktop base `quay.io/fedora/fedora-silverblue:45`, pinned by digest |
| `example-core-fedora-45` | the target: `fedora-45` + the `deskos-core` profile |

The facts are measured against the pinned base and are the ones `fedora-44.yaml`
records, re-read on this release: the comps environment dnf5 resolves for the
`workstation` group, Flatpak 1.18.4 with `flatpak preinstall`, GDM's
`/etc/gdm/custom.conf`, the `distro` dconf database, `update-ca-trust`,
`gtk-update-icon-cache` (gtk3 3.24.52), and the bootc units present but not
enabled. What differs is the release: gnome-shell 51.0 (vs 50.6 on 44), Plymouth
26.134.222 (vs 24.004.60), Dash to Dock 110 (vs 105).

**Naming.** The example is named after the Platform resource it references, as
`example-core-almalinux-10` references `almalinux-10` and
`example-core-ubuntu-2604` references `ubuntu-2604`; the shipped reference
workstation is named after the image it becomes (`deskos-core-fedora44`, like
`deskos-core-centos10`).
