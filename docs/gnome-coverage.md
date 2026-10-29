# GNOME coverage

What of the [GNOME System Administration
Guide](https://help.gnome.org/system-admin-guide/) a DeskOS image can carry, page
by page. Audited 2026-10-29 against `GnomeProfile` and the providers, not against
the guide's own assumption that any dconf key is a setting.

The vocabulary is **closed** (`settings.additionalProperties: false`): a key is
reachable only when a semantic field lowers to it. There is no raw-dconf escape,
by design (ADR 0005, invariant 5), so "GNOME lets you set it" never implies
"DeskOS can".

| Guide page | Mechanism | Verdict |
|---|---|---|
| **Setup** | | |
| Browse GSettings, dconf, profiles, keyfiles | dconf profile and keyfiles | **modelled**: the lowerer writes `distro.d` keyfiles and Platforms declare the database |
| Custom default values | dconf defaults | **modelled**, with provenance and layering |
| Lock down specific settings | dconf locks | **modelled**; Core ships none (invariant 8) |
| Store settings over NFS | `service-db:keyfile/user` first line of the profile | **gap: field** — the platform owns the profile, and no field asks for it |
| Why not GSettings overrides | vendor overrides | **modelled** for the first-boot wallpaper only; no general field |
| **User settings** | | |
| Dim screen when idle | `plugins.power idle-dim`, `idle-brightness`, `session idle-delay` | **partial**: idle delay is modelled; dim and brightness are **gap: two fields** |
| Enable the compose key | `input-sources xkb-options` | **modelled** (`keyboard.xkbOptions`: `compose:caps`, `compose:menu`, `compose:ralt`, `compose:rctrl`) |
| Disallow online accounts | `online-accounts whitelisted-providers` | **gap: field**, or declined: an account is a person's, not the image's |
| Single-application mode | custom session plus a wrapper `Exec` | **refused**: arbitrary shell (invariant 5) |
| Disable command line | `lockdown disable-command-line` | **modelled** |
| Disable file saving | `lockdown disable-save-to-disk` | **modelled** |
| Disable printing | `lockdown disable-printing` | **modelled** |
| Disable repartitioning | a polkit `.policy` file for udisks2 | **gap: decision** — a file with security semantics, not a key; a kind here is a new surface |
| Disable logout and switching | `lockdown disable-log-out`, `disable-user-switching` | **modelled** |
| Disallow fingerprint login | PAM, fprintd and a reader | **outside**: hardware and PAM, not image intent |
| Lock down extensions | `shell enabled-extensions`, `development-tools` | **partial**: the list is modelled and lockable; Looking Glass is **gap: field** |
| **Appearance** | | |
| Extra fonts (all users) | files under `/usr/share/fonts` | **artifact**: `BinaryArtifact` or a package; no field |
| Extra fonts (one user) | `$HOME/.local/share/fonts` | **outside** (`$HOME`) |
| Extra backgrounds | files plus the wallpaper key | **artifact** plus the modelled wallpaper key |
| Lock screen shield | `screensaver picture-uri` | **partial**: the shield is set to the wallpaper; a distinct shield is **gap: field** |
| Lock the screen when idle | `screensaver lock-enabled`, `lock-delay` | **modelled** |
| Custom background | `background picture-uri`, `-dark` | **modelled** (`appearance.wallpaper.light`, `.dark`) |
| Default favorite applications | `shell favorite-apps` | **modelled**, order-sensitive and checked |
| **Login** | | |
| Greeter logo | `login-screen logo` | **modelled** (`appearance.loginLogo`) |
| Disable the user list | `login-screen disable-user-list` | **modelled** since 2026-10-29 (`login.showUserList`) |
| Text banner on the greeter | `login-screen banner-message-enable`, `-text` | **modelled** since 2026-10-29 (`login.banner`), any length, never from a file |
| Default session per user | AccountsService `XSession` | **outside**: a provisioning choice for one account |
| Automatic login | `/etc/gdm/custom.conf` | **outside**: provisioning; the VM harness uses it for tests |
| Automatic logout when idle | `plugins.power sleep-inactive-ac/battery-timeout`, `-type` | **gap: fields**, and a policy that can lose unsaved work |
| Custom session | a session file with an `Exec` | **refused**: arbitrary shell |
| Multiple keyboard layouts at the greeter | `localectl set-x11-keymap`, XKB layouts | **gap: field** — options are modelled, layouts are not |
| Enterprise credentials | FreeIPA or AD join | **Milestone 5**: identity, never image content |
| **Network and hardware** | | |
| Default server list | `$HOME/.config/gtk-3.0/servers` (XBEL) | **outside** (`$HOME`) |
| Default VPN settings | NetworkManager `system-connections` | **gap: kind** — a connection profile, with secrets supplied at runtime |
| **Software** | | |
| Enable machine-wide extensions | `shell enabled-extensions` | **modelled** (`shell.extensions`, plus the platform's catalog) |
| Custom MIME type | XML under `/usr/share/mime` plus `update-mime-database` | **artifact**: a package or a placed file, not a field |
| Override the default application | `/etc/xdg/mimeapps.list` | **modelled** (`applications.*`); a user's file still wins |
| Autostart an application | `/etc/xdg/autostart` with `Exec` | **refused**: arbitrary shell |
| Typical processes | reference | **not a field**: a candidate assertion for a check, not intent |
| **Other** | | |
| Session debugging, custom sessions | logs and session files | **outside** / **refused** |

## What the gaps cost

Three different bills, and conflating them is how a backlog grows without
shipping:

- **Seven fields are cheap**: dim and brightness, the extension lock for Looking
  Glass, a distinct lock shield, idle logout, XKB layouts and the NFS profile
  line (the greeter's two left this list on 2026-10-29). Each is a scalar (or two)
  in
  `GnomeProfile`, a lowering rule, a schema entry and a check; no new kind, and
  the count stays at fourteen.
- **Two are kinds**: a NetworkManager connection (VPN, 802.1X, proxy) and a
  polkit action file. The first is needed by real organizations and needs an ADR
  for secrets at runtime; the second is a security surface and may be refused.
- **Three are refused on purpose**: single-application mode, custom sessions and
  autostart. All three are an `Exec` in a directory; invariant 5 is the reason,
  and it does not bend for convenience.

## Decisions this produces

1. Add the remaining seven fields, one PR per group, each with its check; the
   greeter's two shipped on 2026-10-29.
2. Open the NetworkManager connection ADR if an organization asks; refuse polkit
   unless one shows a real need.
3. Keep the refusals written down here rather than rediscovered in review.
4. Correct the roadmap: it claimed dconf defaults and locks covered more than
   expected. They covered **fourteen** guide items exactly and four partially at the
   audit, which is a different claim; the greeter's two now bring it to sixteen.
