# Desktop resources

Part of the [resource model](../resources.md): the kinds that shape the GNOME session and the theme.

## `GnomeProfile`

**GNOME intent** in administrator vocabulary. An organization layer overrides the
Core defaults it names. The vocabulary is closed: what a guide item needs when no
field exists is in [GNOME coverage](../gnome-coverage.md).

| Field | Values | Default |
|---|---|---|
| `windows.buttons`, `windows.buttonPlacement` | `minimize`, `maximize`, `close`, at least one; `left` or `right` | |
| `applications.browser`, `.fileManager`, `.textEditor`, `.imageViewer`, `.videoPlayer`, `.pdfViewer` | a `*.desktop` id | |
| `appearance.wallpaper.light` (required), `.dark`, `appearance.loginLogo`, `.lockShield`, `appearance.iconTheme`, `appearance.cursorTheme` | assets inside the resource root; theme names | |
| `appearance.fonts.interface`, `.document`, `.monospace`, `.titlebar`, `.terminal`; `appearance.colorScheme`, `.accentColor` | `{family, size}`, size 6 to 72; `default`, `prefer-dark`, `prefer-light`; `blue`, `teal`, `green`, `yellow`, `orange`, `red`, `pink`, `purple`, `slate` | |
| `appearance.fonts.textScaling` | number, 0.5 to 3.0 | |
| `appearance.fonts.antialiasing`, `.hinting`, `.rgbaOrder`, `.lcdfilter` | `none`, `grayscale`, `rgba`; `none`, `slight`, `medium`, `full`; `rgb`, `bgr`, `vrgb`, `vbgr`; `none`, `default`, `light`, `legacy` | |
| `shell.favorites`, `.hotCorners`, `.welcomeTour`, `.workspaces`, `.appFolders[]`, `.extensions[]`, `.developmentTools` | ordered `*.desktop` ids; boolean; boolean (absent keeps the tour, `false` suppresses it); `dynamic` or 1 to 36; `{id, name, apps}` or `{id, name, categories}`; `{uuid, enabled}`; boolean (`false` hides Looking Glass) | |
| `network.proxy.mode`, `.autoconfigUrl`, `.ignoreHosts`, `.useSameProxy`, `.http`, `.https`, `.ftp`, `.socks` | `none`, `auto`, `manual`; http or https URL; host patterns; boolean; `{host, port}` | `mode` yes; a password is never authored |
| `privacy.*` | booleans, plus `recentFilesMaxAge` (days, -1 keeps every recent file) and `oldFilesAge` (1 to 3650 days) | |
| `clock.showWeekday`, `.format`, `keyboard.numLock`, `.terminal`, `.layouts`, `.xkbOptions`, `power.buttonAction` | boolean; `24h` or `12h`; boolean; a `*.desktop` id bound to Ctrl+Alt+T; `[{layout, variant?}]` in switch order, the first one the layout a session starts in; `compose:caps` and the other reviewed XKB options; `nothing`, `suspend`, `hibernate`, `interactive` | |
| `login.showUserList`, `login.banner` | boolean; text of any length, never read from a file | |
| `session.idle.blankAfter`, `.action`, `.after`, `.batteryAction`, `.batteryAfter`, `.dim`, `.dimBrightness`, `session.lock.enabled`, `.delay`, `dock.position`, `.behavior`, `.iconSize`, `.showTrash`, `software.updates`, `locks` | `never` or a duration (`5m`); boolean; 1 to 100, a percentage of the level; boolean; a duration (`0s`); `bottom`, `left`, `right`, `top`; `fixed`, `autohide`, `intellihide`; 16 to 64; boolean; `automatic`, `manual`, `disabled`; semantic names from `defaults` ([locks.md](../locks.md)) | |

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

- **`applications.*`** names the desktop entry for a document role. Core owns the
  role-to-MIME table and writes `/etc/xdg/mimeapps.list`; a user's
  `~/.config/mimeapps.list` still wins, so it is a default, not a lock.
- **`appFolders`** replaces GNOME's built-in folders: declare every folder the app
  grid should have, and a higher layer replaces the list. Not available on RHEL
  10.2, whose GNOME Shell 49.4 overwrites them at first login.
- **`extensions`** decides one UUID at a time, the dock included. The install uses
  the package or pinned artifact the platform's catalog names
  ([ADR 0021](../adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md),
  [ADR 0017](../adr/0017-extensions-flatpak-remote-and-power-button.md)); a UUID
  nobody provides fails a session check rather than passing silently.
- **`dock.*`** models Dash to Dock; it applies only while `shell.extensions`
  enables that extension ([ADR 0005](../adr/0005-managed-baseline-boundary.md)).

### Locks

`spec.locks` makes a setting read-only, using the same names as `defaults`; it is a
dconf lock in the image's database, not a runtime agent. Core never locks
(invariant 8); see [locks.md](../locks.md).

### Lockdown

Each capability a managed profile can turn off is a boolean under
`defaults.lockdown`; a lock makes the chosen value read-only.

