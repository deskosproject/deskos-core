# Desktop resources

Part of the [resource model](../resources.md): the kinds that shape the GNOME session and the theme.

## `GnomeProfile`

**GNOME intent** in administrator vocabulary. An organization layer
overrides the Core defaults it names:

```yaml
apiVersion: desktop.deskos.org/v1alpha1
kind: GnomeProfile
metadata:
  name: example-desktop
spec:
  defaults:
    windows:
      buttons: [close]
    applications:
      browser: google-chrome.desktop
      fileManager: org.gnome.Nautilus.desktop
      textEditor: org.gnome.TextEditor.desktop
      imageViewer: org.gnome.Loupe.desktop
    appearance:
      wallpaper:
        light: ../assets/example.svg
      loginLogo: ../assets/example-login-logo.svg
      colorScheme: prefer-dark   # default | prefer-dark | prefer-light
      accentColor: teal          # blue, teal, green, yellow, orange, red, pink, purple, slate
    software:
      updates: manual            # automatic | manual | disabled
    session:
      idle:
        blankAfter: 5m
      lock:
        enabled: true
        delay: 0s
    shell:
      extensions:
        - {uuid: dash-to-dock@micxgx.gmail.com, enabled: true}
    dock:
      position: bottom
      behavior: intellihide
      iconSize: 40
      showTrash: false
```

Shell, clock and keyboard settings use the same mechanism:

```yaml
spec:
  defaults:
    shell:
      hotCorners: false
      welcomeTour: true            # the first-run welcome dialog (tour)
      workspaces: 4                # dynamic | 1 to 36
      appFolders:
        - id: Development
          name: Development
          apps: [code.desktop]
          categories: [Development]
      extensions:
        - uuid: appindicatorsupport@rgcjonas.gmail.com
          enabled: true            # or false; leaving the uuid out inherits the platform
    clock:
      showWeekday: true
      format: 24h                  # 24h | 12h
    keyboard:
      numLock: true
      terminal: com.mitchellh.ghostty.desktop   # Ctrl+Alt+T
      xkbOptions: [compose:caps]                  # Caps Lock as Compose key
    power:
      buttonAction: hibernate      # nothing | suspend | hibernate | interactive
```

- **`applications.*`** names the desktop entry that handles a document role:
  `browser`, `fileManager`, `textEditor`, `imageViewer`, `videoPlayer` or
  `pdfViewer`. Core owns the role-to-MIME table and writes
  `/etc/xdg/mimeapps.list`; the profile never authors MIME types. A user's
  `~/.config/mimeapps.list` still wins, so this is a default, not a lock.
- **`appFolders`** replaces GNOME's built-in folders (such as
  *Utilities*); declare every folder the app grid should have. A higher
  layer replaces the whole list. Not available on RHEL 10.2, whose GNOME
  Shell 49.4 overwrites them at first login.
- **`extensions`** decides, one UUID at a time, whether a GNOME Shell extension is
  enabled. A profile never names a package, a schema or a dconf key: it says which
  extension and whether it runs, and the rest is not its business. Any UUID may be
  decided, the dock included: when the platform's own catalog declares the
  extension, DeskOS installs the package it names — or, on a platform whose
  repositories do not carry a usable one, the pinned artifact the catalog names
  ([ADR 0021](../adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md)); an
  extension the composition brings itself is installed by the packages the
  composition declares, and a UUID nobody provides shows up as a failed session
  check rather than as silence. The plan always states the list it enables
  ([ADR 0017](../adr/0017-extensions-flatpak-remote-and-power-button.md)).
- **`dock.*`** is the settings DeskOS models for Dash to Dock, and they apply only
  while its extension is enabled through `shell.extensions`. Configuring an
  extension DeskOS does not model is deliberately not possible from the artifact:
  the desktop keeps its own defaults and the user changes them in the session
  ([ADR 0005](../adr/0005-managed-baseline-boundary.md)).
- **`power.buttonAction`** is the hardware power button's action, from GNOME's own
  values.
- **`keyboard.terminal`** binds Ctrl+Alt+T to the given desktop file
  through the Platform's application launcher (`gtk-launch` on EL10), and
  writes the same entry to `/etc/xdg/xdg-terminals.list` so other
  `xdg-terminal-exec` callers pick it too.
- **`keyboard.xkbOptions`** accepts a small reviewed list of XKB options, not
  raw keymap text. `compose:caps` turns Caps Lock into the Compose key; users
  type Shift for capitals and use sequences such as Compose, `'`, `e` for `é`.
- **`welcomeTour`** (default `true`) controls GNOME Shell's first-run
  welcome dialog, which offers the desktop tour. `false` suppresses it.

## `Theme`

A **named, reusable look** — a palette mapped onto the surfaces GNOME
itself defines. A Theme maps a palette onto the same GNOME
setting domain as `GnomeProfile`, so it layers, locks and conflicts like
any other appearance setting:

```yaml
apiVersion: desktop.deskos.org/v1alpha1
kind: Theme
metadata:
  name: tokyo-night
spec:
  mode: dark                    # dark | light
  accent: "#7aa2f7"             # any #RRGGBB; mapped to the closest GNOME accent
  palette:                      # free colors for the GTK and terminal overrides
    background: "#1a1b26"
    foreground: "#a9b1d6"
    red: "#f7768e"              # the six normal ANSI colors also produce
    green: "#9ece6a"            # a Ptyxis terminal palette
    yellow: "#e0af68"
    blue: "#7aa2f7"
    magenta: "#ad8ee6"
    cyan: "#449dab"
  iconTheme: Yaru-blue
  cursorTheme: Adwaita
  fonts:
    monospace: { family: JetBrains Mono, size: 11 }
  wallpaper:
    light: ../assets/tokyo-night-light.svg
    dark: ../assets/tokyo-night-dark.svg
```

- **`mode`** lowers to `appearance.colorScheme` (`dark` → `prefer-dark`).
- **`accent`** is any hex; it is mapped to the closest of GNOME's nine
  accents **by hue** (a light green stays green), with a `slate` fallback
  for near-achromatic colors.
- **`palette`** makes the Theme generate named-color overrides for
  **GTK4/libadwaita and GTK3** (`/etc/skel/.config/gtk-{4,3}.0/gtk.css`), and,
  when the six normal ANSI colors are present, a **Ptyxis terminal palette**
  (`/etc/skel/.local/share/org.gnome.Ptyxis/palettes/`). Users the installer
  or GNOME Initial Setup create inherit them.
- **`wallpaper`**, **`fonts`**, **`iconTheme`** and **`cursorTheme`** are
  the same shapes and lowering as in `GnomeProfile.appearance`.

A Theme is included by a `Profile` like any other resource. A native GTK3
theme, terminals and a GNOME Shell recoloring are still out: the Shell needs
the `user-theme` extension and its own stylesheet; see
[ADR 0013](../adr/0013-theme.md).
