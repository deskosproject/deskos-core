# Resources

The resources this repository ships: the foundation an organization references by
name, the facts about each platform, and the reference workstations. An
organization adds its own next to these in any layout; the
[example organization](../examples/organization-and-role/) is one shape.

The model behind them, meaning envelopes, layers and how sets and keyed
definitions compose, is in [docs/resources.md](../docs/resources.md). This file is
the map of the directories.

| Directory | Kinds | What is in it |
|---|---|---|
| [`platforms/`](platforms/) | `Platform` | one per shipped platform, `centos-stream-10` and `rhel-10`; every platform-specific fact lives here: package groups, boot splash, base digest, and whether the base can fetch a verified artifact |
| [`profiles/`](profiles/) | `Profile` | the layers: `deskos-core`, `deskos-gnome`, `deskos-look`, `deskos-reference-apps`, `deskos-boot`, and the family profiles (`rhel-family`, `fedora`) |
| [`workstations/`](workstations/) | `Workstation` | `deskos-core-<platform>`, the reference workstations; no RHEL one, because RHEL images are never published |
| [`desktop/`](desktop/) | `GnomeProfile` | `deskos-core` for the session basics and `deskos-look` for appearance |
| [`software/`](software/) | `PackageSet`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` | packages and application channels, plus `keys/` for the repository key assets |
| [`system/`](system/) | `BootProfile`, `UpdatePolicy`, `TrustAnchor` | boot appearance, update policy, and `deskos-distrusted-roots` for compromised CA roots |
| [`assets/deskos/`](assets/deskos/) | images | wallpapers, login logo, splash watermark, with [`PROVENANCE.md`](assets/deskos/PROVENANCE.md) |
| [`assets/ca/`](assets/ca/) | certificates | the public CA certificates the blocklist names, with [`PROVENANCE.md`](assets/ca/PROVENANCE.md) |

Some kinds are not here:

- `RpmRepository`, and a `TrustAnchor` that adds a CA, live only in the
  [example organization](../examples/organization-and-role/).
- `AptRepository` is rendered but has no shipped resource; its examples are in the
  tests and in the Ubuntu repository, where apt is the platform's own package
  manager.
- `Theme` has no resource using it yet; its example in
  [docs/resources.md](../docs/resources.md) is illustrative.
- The Ubuntu platform is in its own repository (ADR 0020); AlmaLinux and Fedora
  live in their example roots.