| Field | GNOME key | Effect |
|---|---|---|
| `disablePrinting`, `disablePrintSetup`, `disableSaveToDisk`, `disableApplicationHandlers`, `disableCommandLine`, `disableShowPassword` | `disable-printing`, `disable-print-setup`, `disable-save-to-disk`, `disable-application-handlers`, `disable-command-line`, `disable-show-password` | the printing UI, Save and Save As, handler launches, the command line and password reveal are disabled |
| `disableLogOut`, `disableUserSwitching`, `disableLockScreen` | `disable-log-out`, `disable-user-switching`, `disable-lock-screen` | the user cannot log out, switch users or lock the screen |
| `mountRemovableStorageReadOnly`, `disableUserAdministration` | `mount-removable-storage-devices-as-read-only`, `user-administration-disabled` | removable storage mounts read-only; user administration is disabled |

Set the boolean, then list `lockdown.<field>` under `locks`. `disableLockScreen`
weakens security: use it only when another mechanism locks the session.

**Fonts, surface by surface.** GNOME has one font key per surface, and the
profile names each one:

| Field | GNOME key | Notes |
|---|---|---|
| `interface`, `document`, `monospace` | `org.gnome.desktop.interface` `font-name`, `document-font-name`, `monospace-font-name` | |
| `titlebar` | `org.gnome.desktop.wm.preferences` `titlebar-font` | also turns `titlebar-uses-system-font` off; omitting it keeps GNOME's default, which is the interface font |
| `terminal` | the platform's terminal: `gnome.terminal` names the schema path and the two keys | requires a platform that declares a terminal, or composition fails with that reason |
| `textScaling` | `text-scaling-factor` | a multiplier over every size, not a size |

**Font rendering.** The four rendering settings write
`/etc/fonts/conf.d/60-deskos-fonts.conf`, which fontconfig applies to every
application; mirror the same choice into the interface keys GNOME 47 reads
(`font-antialiasing`, `font-hinting`, `font-rgba-order`, and `font-rendering:
manual`, without which GNOME ignores the other three); and into the Xsettings keys
X11 applications read. Declare only what you mean: an image that declares nothing
keeps the distribution's choices. RGB and BGR depend on the panel, so Core picks
neither.

The families are separate from the rendering, and they are the workstation's
choice ([ADR 0026](../adr/0026-the-families-are-the-workstations-choice.md)).
`deskos-gnome` installs the families every workstation needs: Liberation, Carlito,
Caladea, DejaVu, Noto, the URW base 35, and Gelasio and Overpass as pinned
artifacts because EL10 packages neither. `deskos-adwaita` adds GNOME's own:
Adwaita Sans for the interface and documents, Adwaita Mono for monospace, which is
what GNOME OS ships. A workstation that lists it draws GNOME's look; one that does
not keeps the platform's families, which on EL10 is Red Hat Text and Red Hat Mono.
The terminal follows the monospace family unless a profile names one.

**Idle action.** `session.idle.action` is what an idle machine does and
`.after` is when: `blank`, `suspend`, `shutdown`, `hibernate`, `interactive`,
`nothing` or `logout`, with the same pair on battery in `.batteryAction` and
`.batteryAfter`. They are declared together, because a timeout without an action
or an action without a timeout is a policy that does nothing, and `never` is only
valid with `nothing`. `blankAfter` and the dim fields are the screen, not the
machine: the screen can blank long before the machine suspends.

**Proxy.** `network.proxy` is the system proxy GNOME and the apps that read it
use: `none` ignores everything, `auto` reads the PAC file in `autoconfigUrl`, and
`manual` uses the servers. A password is not a field, because the image would
publish it; a proxy that needs authentication gets its credentials from the fleet.
GNOME uses the HTTP proxy for every protocol unless `useSameProxy` is false, so
declaring `https`, `ftp` or `socks` turns it off when the profile does not say.

**Privacy.** The `privacy` block is what the desktop remembers and reports:
recent files and app usage, when temp and trash files are removed, whether GNOME
reports technical problems or usage statistics, and whether the camera, the
microphone, the sound output, the user's name or a privacy screen are restricted.
GNOME's own defaults are already modest; an organization that answers a compliance
questionnaire can now say so in the image.

### Greeter

The user list and the login banner. Both need a platform that declares
`gnome.loginScreen`, because the keys go to the database the greeter reads; a
platform without one fails composition (invariant 9).

| Field | Effect |
|---|---|
| `login.showUserList` | `false` hides the list, so people type their name; the key is written either way |
| `login.banner` | the message above the prompt; an empty string turns it off, and leaving the field out changes nothing. GNOME folds a long message into two columns and cannot read it from a file |

## `Theme`

A **named, reusable look**: a palette on GNOME's own surfaces, in the same setting
domain as `GnomeProfile`, so it layers and locks like any appearance setting.

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
  (`/etc/skel/.config/gtk-{4,3}.0/gtk.css`) and, with the six normal ANSI colors, a
  **Ptyxis terminal palette** (`/etc/skel/.local/share/org.gnome.Ptyxis/palettes/`),
  which users the installer or GNOME Initial Setup create inherit.
- **`wallpaper`**, **`fonts`**, **`iconTheme`** and **`cursorTheme`** have the
  same shapes and lowering as in `GnomeProfile.appearance`.

A native GTK3 theme, terminals and a GNOME Shell recoloring are still out
([ADR 0013](../adr/0013-theme.md)).
