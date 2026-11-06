# GNOME coverage

What of the [GNOME System Administration
Guide](https://help.gnome.org/system-admin-guide/) a DeskOS image can carry, page
by page. Audited 2026-11-06 against `GnomeProfile` and the providers, and
cross-checked with the RHEL 10 [Administering RHEL by using the GNOME desktop
environment](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/10) guide,
not against the guide's own assumption that any dconf key is a setting.

This page is also the evidence for [ADR 0024](adr/0024-what-a-desktop-platform-must-expose.md):
a desktop is adopted because it exposes file-based configuration with precedence
and a lock layer that a check can read, not because it is popular. The audit is
how "GNOME meets that today" is a claim instead of an opinion.

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
| Store settings over NFS | `service-db:keyfile/user` first line of the profile | **gap: field**: the platform owns the profile, and no field asks for it |
| Why not GSettings overrides | vendor overrides | **modelled** for the first-boot wallpaper only; no general field |
| **User settings** | | |
| Notifications on a locked screen | `notifications show-banners`, `show-in-lock-screen` | **modelled** since 2026-10-30 (`notifications.banners`, `.onLockScreen`) |
| Privacy defaults | `org.gnome.desktop.privacy` recent files, temp/trash removal, telemetry, camera, microphone | **modelled** since 2026-10-30 (`privacy.*`) |
| Dim screen when idle | `plugins.power idle-dim`, `idle-brightness`, `session idle-delay` | **modelled** since 2026-10-29 (`session.idle.blankAfter`, `.dim`, `.dimBrightness`) |
| Enable the compose key | `input-sources xkb-options` | **modelled** (`keyboard.xkbOptions`: `compose:caps`, `compose:menu`, `compose:ralt`, `compose:rctrl`) |
| Disallow online accounts | `online-accounts whitelisted-providers` | **gap: field**, or declined: an account is a person's, not the image's |
| Single-application mode | custom session plus a wrapper `Exec` | **refused**: arbitrary shell (invariant 5) |
| Disable command line | `lockdown disable-command-line` | **modelled** |
| Disable file saving | `lockdown disable-save-to-disk` | **modelled** |
| Disable printing | `lockdown disable-printing` | **modelled** |
| Disable repartitioning | a polkit `.policy` file for udisks2 | **gap: decision**: a file with security semantics, not a key; a kind here is a new surface |
| Disable logout and switching | `lockdown disable-log-out`, `disable-user-switching` | **modelled** |
| Disallow fingerprint login | PAM, fprintd and a reader | **outside**: hardware and PAM, not image intent |
| Lock down extensions | `shell enabled-extensions`, `development-tools` | **modelled** since 2026-10-29 (`shell.extensions`, `shell.developmentTools`) |
| **Appearance** | | |
| Choose the fonts, surface by surface | `interface font-name`, `document-font-name`, `monospace-font-name`; `wm titlebar-font`; the terminal's `font-name` | **modelled** since 2026-10-30 (`appearance.fonts.interface`, `.document`, `.monospace`, `.titlebar`, `.terminal`) |
| Zoom text without changing sizes | `interface text-scaling-factor` | **modelled** (`appearance.fonts.textScaling`, 0.5 to 3.0) |
| Subpixel rendering and hinting | `interface font-rendering`, `font-antialiasing`, `font-hinting`, `font-rgba-order`, and fontconfig's own file | **modelled** (`appearance.fonts.antialiasing`, `.hinting`, `.rgbaOrder`, `.lcdfilter`): the file for every application, the keys for GNOME and X11 |
| Extra fonts (all users) | files under `/usr/share/fonts` | **artifact**: a package or a placed file; `deskos-gnome` ships Noto (Latin, symbols, math, color emoji), Red Hat Text and Display, Overpass, Gelasio, Liberation, DejaVu, Carlito, Caladea and the URW base 35, and `deskos-adwaita` adds Adwaita Sans and Mono (ADR 0026) |
| Extra fonts (one user) | `$HOME/.local/share/fonts` | **outside** (`$HOME`) |
| Extra backgrounds | files plus the wallpaper key | **artifact** plus the modelled wallpaper key |
| Lock screen shield | `screensaver picture-uri` | **not supported**: since GNOME 40 the lock screen blurs the current background; `appearance.lockShield` was removed on 2026-11-06 |
| Background rendering options and colours | `background picture-options`, `primary-color`, `secondary-color`, `color-shading-type` | **modelled** since 2026-11-06 (`appearance.wallpaper.options`, `.primaryColor`, `.secondaryColor`, `.colorShadingType`) |
| Extra backgrounds gallery | `/usr/share/gnome-background-properties/*.xml` | **artifact**: a package or a placed file; no gallery field |
| Lock the screen when idle | `screensaver lock-enabled`, `lock-delay` | **modelled** |
| Custom background | `background picture-uri`, `-dark` | **modelled** (`appearance.wallpaper.light`, `.dark`) |
| Default favorite applications | `shell favorite-apps` | **modelled**, order-sensitive and checked |
| **Login** | | |
| Greeter logo | `login-screen logo` | **modelled** (`appearance.loginLogo`) |
| Disable the user list | `login-screen disable-user-list` | **modelled** since 2026-10-29 (`login.showUserList`) |
| Text banner on the greeter | `login-screen banner-message-enable`, `-text` | **modelled** since 2026-10-29 (`login.banner`), any length, never from a file |
| Default session per user | AccountsService `XSession` | **outside**: a provisioning choice for one account |
| Automatic login | `/etc/gdm/custom.conf` | **outside**: provisioning; the VM harness uses it for tests |
| Automatic logout, suspend or shutdown when idle | `plugins.power sleep-inactive-ac/battery-timeout`, `-type` | **modelled** since 2026-10-30 (`session.idle.action`, `.after`, `.batteryAction`, `.batteryAfter`); a policy that can lose unsaved work, so the pair is explicit |
| Power button when GNOME is not running | `logind HandlePowerKey` | **modelled** since 2026-11-06 (`power.keyAction`; `/etc/systemd/logind.conf.d/50-deskos-power.conf`) |
| Lid close behaviour | `logind HandleLidSwitch` | **modelled** since 2026-11-06 (`power.lidSwitch`) |
| Custom session | a session file with an `Exec` | **refused**: arbitrary shell |
| Multiple keyboard layouts | `input-sources sources` | **modelled** since 2026-10-30 (`keyboard.layouts`, in switch order; the key lands in the distro database, which GDM's profile also reads) |
| Console keymap | `localectl`, `/etc/vconsole.conf` | **outside**: the graphical session is Wayland and reads `input-sources`, and the TTY keymap is set by the installer, which is provisioning (architecture.md, "What each layer owns") |
| Enterprise credentials | FreeIPA or AD join | **Milestone 5**: identity, never image content |
| Classification banner and heads-up display | an extension plus `org.gnome.shell.extensions.*` settings | **gap: decision**: the extension can be enabled, its message and colours cannot; a typed extension-settings surface or a reviewed dconf overlay |
| GDM switchable authentication | `authselect with-switchable-auth`, SSSD `pam_json_services`, `login-screen enable-switchable-authentication` | **outside**: identity and provisioning; the image carries no realm |
| **Network and hardware** | | |
| System proxy | `org.gnome.system.proxy` mode, PAC, ignore hosts, per-protocol servers | **modelled** since 2026-10-30 (`network.proxy`); a password is refused as a secret |
| Default server list | `$HOME/.config/gtk-3.0/servers` (XBEL) | **outside** (`$HOME`) |
| 802.1X networks | NetworkManager `system-connections` | **modelled** since 2026-10-30 (`NetworkProfile`, [ADR 0027](adr/0027-a-network-profile-is-typed-and-its-secrets-are-not.md)): the trust decision in the image, the secret and the identity from the fleet |
| Default VPN settings | NetworkManager `vpn` | **gap: kind**: a second slice, with the plugin's fields typed and no free-form `vpn.data` |
| **Software** | | |
| Which search providers answer | `search-providers disable-external`, `disabled` | **modelled** since 2026-10-30 (`search.external`, `.disabled`) |
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

- **The cheap fields have shipped**: the background rendering keys, the logind
  power key and lid switch, idle logout, the XKB layouts and the greeter's two.
  The NFS profile line remains: it edits `/etc/dconf/profile/user`, which the
  platform owns, so it is not a scalar in `GnomeProfile`.
- **Two are decisions**: extension settings (classification banner, heads-up
  display) need a typed surface or a reviewed dconf overlay; polkit
  repartitioning is a security surface and may be refused. The NFS line needs a
  platform profile file. None is a new kind yet.
- **Three are refused on purpose**: single-application mode, custom sessions and
  autostart. All three are an `Exec` in a directory; invariant 5 is the reason,
  and it does not bend for convenience.

## Decisions this produces

1. Open the extension-settings decision: a typed surface for the documented
   extensions, or a reviewed organization dconf overlay; not a raw map on
   `GnomeProfile`.
2. Add the NFS profile line as a platform fact, or refuse it with a reason; a
   platform that owns its profile cannot get it from a scalar alone.
3. Open the polkit decision; refuse unless an organization shows a real need.
4. Keep the refusals written down here rather than rediscovered in review.
5. The lock screen shield left the model: GNOME 40+ has no separate image.
