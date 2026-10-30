# Locks on a managed workstation

A lock makes a GNOME setting read-only for the user. It is a dconf lock in the
image's own database: the value is still a default, and no user, local or site
value overrides it. It is not an agent and does not run on the machine. Core never
locks (invariant 8); an organization declares a lock in its own `GnomeProfile`, at
the layer that owns the policy.

## Declare one

```yaml
apiVersion: desktop.deskos.org/v1alpha1
kind: GnomeProfile
metadata:
  name: example-managed-desktop
spec:
  locks:
    - shell.favorites
    - appearance.wallpaper
```

`locks` uses the same semantic names as `defaults`; `deskosctl` validates them and
renders them into the image's dconf database. Locks accumulate across the
foundation, organization, role and workstation layers, so a higher layer cannot
unlock what a lower one locked. Change the layer that declares it.

## How it works

**Composition, at build time.** The value under `defaults` composes as usual, and
the lock does not freeze it: a workstation above the organization that declares
another wallpaper ships its own, still locked. A lock needs an effective value, or
composition fails with “GNOME lock … has no effective value to lock”.

**Runtime, in GNOME.** The backend writes the lock into the image's dconf database,
so at login GNOME Settings and `gsettings set` answer that the key is not writable
([runtime.md](runtime.md#gnome)).

### Wallpaper, concretely

`appearance.wallpaper` lowers to three keys (background light, background dark and
screensaver), so a lock on the setting locks all three. Declare it first (usually
inherited from `deskos-look`), then lock it:

```yaml
spec:
  defaults:
    appearance:
      wallpaper:
        light: ../assets/org-wallpaper.svg
  locks:
    - appearance.wallpaper
```

The Settings chooser and `gsettings` cannot change it. `appearance.loginLogo` is a
different setting, not covered by this lock.

## What can be locked

- **Windows:** `windows.buttons`, `windows.buttonPlacement`.
- **Appearance:** `appearance.wallpaper`, `.loginLogo`, `.lockShield`,
  `.fonts.interface`, `.fonts.document`, `.fonts.monospace`, `.fonts.titlebar`,
  `.fonts.terminal`, `.fonts.textScaling`, `.fonts.antialiasing`, `.fonts.hinting`,
  `.fonts.rgbaOrder`, `.fonts.lcdfilter`, `.iconTheme`, `.cursorTheme`,
  `.colorScheme`, `.accentColor`.
- **Shell:** `shell.favorites`, `shell.hotCorners`, `shell.welcomeTour`,
  `shell.workspaces`, `shell.appFolders`, `shell.extensions`.
- **Session, dock:** `session.idle.blankAfter`, `session.lock.enabled`,
  `session.lock.delay`, `dock.position`, `dock.behavior`, `dock.iconSize`,
  `dock.showTrash`.
- **Software:** `software.updates`.
- **Network, privacy:** `network.proxy.mode`, `.autoconfigUrl`, `.ignoreHosts`,
  `.useSameProxy`, `.http`, `.https`, `.ftp`, `.socks`, `privacy.rememberRecentFiles`,
  `privacy.recentFilesMaxAge`, `privacy.rememberAppUsage`, `privacy.removeOldTempFiles`,
  `privacy.removeOldTrashFiles`, `privacy.oldFilesAge`, `privacy.reportTechnicalProblems`,
  `privacy.sendSoftwareUsageStats`, `privacy.disableCamera`, `privacy.disableMicrophone`,
  `privacy.disableSoundOutput`, `privacy.hideIdentity`, `privacy.privacyScreen`.
- **Clock, keyboard, power:** `clock.showWeekday`, `clock.format`,
  `keyboard.numLock`, `keyboard.terminal`, `keyboard.xkbOptions`,
  `power.buttonAction`.

`applications.*` and `appearance.palette` are not lockable: the first is a default
GNOME resolves, the second is generated into the theme.

## What fits a managed laptop

- **Branding:** wallpaper, login logo, fonts, icons, colour scheme, accent.
- **A lab or kiosk:** favorites, extensions, app folders, hot corners, workspaces,
  welcome tour.
- **A standard environment:** XKB options, the terminal shortcut, Num Lock, clock
  format, and `power.buttonAction` if the fleet standardizes it.
- **Session:** `session.lock.enabled` true and a short `session.lock.delay`; never
  lock it to `false`.

## What a lock breaks

- **Personalization.** A locked wallpaper, font or accent is a decision the user
  cannot revisit; say why in the organization's documentation.
- **Extensions and favorites.** Users cannot disable a misbehaving extension, add a
  favorite or change the number of workspaces; keep the list short and tested.
- **GNOME Software updates.** `software.updates` is GNOME Software's switch; it does
  not stop the image update (`bootc`) or the Flatpak policy.
- **The session lock.** Locking `session.lock.enabled` false disables the lock for
  every user; a short delay with a long idle timeout surprises people.

## Layers and the running session

- Locks union across layers: one at the foundation or organization layer survives a
  role or workstation that does not mention it.
- A `local` or `site` dconf value loses to a lock; an application that ignores dconf
  (Firefox policies, Electron, LibreOffice options) is not covered.

## How to verify

`deskos report` lists the locks the plan declares. `deskos check` says in its scope
that it does not measure whether the session enforces them; the instrumented
session check writes every locked key and expects the write to fail. Run it before
a fleet.

## Scope

A lock covers GNOME settings in the image's dconf database, not browser policies,
Flatpak permissions (use Flatseal or an override), CLI tools, systemd units or the
kernel; each has its own mechanism.
