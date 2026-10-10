# ADR 0013: Theme as the thirteenth public kind

**Status:** accepted (2026-10-09)

## Context

A workstation's look is a named thing. palette ships themes as a palette
(`colors.toml`: a mode, an accent, background/foreground and terminal
colors) alongside wallpapers, and `palette theme set <name>` applies it
across the desktop. DeskOS's `GnomeProfile` can already set the appearance
one workstation at a time (color scheme, accent, icon and cursor theme,
fonts, wallpaper), but there is no **reusable, named palette**: every
workstation repeats the values, and a free accent color has nowhere to go —
`accentColor` accepts only GNOME's nine names.

GNOME defines the surfaces a system image can theme without patching a
package:

- `org.gnome.desktop.interface`: `color-scheme`, `accent-color` (nine named
  accents), `gtk-theme`, `icon-theme`, `cursor-theme`, fonts.
- `org.gnome.desktop.background`: the wallpaper.
- GNOME Shell follows the accent color (GNOME 47+).

Full GTK/libadwaita and Shell recoloring needs CSS and, for the Shell, the
`user-theme` extension; that is out of scope of this decision.

## Decision

Add `desktop.deskos.org/v1alpha1` `Theme`:

| Field | Value |
|---|---|
| `mode` | `dark` or `light`, lowered to `color-scheme` |
| `accent` | any `#RRGGBB`, mapped to the closest GNOME accent by hue |
| `iconTheme`, `cursorTheme` | theme names, lowered as-is |
| `fonts` | the same font shape as `GnomeProfile.appearance.fonts` |
| `wallpaper` | light/dark assets, like `GnomeProfile.appearance.wallpaper` |

A Theme contributes to the **same** GNOME setting domain as
`GnomeProfile`, so layering, locking and conflict reporting work unchanged:
a Theme and a GnomeProfile that both set `appearance.accentColor` at the
same layer conflict, and a higher layer wins. A Theme sets at least one
field; a defined schema and typed decoder both enforce the rules.

Mapping a free accent to one of nine is a loss, so it is done by **hue** (a
light green stays green), with a `slate` fallback for near-achromatic
colors.

## Slice 2: the palette (implemented)

`palette` makes a Theme generate the look's overrides. `background` and
`foreground` produce a GTK4/libadwaita and a GTK3 named-color override
(`--accent-*`, `--window-*`, `--view-*`, `--headerbar-*`, `--card-*`,
`--sidebar-*`, `--popover-*`; `@accent_*`, `@theme_*`, …). The six normal
ANSI colors (`red`…`cyan`, plus optional `black`/`white`) additionally
produce a **Ptyxis terminal palette**, and a dconf drop in
(`distro.d/60-deskos-ptyxis`) that provisions a fixed Ptyxis profile and
selects that palette, so a terminal uses the Theme by default. The profile
keys live under a relocatable schema, so they are a generated dconf fragment
with a bracketed section. The compiler synthesizes the files (a new
`GeneratedFile` IR leaf, which needs no asset) and installs them at
`/etc/skel/…` and `/etc/dconf/db/distro.d/…`, so users created by the
installer or GNOME Initial Setup inherit them; libadwaita 1.4+, GTK3 and
Ptyxis read them. Whether Ptyxis honours the preset profile is not yet
verified in a booted session.

The **GNOME Shell** is themed by `deskos-theme apply --shell-css`: it
replaces gnome-shell's Adwaita base colors in the distro's compiled
stylesheet and writes two packs (`deskos-a`/`deskos-b`), so the running Shell
can be told to reload by switching between them. It needs the **User Themes**
extension, and derived colors (shadows, borders) stay as compiled. The
command also generates a **Ghostty** theme, and reads either a DeskOS `Theme`
or an **palette** theme directory (`colors.toml`).

## Consequences

- The public API has **exactly thirteen kinds**; a test enforces the count.
- Themes are how DeskOS expresses "many looks" without repeating settings;
  an palette `colors.toml` is the reference palette shape.
- `deskos-theme` is the runtime applier (best effort); the compiler bakes the
  install-time default. Neither is verified in a booted session yet.
