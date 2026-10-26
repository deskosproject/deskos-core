# Locks on a managed workstation

A lock makes a GNOME setting read-only for the user. It is a dconf lock in the
image's own database: the value is still a default, and no user, local or site
value can override it. It is not an agent, it does not reconcile, and it does not
run on the machine.

Core never locks (invariant 8). An organization that wants a locked setting
declares it in its own `GnomeProfile`, at the layer that owns the policy.

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

`locks` uses the same semantic names as `defaults`; `deskosctl` validates them
and renders them into the image's dconf database. Locks are a **set**: they
accumulate across the foundation, organization, role and workstation layers, so
a higher layer cannot unlock what a lower one locked. Change the layer that
declares the lock.

## How it works

Two different things, often confused:

1. **Composition, at build time.** A `GnomeProfile` at any layer declares a
   value under `defaults` and, separately, a setting under `locks`. The value
   composes as usual: the highest layer wins. The lock is a set: it accumulates,
   and a higher layer cannot remove it. A lock does **not** freeze the
   composition: if a workstation above the organization declares another
   wallpaper, the image ships the workstation's wallpaper — still locked. The
   lock applies to the dconf key, not to the value or the layer. If an
   organization needs one fixed value, it must own the profiles and workstations;
   the model has no composition lock.
   A lock needs an effective value: if no included profile sets the setting,
   composition fails with “GNOME lock … has no effective value to lock”.
2. **Runtime, in GNOME.** The backend writes the lock into the image's dconf
   database. At login, dconf reads it; GNOME Settings and `gsettings set` answer
   that the key is not writable, and a user, local or site value cannot override
   it. There is no agent and no policy service: the image carries the lock.

### Wallpaper, concretely

`appearance.wallpaper` lowers to three keys — background light, background dark
and screensaver — so a lock on the setting locks all three. Declare a wallpaper
first (usually inherited from `deskos-look`), then lock it:

```yaml
spec:
  defaults:
    appearance:
      wallpaper:
        light: ../assets/org-wallpaper.svg
  locks:
    - appearance.wallpaper
```

The image ships that wallpaper locked: the Settings chooser and `gsettings`
cannot change it. If a workstation includes this profile and sets another
wallpaper, the image ships the other one, also locked; the lock is not a veto on
composition. `appearance.loginLogo` is a different setting and is not covered by
this lock; the login screen has its own profile.

## What can be locked

The model knows these settings, so these are the names a lock accepts:

- **Windows:** `windows.buttons`, `windows.buttonPlacement`.
- **Appearance:** `appearance.wallpaper`, `appearance.loginLogo`,
  `appearance.fonts.interface`, `appearance.fonts.document`,
  `appearance.fonts.monospace`, `appearance.iconTheme`,
  `appearance.cursorTheme`, `appearance.colorScheme`,
  `appearance.accentColor`.
- **Shell:** `shell.favorites`, `shell.hotCorners`, `shell.welcomeTour`,
  `shell.workspaces`, `shell.appFolders`, `shell.extensions`.
- **Session:** `session.idle.blankAfter`, `session.lock.enabled`,
  `session.lock.delay`.
- **Dock:** `dock.position`, `dock.behavior`, `dock.iconSize`,
  `dock.showTrash`.
- **Software:** `software.updates`.
- **Clock:** `clock.showWeekday`, `clock.format`.
- **Keyboard:** `keyboard.numLock`, `keyboard.terminal`,
  `keyboard.xkbOptions`.
- **Power:** `power.buttonAction`.

`applications.*` and `appearance.palette` are not lockable: the first is a
default GNOME resolves, the second is generated into the theme.

## What fits a managed laptop

- **Branding** — wallpaper, login logo, fonts, icons, colour scheme, accent:
  a fleet looks the same and a user cannot repaint it.
- **A lab or kiosk** — favorites, enabled extensions, app folders, hot corners,
  workspaces and the welcome tour: the session is the same for everyone.
- **A standard environment** — keyboard layout options, the terminal shortcut,
  Num Lock, clock format.
- **Updates** — `software.updates` is GNOME Software's own switch. It does not
  control the image update (`bootc`) or the Flatpak policy; those have their own
  resources and units.
- **Session** — locking `session.lock.enabled` to `true` and a short
  `session.lock.delay` enforces the screen lock. **Never lock it to `false`.**
- **Power** — `power.buttonAction` if the fleet standardizes what the button does.

Start with a handful. A lock is easy to add and hard to justify later.

## What a lock breaks

- **Personalization.** A locked wallpaper, font or accent is a decision the user
  cannot revisit; say why in the organization's documentation.
- **Extension troubleshooting.** A locked `shell.extensions` list means a user
  cannot disable an extension that misbehaves. Keep the list short and tested.
- **Favorites and workspaces.** Users cannot add a favorite or change the number
  of workspaces; a lab may want this, a developer probably does not.
- **GNOME Software updates.** `software.updates=false` stops the application
  store's updates; it does not stop the image or the Flatpak policy. Do not use
  it as a substitute for either.
- **The session lock.** Locking `session.lock.enabled=false` disables the lock
  for every user. Locking a very short delay with a long idle timeout surprises
  people; test it on a real machine.

## Layers and the running session

- Locks union across layers; a lock at the foundation or organization layer
  survives a role or workstation that does not mention it.
- A `local` or `site` dconf value loses to a lock. That is the point: a lock is
  image-level enforcement, not a default a user may override.
- An application that ignores dconf (Firefox policies, Electron settings,
  LibreOffice options) is not covered by a lock.

## How to verify

- `deskos report` reads the plan the image carries and reports the locks it
  declares; `deskos check` says in its scope that it does not measure whether
  the session enforces them.
- The instrumented session check is the proof: it writes every locked key and
  expects the write to fail and the value to stay the same.
- Before a fleet, run that check on the example and then on one real machine.
  Try the setting by hand: `gsettings set` should answer that the key is not
  writable.

## Scope

A lock covers GNOME settings in the image's dconf database. It does not cover
browser policies, Flatpak permissions (use Flatseal or an override), CLI tools,
systemd units or the kernel; each has its own mechanism, and none of them is a
reason to add a lock that only looks like enforcement.
