# Session check (session, instrumented boot): FAIL

- Boot: pass []
- PASS: session results reported (session_type=wayland)
- PASS: user extensions allowed (disable-user-extensions=false)
- PASS: plan binary /usr/bin/deskos (sha256 eead05aebf12d8dc32d48e35865039fb1e6b021490eb3a66dafd811a204f9b47, mode 755, as the plan declares)
- PASS: extension loads (appindicatorsupport@rgcjonas.gmail.com) (Enabled/State=Yes/ACTIVE)
- PASS: extension loads (dash-to-dock@micxgx.gmail.com) (Enabled/State=Yes/ACTIVE)
- PASS: tray watcher serves the session bus (tray_watcher=present)
- PASS: tray watcher accepts a registration (tray_register=accepted)
- PASS: wallpaper as planned (got 'file:///usr/share/deskos/backgrounds/deskos-light.svg' want 'file:///usr/share/deskos/backgrounds/deskos-light.svg')
- FAIL: login logo file exists (logo=unset file=not read)
- FAIL: the greeter reads the login logo (greeter=/usr/share/deskos/branding/deskos-login-logo.svg session=unset)
- PASS: screen locks on request (screen_locked=true)
- PASS: deskos report names the installed workstation (got deskos-core-centos-stream-10 (exit=0))
- PASS: deskos check runs on the booted image (exit=1)
- PASS: declared system Flatpaks are installed (missing=none)
- PASS: the guest says whether Firefox is installed (firefox_present=yes)
- PASS: Firefox runs headless (exit=0 screenshot=yes)
- PASS: Firefox fetches the harness page (fetch=ok content=ok requests=1 screenshot=yes)
- PASS: plan default /org/gnome/desktop/background/picture-uri (the image's distro database carries 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/background/picture-uri-dark (the image's distro database carries 'file:///usr/share/deskos/backgrounds/deskos-dark.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/deskos-dark.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/lock-delay (the image's distro database carries uint32 0, as the plan declares; the session sees uint32 0, as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/lock-enabled (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/picture-uri (the image's distro database carries 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/session/idle-delay (the image's distro database carries uint32 600, as the plan declares; the session sees uint32 600, as the plan declares)
- PASS: plan default /org/gnome/desktop/wm/preferences/button-layout (the image's distro database carries ':minimize,maximize,close', as the plan declares; the session sees ':minimize,maximize,close', as the plan declares)
- FAIL: plan default /org/gnome/login-screen/logo (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
- PASS: plan default /org/gnome/shell/enabled-extensions (the image's distro database carries ['background-logo@fedorahosted.org', 'appindicatorsupport@rgcjonas.gmail.com', 'dash-to-dock@micxgx.gmail.com'], as the plan declares; the session sees ['background-logo@fedorahosted.org', 'appindicatorsupport@rgcjonas.gmail.com', 'dash-to-dock@micxgx.gmail.com'], as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/autohide (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dash-max-icon-size (the image's distro database carries 40, as the plan declares; the session sees 40, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dock-fixed (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dock-position (the image's distro database carries 'BOTTOM', as the plan declares; the session sees 'BOTTOM', as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/intellihide (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/show-trash (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan default /org/gnome/software/allow-updates (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/software/download-updates (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan vendor default org.gnome.desktop.background picture-uri (the compiled schema default carries 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/deskos-light.svg', as the plan declares)
- PASS: plan vendor default org.gnome.desktop.background picture-uri-dark (the compiled schema default carries 'file:///usr/share/deskos/backgrounds/deskos-dark.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/deskos-dark.svg', as the plan declares)
- PASS: no unexpected failed system units (failed=mcelog.service; known non-blocking: mcelog.service (see Platform diagnostics))
- PASS: no failed user units (failed=)
- Plan defaults: 19 declared; 18 verified in the image, 18 verified in the session, 2 unverified reads
  - plan default /org/gnome/login-screen/logo [image: unread] (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
  - plan default /org/gnome/login-screen/logo [session: unread] (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
- Plan binaries: 1 declared; 1 verified against the checksum the plan pins, 0 verified as installed (the plan pins an archive)

## Platform diagnostics

- FAIL (known, non-blocking): mcelog.service, AMD guest without edac_mce_amd (LoadState=loaded ActiveState=failed SubState=failed Result=exit-code ConditionResult=yes ExecMainStatus=1)
- CPU: vendor=AuthenticAMD family=25 model=AMD EPYC 9V74 80-Core Processor
- edac_mce_amd=absent /dev/mcelog=present
- kernel mce/edac lines: 1 (first 100 at most)

```
[    1.232929] localhost kernel: EDAC MC: Ver: 3.0.0
```

Journal excerpt: journal.txt. Frames: frames/ and summary.md.
