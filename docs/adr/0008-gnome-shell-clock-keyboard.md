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
- `shell.appFolders` is one list. When `folder-children` is non-empty,
  GNOME Shell 47 does not create its default folders
  (`_ensureDefaultFolders` in `appDisplay.js`), so declared folders replace
  them.
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
