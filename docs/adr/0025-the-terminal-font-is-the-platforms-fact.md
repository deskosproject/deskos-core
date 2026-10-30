# ADR 0025: the terminal font is the platform's fact

**Status:** accepted (2026-10-30)

## Context

GNOME has a font key for every surface it draws, and `GnomeProfile` models them:
`interface`, `document`, `monospace` and, since this decision, `titlebar` and
`textScaling`. A terminal is not one of those surfaces. It is a packaged
application with its own GSettings schema, and on the EL10 family it is Ptyxis:
`org.gnome.Ptyxis`, path `/org/gnome/Ptyxis/`, keys `font-name` and
`use-system-font`, where the second decides whether the first is read at all.

The person deploying a workstation does not think in schemas. "Set the terminal
font" is one intent, and leaving it out of the model means the one surface a
support engineer reads all day is the one the image cannot name. Three ways to
carry it were available:

1. Hardcode Ptyxis in the desktop provider.
2. A general "application settings" field: any path, any key.
3. The platform declares its terminal; the profile sets the font.

Option 2 is a raw dconf escape by another name, which the resource model refuses
(ADR 0005, invariant 5). Option 1 puts one platform's application into Core's
semantics, which invariant 1 and ADR 0004 forbid: Core compiles intent, the
platform supplies the facts.

## Decision

`Platform.spec.gnome.terminal` names the terminal the desktop ships and where its
font lives:

```yaml
terminal:
  package: ptyxis
  dconfPath: /org/gnome/Ptyxis/
  fontKey: font-name
  useSystemFontKey: use-system-font
```

`GnomeProfile.appearance.fonts.terminal` (and the same field in a `Theme`) is one
font, `{family, size}`. It lowers to two defaults: the platform's path plus
`fontKey` receives `"<family> <size>"`, and the same path plus
`useSystemFontKey` receives `false`, because a terminal that uses the system font
ignores the family it was given. The platform's `package` is added to the image,
so a font for a terminal that is not installed cannot be declared either.

A platform that does not declare a terminal cannot carry the setting:
composition fails and says which platform lacks which fact, instead of writing a
key nothing reads (invariant 9). The same rule covers `titlebar`, which GNOME
honours only while `titlebar-uses-system-font` is `false`: both keys are written
together.

## Consequences

- Core knows no application's schema. A platform that ships GNOME Terminal, or
  GNOME Console, or a terminal of its own, declares its path and keys, and the
  profile field does not change.
- The fact is reviewable where platform facts live, next to `appLauncher`,
  `iconCache` and the extension catalog, and the platform page in the docs lists
  it (ADR 0024).
- `deskos-gnome` now names the family: GNOME's default monospace is Source Code
  Pro, which EL10 does not package, so before this every monospace field and
  every terminal silently fell back to whatever matched. It names DejaVu Sans
  Mono, which the font set installs.
- The rendering keys follow the same principle at the GNOME level: GNOME 47 reads
  `font-antialiasing`, `font-hinting` and `font-rgba-order` only while
  `font-rendering` is `manual`, so declaring any of the three writes `manual`
  too. The Xsettings keys and the generated fontconfig file stay for X11
  applications and for GTK on Wayland, which asks fontconfig directly.
- A future field for another packaged application's settings should start from
  this shape: the platform names the schema path and the keys, the profile
  states the intent. It does not create a general dconf field.
