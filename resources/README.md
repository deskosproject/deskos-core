# Resources

The resources **DeskOS Core ships**: the foundation a flavor references by name, the facts
about each platform, and the reference workstations. An organization's own root adds its
resources next to these, in whatever layout it likes — [`examples/organization-and-role/`](../examples/organization-and-role/)
is the shape of one.

The model behind them (envelopes, layers, schemas, how a set unions and a keyed definition
must match) is [`docs/resources.md`](../docs/resources.md). This file is only the map of
the directories, because a directory holding one kind and a directory named after one are
different things.

| Directory | Kinds it holds | What is in it |
|---|---|---|
| [`platforms/`](platforms/) | `Platform` | One per platform DeskOS ships: `centos-stream-10` and `rhel-10`. The Ubuntu platform is in its own repository (ADR 0020); AlmaLinux and Fedora live in their example roots. Every platform-specific name and fact lives here and nowhere else: package groups, boot splash, the base image by digest, and whether the base can fetch a verified artifact |
| [`profiles/`](profiles/) | `Profile` | The layers: `deskos-core` (the foundation), `deskos-gnome` (the desktop defaults), `deskos-look` (wallpaper, login logo, boot splash), `deskos-reference-apps`, `deskos-boot`, and the family ones (`rhel-family`, `fedora`) |
| [`workstations/`](workstations/) | `Workstation` | The reference workstations, `deskos-core-<platform>`: what a reader is meant to take. No RHEL one, because RHEL-derived images are never published |
| [`desktop/`](desktop/) | `GnomeProfile` | GNOME settings: `deskos-core` (the session basics) and `deskos-look` (the appearance) |
| [`software/`](software/) | `PackageSet`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` | Packages and application channels, plus `keys/` for the repository key assets those resources reference |
| [`system/`](system/) | `BootProfile`, `UpdatePolicy` | Boot appearance and the image update policy, including `deskos-splash` and the per-family update policies |
| [`assets/deskos/`](assets/deskos/) | — | The images the resources reference: wallpapers, the login logo, the splash watermark, with [`PROVENANCE.md`](assets/deskos/PROVENANCE.md) for where they come from |

Two kinds live **only in the examples**, which is worth knowing before hunting for them
here: `RpmRepository` and `TrustAnchor`, both in
[`examples/organization-and-role/`](../examples/organization-and-role/). `AptRepository`, the
third kind this repository renders but does not ship a resource for, appears in the tests and
in the Ubuntu repository, where apt is the platform's own package manager. `Theme` is the one kind this
repository defines without a resource using it yet: its example in
[`docs/resources.md`](../docs/resources.md) is illustrative, not a file here.
