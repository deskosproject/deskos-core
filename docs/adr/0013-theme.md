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

## Slice 2: the palette and the applier (implemented)

`palette` makes a Theme generate the look's overrides. `background` and
`foreground` produce a GTK4/libadwaita override, deliberately conservative:
only the window and view surfaces, so libadwaita keeps its own hierarchy,
contrast and interaction states. The six normal ANSI colors (`red`…`cyan`,
plus optional `black`/`white`) additionally produce a **Ptyxis terminal
palette**, a dconf drop in (`distro.d/60-deskos-ptyxis`) that provisions a
Ptyxis profile and selects that palette, and a **Ghostty** theme. The compiler
synthesizes the files (a new `GeneratedFile` IR leaf, which needs no asset).

**The accent is not overridden in CSS.** The accent is a GNOME setting: the
generated override carries no accent, so apps and the Shell follow the native
`accent-color` and a user who changes it keeps a consistent look. (A pinned
CSS accent would not follow, and `var(--accent-*)` needs GTK ≥ 4.16.) Mapping
a free accent to one of nine is done by **hue**, with a `slate` fallback.

**Where the files go.** The compiler installs them into `/etc/skel/…` and
`/etc/dconf/db/distro.d/…`. The dconf defaults reach every user who has not
pinned the key; the `/etc/skel` files reach only accounts created after the
image. Existing accounts keep their own `gtk-4.0/gtk.css`, and a first-login
unit writing into `$HOME` would create persistent state that competes with a
user's own file. Migrating an existing account is therefore out of scope (a
future explicit user command, with a preview, would be the way).

**`deskos-theme`** is the runtime counterpart: `apply --palette <dir>` reads
an palette theme directory, validates every color as `#RRGGBB`, and writes
the same files (`--user` to `$HOME`, `--system` under a root). `--user` is
configuration management and says so: it also sets `color-scheme`, the accent
and the wallpaper with `gsettings`, and `deskos-theme reset` returns those
keys to the image default. On `--system` with a root other than `/` it only
prepares the tree; `dconf update` runs inside the environment that will use it
(the image build), never on a host against another root. Icons, cursors and
fonts are not applied: an palette `colors.toml` does not carry them and
inventing values is worse than leaving them.

## Optional adapters

Two surfaces are explicit opt ins, off the default path:

- **GNOME Shell** (`--shell-css`): replaces gnome-shell's Adwaita base hexes
  in the distro's compiled stylesheet and writes two packs
  (`deskos-a`/`deskos-b`) so the running Shell can reload by switching. A hex
  replacement does not preserve meaning (a white for text can be a border or
  an indicator) and can fail silently when upstream CSS changes; it also
  repeats the whole `enabled-extensions` list, so it can clobber other
  extensions. For the product, use the native accent; a full Shell theme
  belongs in its own track (generate from the exact packaged GNOME SCSS, with
  per-version tests).
- **GTK3** (`gtk-3.0/gtk.css`): `@define-color` from a user provider does not
  reliably reach the active theme's resolved rules. It is best effort and
  unverified until tested against the specific GTK3 theme; if it depends on
  `adw-gtk3`, that dependency must be declared and enabled.

The GNOME Shell welcome dialog (`shell.welcomeTour`) is on by default; `false`
writes `welcome-dialog-last-shown-version`, a value gnome-shell owns. The
constant is a dependency on the upstream tour change version, not on the GNOME
version: the test asserts suppression on the packaged Shell.

## Consequences

- The public API has **exactly thirteen kinds**; a test enforces the count.
- Themes are how DeskOS expresses "many looks" without repeating settings;
  an palette `colors.toml` is the reference palette shape. Promising visual
  parity with palette would be wrong: GNOME has deliberate limits and apps
  that read the accent by API, not CSS.
- `deskos-theme` is the runtime applier; the compiler bakes the install-time
  default. A booted-session check of GTK4 and Ptyxis is the next gate; the
  Shell and GTK3 stay opt in.
