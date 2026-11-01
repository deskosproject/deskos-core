# Desktop parity across flavors

Question: what does a DeskOS desktop look like on each flavor, and where does it
differ?

Every flavor composes the same desktop from the same profiles. A platform
contributes its stack and, on one flavor, its families; it does not contribute the
look. The reference workstations are
[CentOS Stream 10](../resources/workstations/deskos-core-centos-stream-10.yaml),
[Fedora 45](../resources/workstations/deskos-core-fedora-45.yaml) and
[AlmaLinux 10](../resources/workstations/deskos-core-almalinux-10.yaml); the RHEL
fleet is built by the organization ([example platforms](example-platforms.md)).

## What every flavor shares

| Piece | Resource | What it is |
|---|---|---|
| Wallpaper, login logo, splash | `Profile/deskos-look` | the DeskOS artwork: light and dark SVG wallpapers, the GDM lockup and the Plymouth watermark |
| Window buttons | `GnomeProfile/deskos-core` | minimize, maximize and close, on the right |
| Hot corner | `GnomeProfile/deskos-core` | the top-left corner opens the overview |
| Dock | `Profile/deskos-dock` | Dash to Dock at the bottom, intellihide, 40 px icons, no trash |
| Tray | `Profile/deskos-gnome` | the pinned AppIndicator artifact (ADR 0021) |
| Store | `Profile/deskos-reference-apps` | Bazaar; every platform removes GNOME Software and the build fails if it is still installed (ADR 0022) |
| Session | `GnomeProfile/deskos-core` | session locked at login, screen blank after 10 minutes, software updates manual |
| Command line | `Profile/deskos-gnome` | zsh, tmux, rsync, bash-completion, tree, wget and vim-enhanced |
| Families | `Profile/deskos-adwaita` | Adwaita Sans and Adwaita Mono: pinned artifacts on EL10, the base's own files on Fedora 45 |

These are defaults, not locks: an organization that declares another wallpaper,
login logo or set of buttons wins at its layer, and `make check` shows it in the
example organization. Core never locks (invariant 8).

The families are the one deliberate difference. RHEL 10 keeps the families its
desktop ships, **Red Hat Text** and **Red Hat Mono**, because the fleet that runs it
keeps the vendor's look; every other flavor draws GNOME's Adwaita Sans and Adwaita
Mono. EL10 does not package those, so they arrive as pinned artifacts on EL10 and
from the base on Fedora (ADR 0026).

## What each platform brings

Measured 2026-11-01 with `dnf repoquery` on the pinned digests: CentOS Stream 10
`0af0dc39`, AlmaLinux 10 `0a036afe`, Fedora 45 `d81de11d`.

| | CentOS Stream 10 | Fedora 45 | AlmaLinux 10 | RHEL 10.2 |
|---|---|---|---|---|
| GNOME Shell | 49.5-18 | **51.0-1** | 49.4-9 | 49.4-9* |
| GDM | 47.0-32 | **51.0-1** | 47.0-24 | 47.0* |
| Ptyxis | 47.13-1 | **50.1-6** | 47.13-1 | 47.13* |
| Flatpak | **1.18.0-2** | 1.18.4 | 1.16.0-9 | 1.16.0-9† |
| Mesa | 26.1.1-2 | 26.2.3-1 | 25.2.7-4 | 25.2.7* |
| PipeWire | 1.6.9-1 | 1.6.9-1 | 1.4.11-1 | 1.4.11* |
| Firefox | 140.17.0-1 | **158.0.1-2** | 153.5.0-1 | 140.16.0-1† |

`*` RHEL 10.2 publishes the version AlmaLinux rebuilds: the `el10_2` NEVRA without
the `.alma` suffix. `†` recorded in this repository
([ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md), the RHEL 10.2
scans in [research notes](research-notes.md)). Fedora's Flatpak package reports
`1.19.0^really1.18.4`; the table shows the upstream release it carries.

## What the numbers say

- **CentOS Stream 10 is the branch RHEL 10.3 is cut from**, and it is ahead of
  RHEL 10.2 on the stack: GNOME 49.5 against 49.4, Flatpak 1.18 against 1.16, Mesa
  26.1 against 25.2, PipeWire 1.6 against 1.4. "Abandoned" is the wrong reading.
- **AlmaLinux 10.2 is RHEL 10.2 with its own branding**, plus its own choices: CRB
  is enabled by default, and Firefox and Thunderbird are rebased to the latest
  upstream release (153.5) instead of the ESR 140 that RHEL and CentOS Stream ship.
  That is a support model, not a polish level: features sooner, no ESR window and no
  vendor validation against the desktop.
- **Fedora 45 is the newest stack** (GNOME 51, GDM 51, Ptyxis 50) and publishes the
  newest Firefox (158.0.1) because it tracks the release, not the ESR.

## What this page is not

It is not a promise that a vendor's next minor release keeps the same versions;
every pin and every platform decision is reviewed when it moves. It is also not a
support contract: the vendor still labels the AlmaLinux bootc base experimental,
and [Platforms](platforms.md) records what was verified.
