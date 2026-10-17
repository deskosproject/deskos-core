# Example: DeskOS Core on Fedora 45

The minimal DeskOS Core composition on Fedora 45, on the Fedora Project's own
desktop bootc base, Silverblue. It is a **separate resource root** that references
the shipped `deskos-core` profile and the shipped `fedora-45` Platform by name:

```bash
deskosctl plan ./examples/core-fedora-45 \
  --workstation example-core-fedora-45
```

| Resource | Contents |
|---|---|
| `fedora-45` (platform) | shipped in `resources/platforms/`: Fedora facts on the vendor's bootc desktop base `quay.io/fedora/fedora-silverblue:45` |
| `example-core-fedora-45` | the target: `fedora-45` + the `deskos-core` profile |

The base is published by the Fedora Project, so the platform follows ADR 0004's
official-base rule and declares no organization owner or tier (see the platform
resource). Why Silverblue and not the minimal `quay.io/fedora/fedora-bootc`, and
why the GNOME base stays distribution packages rather than moving to Flatpak, is
written out in `../core-fedora-44/README.md` and in the platform resource.

**Measured caveat for this release:** on 2026-10-17 `:45` is a prerelease
(`45.20261016.n.0 (Silverblue Prerelease)`), and the base enables Fedora's
`updates-testing` repository, which the released `:44` does not. Packages
installed during a build can therefore come from testing. The platform's
`family: fedora` and the release in its name are facts; what Fedora's six-month
cadence means for support is not decided here.

The image has **not** been built, booted or checked here, and there is no
published Fedora image.
