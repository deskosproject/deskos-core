# What DeskOS takes from the GNOME HIG

The [GNOME Human Interface Guidelines](https://developer.gnome.org/hig/) are
written for app authors. An image ships the defaults those apps are designed
against, so a handful of its pages are policy for us, and the rest are not ours to
decide. This page says which is which.

An image that contradicts the HIG makes every app worse at once: a font the apps
were not measured with, an accent that fails contrast, a notification surface the
design assumes is off.

## Pages that are image policy

| HIG page | What the image does | Where |
|---|---|---|
| [Typography](https://developer.gnome.org/hig/guidelines/typography.html) | the default family is Adwaita Sans, "a custom built variant of Inter", with Adwaita Mono for monospace; document text is one point larger than interface text | `appearance.fonts.interface`, `.document`, `.monospace`; [ADR 0026](../adr/0026-the-families-are-the-workstations-choice.md) |
| [Accessibility](https://developer.gnome.org/hig/guidelines/accessibility.html) | text scaling, and a compose key for users who type accented text | `appearance.fonts.textScaling`, `keyboard.xkbOptions` |
| [Palette](https://developer.gnome.org/hig/reference/palette.html) and UI styling | a named colour scheme and accent, and a theme palette for the terminal | `appearance.colorScheme`, `.accentColor`, `Theme` |
| [Keyboard](https://developer.gnome.org/hig/guidelines/keyboard.html) | layouts in switch order, and the standard terminal shortcut | `keyboard.layouts`, `.xkbOptions`, `.terminal` |
| [Notifications](https://developer.gnome.org/hig/patterns/feedback/notifications.html) | whether notices appear, and whether they appear on a locked screen | `notifications.banners`, `.onLockScreen` |
| [Backgrounds](https://developer.gnome.org/hig/reference/backgrounds.html) | a light and a dark wallpaper | `appearance.wallpaper` |
| App icons and UI icons | an icon and cursor theme, when an organization ships its own | `appearance.iconTheme`, `.cursorTheme` |

## Pages that are not

Windows, header bars, popovers, buttons, menus, dialogs, view switchers, pointer
and touch, and the writing style guide describe how an application is built. A
workstation image does not restyle applications, and a field that did would be a
GTK theme with an escape hatch attached.

The HIG targets GTK 4 and libadwaita; an EL10 image also carries GTK 3
applications. The image follows the HIG where it is a *default* (family, sizes,
palette, contrast) and leaves the widgets to the toolkit, which is why the same
workstation looks coherent across both.

## Where this is checked

`docs/gnome-coverage.md` maps the GNOME system administration guide to the fields;
this page maps the design guide to them. A setting that is in neither is either a
gap somebody should record, or a decision somebody already refused.
