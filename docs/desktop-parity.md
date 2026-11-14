# Desktop parity across flavors

Question: what does a DeskOS desktop look like on each flavor, and where does it
differ?

Every flavor composes the same desktop from the same profiles. A platform
contributes its stack and, on one flavor, its families; it does not contribute
the look.

The reference workstations are [CentOS Stream
10](../resources/workstations/deskos-core-centos-stream-10.yaml), [AlmaLinux
10](../resources/workstations/deskos-core-almalinux-10.yaml) and [Fedora
45](../resources/workstations/deskos-core-fedora-45.yaml); the RHEL fleet is
built by the organization ([example platforms](example-platforms.md)).

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

Measured 2026-11-01 with `dnf repoquery` on the digests then pinned: CentOS
Stream 10 `0af0dc39`, AlmaLinux 10 `0a036afe`, Fedora 45 `900e1adf`. The CentOS
pin moved to `f43c24c5` on 2026-11-03; re-measure before relying on a version
tie. The RHEL column is the installed stack of the 10.3 factory build
(2026-11-14), not a repoquery; AlmaLinux 10.3 is not published yet.

| | CentOS Stream 10 | AlmaLinux 10 | Fedora 45 | RHEL 10.3 |
|---|---|---|---|---|
| GNOME Shell | 49.5-18 | 49.4-9 | **51.1-1** | 49.5-13 |
| GDM | 47.0-32 | 47.0-24 | **51.0-1** | 47.0-32 |
| Ptyxis | 47.13-1 | 47.13-1 | **50.1-6** | 47.13-1 |
| Flatpak | **1.18.0-2** | 1.16.0-9 | 1.18.4 | 1.18.0-1 |
| Mesa | 26.1.1-2 | 25.2.7-4 | 26.2.3-1 | 26.1.1-2 |
| PipeWire | 1.6.9-1 | 1.4.11-1 | 1.6.9-1 | 1.4.11-1.1.1 |
| Firefox | 140.17.0-1 | 153.5.0-1 | **159.0-1** | 153.7.0-1 |

RHEL 10.3 values are the installed packages of the factory build (2026-11-14);
their release suffixes are `el10` and `el10_3`. The other columns are `dnf
repoquery` results on the pinned digests. Fedora's Flatpak package reports
`1.19.0^really1.18.4`; the table shows the upstream release it carries.

## What the numbers say

- **CentOS Stream 10 is the branch RHEL 10.3 is cut from**, and 10.3 has caught
  up with it on GNOME (49.5), Flatpak (1.18) and Mesa (26.1). PipeWire stays one
  branch behind (1.4 against 1.6). "Abandoned" is the wrong reading.
- **AlmaLinux 10.2 is RHEL 10.2 with its own branding**, plus its own choices: CRB
  is enabled by default, and Firefox and Thunderbird are rebased to the latest
  upstream release (153.5). RHEL 10.3 has since rebased its own Firefox to 153.7,
  so the older contrast with an ESR 140 no longer holds; AlmaLinux 10.3 is not
  published yet.
- **Fedora 45 is the newest stack** (GNOME 51, GDM 51, Ptyxis 50) and publishes the
  newest Firefox (159.0) because it tracks the release rather than a vendor's
  cadence.

## What this page is not

It is not a promise that a vendor's next minor release keeps the same versions;
every pin and every platform decision is reviewed when it moves. It is also not a
support contract: the vendor still labels the AlmaLinux bootc base experimental,
and [Platforms](platforms.md) records what was verified.
