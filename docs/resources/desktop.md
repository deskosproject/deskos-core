# Desktop resources

Part of the [resource model](../resources.md): the kinds that shape the GNOME session and the theme.

## `GnomeProfile`

**GNOME intent** in administrator vocabulary. An organization layer overrides
the Core defaults it names. The vocabulary is closed on purpose: what a guide
item needs when no field exists is in [GNOME coverage](../gnome-coverage.md).

| Field | Values | Default |
|---|---|---|
| `windows.buttons`, `windows.buttonPlacement` | `minimize`, `maximize`, `close`, at least one; `left` or `right` | |
| `applications.browser`, `.fileManager`, `.textEditor`, `.imageViewer`, `.videoPlayer`, `.pdfViewer` | a `*.desktop` id | |
| `appearance.wallpaper.light` (required), `.dark`, `appearance.loginLogo`, `appearance.iconTheme`, `appearance.cursorTheme` | assets inside the resource root; theme names | |
| `appearance.fonts.interface`, `.document`, `.monospace`; `appearance.colorScheme`, `.accentColor` | `{family, size}`, size 6 to 72; `default`, `prefer-dark`, `prefer-light`; `blue`, `teal`, `green`, `yellow`, `orange`, `red`, `pink`, `purple`, `slate` | |
| `shell.favorites`, `.hotCorners`, `.welcomeTour`, `.workspaces`, `.appFolders[]`, `.extensions[]` | ordered `*.desktop` ids; boolean; boolean (absent keeps the tour, `false` suppresses it); `dynamic` or 1 to 36; `{id, name, apps}` or `{id, name, categories}`; `{uuid, enabled}` | |
| `clock.showWeekday`, `.format`, `keyboard.numLock`, `.terminal`, `.xkbOptions`, `power.buttonAction` | boolean; `24h` or `12h`; boolean; a `*.desktop` id bound to Ctrl+Alt+T; `compose:caps` and the other reviewed XKB options; `nothing`, `suspend`, `hibernate`, `interactive` | |
| `session.idle.blankAfter`, `session.lock.enabled`, `.delay`, `dock.position`, `.behavior`, `.iconSize`, `.showTrash`, `software.updates`, `locks` | `never` or a duration (`5m`); boolean; a duration (`0s`); `bottom`, `left`, `right`, `top`; `fixed`, `autohide`, `intellihide`; 16 to 64; boolean; `automatic`, `manual`, `disabled`; semantic names from `defaults` ([locks.md](../locks.md)) | |

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
    appearance:
      wallpaper: {light: ../assets/example.svg}
      colorScheme: prefer-dark   # default | prefer-dark | prefer-light
      accentColor: teal          # blue, teal, green, yellow, orange, red, pink, purple, slate
    software:
      updates: manual            # automatic | manual | disabled
    session:
      idle: {blankAfter: 5m}
    shell:
      extensions:
        - {uuid: dash-to-dock@micxgx.gmail.com, enabled: true}
    dock:
      position: bottom
```

- **`applications.*`** names the desktop entry for a document role; Core owns the
  role-to-MIME table and writes `/etc/xdg/mimeapps.list`, and a user's
  `~/.config/mimeapps.list` still wins: a default, not a lock.
- **`appFolders`** replaces GNOME's built-in folders: declare every folder the app
  grid should have, and a higher layer replaces the list. Not available on RHEL
  10.2, whose GNOME Shell 49.4 overwrites them at first login.
- **`extensions`** decides one UUID at a time, the dock included; the install uses
  the package or pinned artifact the platform's catalog names
  ([ADR 0021](../adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md),
  [ADR 0017](../adr/0017-extensions-flatpak-remote-and-power-button.md)), and a
  UUID nobody provides fails a session check rather than passing silently.
- **`dock.*`** is what DeskOS models for Dash to Dock, applied only while its
  extension is enabled through `shell.extensions`
  ([ADR 0005](../adr/0005-managed-baseline-boundary.md)).

### Locks

A profile makes a setting read-only with `spec.locks`, using the same semantic
names as `defaults`. A lock is a dconf lock in the image's database, not a runtime
agent: it needs an effective value and stops the running user from changing the
key, but not a higher layer from choosing another composed value. Core never locks
(invariant 8); see [locks.md](../locks.md).

### Lockdown

GNOME has a set of capabilities a managed profile can turn off; each is a boolean
under `defaults.lockdown`, and a lock makes the chosen value read-only.

| Field | GNOME key | Effect |
|---|---|---|
| `disablePrinting`, `disablePrintSetup`, `disableSaveToDisk`, `disableApplicationHandlers`, `disableCommandLine`, `disableShowPassword` | `disable-printing`, `disable-print-setup`, `disable-save-to-disk`, `disable-application-handlers`, `disable-command-line`, `disable-show-password` | the printing UI, Save and Save As, handler launches, the command line and password reveal are disabled |
| `disableLogOut`, `disableUserSwitching`, `disableLockScreen` | `disable-log-out`, `disable-user-switching`, `disable-lock-screen` | the user cannot log out, switch users or lock the screen |
| `mountRemovableStorageReadOnly`, `disableUserAdministration` | `mount-removable-storage-devices-as-read-only`, `user-administration-disabled` | removable storage mounts read-only; user administration is disabled |

Set the boolean, then list `lockdown.<field>` under `locks`. `disableLockScreen`
weakens security: use it only when another mechanism locks the session. Online
accounts and repartitioning are not here: one is per-provider configuration, the
other a polkit/udisks policy.

## `Theme`

A **named, reusable look**: a palette on GNOME's own surfaces, in the same setting
domain as `GnomeProfile`, so it layers and locks like any appearance setting:

| Field | Values | Required |
|---|---|---|
| `mode`, `accent`, `palette.*`, `iconTheme`, `cursorTheme`, `fonts.*`, `wallpaper.light`/`.dark` | `dark` or `light`; any `#RRGGBB`, mapped to the closest GNOME accent; `background`, `foreground`, `black`, `white` and the six ANSI `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`; theme names; `{family, size}`; assets (`light` required); shapes as `GnomeProfile.appearance` | `palette` needs at least one |

```yaml
spec:
  mode: dark                    # dark | light
  accent: "#7aa2f7"             # any #RRGGBB; mapped to the closest GNOME accent
  palette:                      # free colors for the GTK and terminal overrides
    background: "#1a1b26"
    foreground: "#a9b1d6"
    red: "#f7768e"              # the six normal ANSI colors also produce
    green: "#9ece6a"            # a Ptyxis terminal palette
  iconTheme: Yaru-blue
  fonts: {monospace: {family: JetBrains Mono, size: 11}}
  wallpaper: {light: ../assets/tokyo-night-light.svg, dark: ../assets/tokyo-night-dark.svg}
```

- **`mode`** lowers to `appearance.colorScheme` (`dark` → `prefer-dark`);
  **`accent`** is mapped **by hue** (a light green stays green), with a `slate`
  fallback for near-achromatic colors.
- **`palette`** generates named-color overrides for **GTK4/libadwaita and GTK3**
  (`/etc/skel/.config/gtk-{4,3}.0/gtk.css`) and, with the six normal ANSI colors,
  a **Ptyxis terminal palette**
  (`/etc/skel/.local/share/org.gnome.Ptyxis/palettes/`), which users the installer
  or GNOME Initial Setup create inherit.
- **`wallpaper`**, **`fonts`**, **`iconTheme`** and **`cursorTheme`** have the
  same shapes and lowering as in `GnomeProfile.appearance`.

A native GTK3 theme, terminals and a GNOME Shell recoloring are still out; see
[ADR 0013](../adr/0013-theme.md).
