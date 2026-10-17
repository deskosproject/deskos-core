# Example: DeskOS Core on Fedora 44

The minimal DeskOS Core composition on Fedora 44, on the Fedora Project's own
desktop bootc base, Silverblue. It is a **separate resource root** that references
the shipped `deskos-core` and `deskos-gnome` profiles and the shipped `fedora-44`
Platform by name:

```bash
deskosctl plan ./examples/core-fedora-44 \
  --workstation example-core-fedora-44
```

| Resource | Contents |
|---|---|
| `fedora-44` (platform) | shipped in `resources/platforms/`: Fedora facts on the vendor's bootc desktop base `quay.io/fedora/fedora-silverblue:44` |
| `example-core-fedora-44` | the target: `fedora-44` + the `deskos-core` and `deskos-gnome` profiles |

The base is published by the Fedora Project, so the platform follows ADR 0004's
official-base rule and declares no organization owner or tier (see the platform
resource). It is Silverblue and not the minimal `quay.io/fedora/fedora-bootc`
because that base carries no GNOME, no GDM, no Flatpak and no Plymouth at all:
the GNOME base stays distribution packages here, as it does on the EL platforms,
and only an organization's extra applications go to Flatpak — which is the
opposite of what a cloud-native desktop image such as Bluefin does with its own
GNOME base applications.

The platform's facts are measured against the pinned base (comments in
`resources/platforms/fedora-44.yaml`), and the image **builds**: two local builds
on 2026-10-17 (this example, image `c384826141cf`; and the reference workstation
`deskos-core-fedora44`, image `5b44f2829d1e`) completed with `bootc container
lint` reporting 12 checks passed and 2 warnings. No Fedora disk has been
**booted**, and there is no published Fedora image; the shipped reference
workstation is `deskos-core-fedora44`, and
[`../core-fedora-45/`](../core-fedora-45/README.md) is the prerelease evaluation
of the next Fedora release, which is not an offered platform.

**Naming.** The example is named after the Platform resource it references, as
`example-core-almalinux-10` references `almalinux-10` and
`example-core-ubuntu-2604` references `ubuntu-2604`; the shipped reference
workstation is named after the image it becomes (`deskos-core-fedora44`, like
`deskos-core-centos10`).
