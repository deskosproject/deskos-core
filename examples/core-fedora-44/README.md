# Example: DeskOS Core on Fedora 44

The minimal DeskOS Core composition on Fedora 44, on the Fedora Project's own
desktop bootc base, Silverblue. It is a **separate resource root** that references
the shipped `deskos-core` profile and the shipped `fedora-44` Platform by name:

```bash
deskosctl plan ./examples/core-fedora-44 \
  --workstation example-core-fedora-44
```

| Resource | Contents |
|---|---|
| `fedora-44` (platform) | shipped in `resources/platforms/`: Fedora facts on the vendor's bootc desktop base `quay.io/fedora/fedora-silverblue:44` |
| `example-core-fedora-44` | the target: `fedora-44` + the `deskos-core` profile |

The base is published by the Fedora Project, so the platform follows ADR 0004's
official-base rule and declares no organization owner or tier (see the platform
resource). It is Silverblue and not the minimal `quay.io/fedora/fedora-bootc`
because that base carries no GNOME, no GDM, no Flatpak and no Plymouth at all:
the GNOME base stays distribution packages here, as it does on the EL platforms,
and only an organization's extra applications go to Flatpak — which is the
opposite of what a cloud-native desktop image such as Bluefin does with its own
GNOME base applications.

The platform's facts are measured against the pinned base (see the comments in
`resources/platforms/fedora-44.yaml`); the image has **not** been built, booted or
checked here, and there is no published Fedora image. The Fedora family profile
`fedora` is the family's own browser, firewall and Flatpak declaration; on this
base it installs nothing, because Silverblue already carries all three.
