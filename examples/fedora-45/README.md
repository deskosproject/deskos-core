# Example: DeskOS Core on Fedora 45

The minimal DeskOS Core composition on Fedora 45, on the Fedora Project's own
desktop bootc base, Silverblue. It is a **separate resource root** that references
the shipped `deskos-core` and `deskos-gnome` profiles and the shipped `fedora-45`
Platform by name:

```bash
deskosctl plan ./examples/fedora-45 \
  --workstation example-fedora-45
```

| Resource | Contents |
|---|---|
| `fedora-45` (platform) | shipped in `resources/platforms/`: Fedora facts on the vendor's bootc desktop base `quay.io/fedora/fedora-silverblue:45` |
| `example-fedora-45` | the target: `fedora-45` + the `deskos-core` and `deskos-gnome` profiles |

The base is published by the Fedora Project, so the platform follows ADR 0004's
official-base rule and declares no organization owner or tier (see the platform
resource). It is Silverblue and not the minimal `quay.io/fedora/fedora-bootc`
because that base carries no GNOME, no GDM, no Flatpak and no Plymouth at all:
the GNOME base stays distribution packages here, as it does on the EL platforms,
and only an organization's extra applications go to Flatpak — which is the
opposite of what a cloud-native desktop image such as Bluefin does with its own
GNOME base applications.

The platform is pinned to a **prerelease** compose of Fedora 45
(`45.20261019.n.0`), whose base enables Fedora's `updates-testing` repository.
`resources/platforms/fedora-45.yaml` records that, and the rest of the measured
facts, against the pin.

The image **builds**; see the build evidence in
[platforms.md](../../docs/platforms.md). CI builds a QCOW2 of the example and boots
it — boot, session and first-boot checks — though that run is not yet green, so
there is no Fedora boot evidence to claim; no Fedora image is published, and no
Fedora disk has been installed on hardware. The shipped reference workstation is
`deskos-core-fedora-45`, and Fedora 44 is retired
([roadmap.md](../../docs/roadmap.md)).

**Naming.** The example is named after the Platform resource it references, as
`example-almalinux-10` references `almalinux-10`; the shipped reference
workstation is named after the image it becomes (`deskos-core-fedora-45`, like
`deskos-core-centos-stream-10`). The Ubuntu platform follows the same rule in its own
repository (`deskos-ubuntu` on `ubuntu-2604`).
