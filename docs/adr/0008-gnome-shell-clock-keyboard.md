# ADR 0008: GnomeProfile shell, clock and keyboard settings

**Status:** accepted (2026-10-04)

## Context

Organizations want to set the hot corners, the workspace count, app grid
folders, the clock and a terminal shortcut, as Bluefin does with GSettings
overrides and dconf files. Without semantic fields they would need raw
dconf keys, which `GnomeProfile` does not accept.

## Decision

`GnomeProfile.spec.defaults` gains `shell.hotCorners`, `shell.workspaces`,
`shell.appFolders`, `clock.showWeekday`, `clock.format`, `keyboard.numLock`
and `keyboard.terminal`. Each is one scalar setting with the usual layer
precedence, conflict detection and `locks`.

- `shell.workspaces` is `dynamic` or a fixed count; a count sets
  `dynamic-workspaces` false and `num-workspaces`.
- `shell.appFolders` is one list. GNOME Shell 49.5 does not create its
  default folders when `folder-children` is non-empty
  (`_ensureDefaultFolders` in `appDisplay.js`), so declared folders replace
  them. GNOME Shell 49.4 checks only the user's own value and writes its
  defaults over the administrator's at first login (fixed upstream in
  4b1d80383c, released in 49.5). The Platform fact `gnome.appFolders`
  says whether the shell keeps them; RHEL 10.2 ships 49.4, so its
  Platform rejects the setting.
- `keyboard.terminal` names a desktop file. The binding is fixed at
  Ctrl+Alt+T and the command is the Platform's `gnome.appLauncher`
  followed by the desktop file id; the launcher's package is installed.
  A Platform without a launcher rejects the setting.

Core sets none of them.

## Consequences

- No new kind; `GnomeProfile` stays the only GNOME resource.
- A custom shortcut is stored at
  `/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/deskos-terminal/`.
  Users' own shortcuts are added to the same list by GNOME Settings.
- The desktop file is not checked at build time: it may come from a
  Flatpak installed at first boot.
