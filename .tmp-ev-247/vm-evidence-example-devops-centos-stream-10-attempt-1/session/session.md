# Session check (session, instrumented boot): FAIL

- Boot: pass []
- PASS: session results reported (session_type=wayland)
- PASS: user extensions allowed (disable-user-extensions=false)
- PASS: plan binary /usr/bin/deskos (sha256 eead05aebf12d8dc32d48e35865039fb1e6b021490eb3a66dafd811a204f9b47, mode 755, as the plan declares)
- PASS: plan binary /usr/local/bin/oc (installed and executable (sha256 f08f0eaac81b…), mode 755, as the plan declares; the plan pins the tar.gz archive's checksum, which the build verified and this cannot compare)
- PASS: extension loads (appindicatorsupport@rgcjonas.gmail.com) (Enabled/State=Yes/ACTIVE)
- PASS: extension loads (dash-to-dock@micxgx.gmail.com) (Enabled/State=Yes/ACTIVE)
- PASS: tray watcher serves the session bus (tray_watcher=present)
- PASS: tray watcher accepts a registration (tray_register=accepted)
- PASS: favorites as planned (got ['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'] want ['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'])
- PASS: favorite entry google-chrome.desktop (google-chrome.desktop is installed)
- PASS: favorite entry org.gnome.Nautilus.desktop (org.gnome.Nautilus.desktop is installed)
- PASS: favorite entry org.gnome.Ptyxis.desktop (org.gnome.Ptyxis.desktop is installed)
- PASS: favorite entry com.microsoft.VSCode.desktop (com.microsoft.VSCode.desktop is installed)
- PASS: favorite command google-chrome.desktop (google-chrome.desktop launches a command this image ships)
- PASS: favorite command org.gnome.Nautilus.desktop (org.gnome.Nautilus.desktop launches a command this image ships)
- PASS: favorite command org.gnome.Ptyxis.desktop (org.gnome.Ptyxis.desktop launches a command this image ships)
- PASS: favorite command com.microsoft.VSCode.desktop (com.microsoft.VSCode.desktop launches a command this image ships)
- PASS: wallpaper as planned (got 'file:///usr/share/deskos/backgrounds/example.svg' want 'file:///usr/share/deskos/backgrounds/example.svg')
- FAIL: login logo file exists (logo=unset file=not read)
- FAIL: the greeter reads the login logo (greeter=/usr/share/deskos/branding/example-login-logo.svg session=unset)
- PASS: screen locks on request (screen_locked=true)
- PASS: deskos report names the installed workstation (got example-devops-centos-stream-10 (exit=0))
- PASS: deskos check runs on the booted image (exit=1)
- PASS: declared system Flatpaks are installed (missing=none)
- PASS: the guest says whether Firefox is installed (firefox_present=yes)
- PASS: Firefox runs headless (exit=0 screenshot=yes)
- PASS: Firefox fetches the harness page (fetch=ok content=ok requests=1 screenshot=yes)
- PASS: plan default /org/gnome/desktop/background/picture-uri (the image's distro database carries 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/background/picture-uri-dark (the image's distro database carries 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/accent-color (the image's distro database carries 'blue', as the plan declares; the session sees 'blue', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/color-scheme (the image's distro database carries 'prefer-dark', as the plan declares; the session sees 'prefer-dark', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/cursor-theme (the image's distro database carries 'Adwaita', as the plan declares; the session sees 'Adwaita', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/document-font-name (the image's distro database carries 'Noto Sans 11', as the plan declares; the session sees 'Noto Sans 11', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/font-name (the image's distro database carries 'Noto Sans 11', as the plan declares; the session sees 'Noto Sans 11', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/icon-theme (the image's distro database carries 'Adwaita', as the plan declares; the session sees 'Adwaita', as the plan declares)
- PASS: plan default /org/gnome/desktop/interface/monospace-font-name (the image's distro database carries 'Noto Sans Mono 10', as the plan declares; the session sees 'Noto Sans Mono 10', as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/lock-delay (the image's distro database carries uint32 0, as the plan declares; the session sees uint32 0, as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/lock-enabled (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/desktop/screensaver/picture-uri (the image's distro database carries 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares)
- PASS: plan default /org/gnome/desktop/session/idle-delay (the image's distro database carries uint32 300, as the plan declares; the session sees uint32 300, as the plan declares)
- PASS: plan default /org/gnome/desktop/wm/preferences/button-layout (the image's distro database carries ':close', as the plan declares; the session sees ':close', as the plan declares)
- FAIL: plan default /org/gnome/login-screen/logo (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
- PASS: plan default /org/gnome/shell/enabled-extensions (the image's distro database carries ['background-logo@fedorahosted.org', 'appindicatorsupport@rgcjonas.gmail.com', 'dash-to-dock@micxgx.gmail.com'], as the plan declares; the session sees ['background-logo@fedorahosted.org', 'appindicatorsupport@rgcjonas.gmail.com', 'dash-to-dock@micxgx.gmail.com'], as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/autohide (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dash-max-icon-size (the image's distro database carries 40, as the plan declares; the session sees 40, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dock-fixed (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/dock-position (the image's distro database carries 'BOTTOM', as the plan declares; the session sees 'BOTTOM', as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/intellihide (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/shell/extensions/dash-to-dock/show-trash (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan default /org/gnome/shell/favorite-apps (the image's distro database carries ['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'], as the plan declares; the session sees ['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'], as the plan declares)
- PASS: plan default /org/gnome/software/allow-updates (the image's distro database carries true, as the plan declares; the session sees true, as the plan declares)
- PASS: plan default /org/gnome/software/download-updates (the image's distro database carries false, as the plan declares; the session sees false, as the plan declares)
- PASS: plan lock /org/gnome/shell/favorite-apps cannot be changed (write=locked before=['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'] after=['google-chrome.desktop', 'org.gnome.Nautilus.desktop', 'org.gnome.Ptyxis.desktop', 'com.microsoft.VSCode.desktop'])
- PASS: plan vendor default org.gnome.desktop.background picture-uri (the compiled schema default carries 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares)
- PASS: plan vendor default org.gnome.desktop.background picture-uri-dark (the compiled schema default carries 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares; the session sees 'file:///usr/share/deskos/backgrounds/example.svg', as the plan declares)
- PASS: no unexpected failed system units (failed=mcelog.service; known non-blocking: mcelog.service (see Platform diagnostics))
- PASS: no failed user units (failed=)
- Plan defaults: 27 declared; 26 verified in the image, 26 verified in the session, 2 unverified reads
  - plan default /org/gnome/login-screen/logo [image: unread] (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
  - plan default /org/gnome/login-screen/logo [session: unread] (the image's distro database carries no value for this key; the session sees no value for this key; the image is not shown to carry the declared default)
- Plan binaries: 2 declared; 1 verified against the checksum the plan pins, 1 verified as installed (the plan pins an archive)

## Platform diagnostics

- FAIL (known, non-blocking): mcelog.service, AMD guest without edac_mce_amd (LoadState=loaded ActiveState=failed SubState=failed Result=exit-code ConditionResult=yes ExecMainStatus=1)
- CPU: vendor=AuthenticAMD family=26 model=AMD EPYC 9V45 96-Core Processor
- edac_mce_amd=absent /dev/mcelog=present
- kernel mce/edac lines: 6 (first 100 at most)

```
[    0.923467] localhost kernel: Command line: BOOT_IMAGE=(hd0,gpt3)/boot/ostree/default-73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/vmlinuz-6.12.0-279.el10.x86_64 root=UUID=2b861345-635b-44a7-8f02-0a0f23c6c040 rw boot=UUID=c3ded7f9-764b-44f2-81a5-6899cf210ef8 quiet rhgb ostree=/ostree/boot.1/default/73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/0
[    0.924467] localhost kernel: Kernel command line: BOOT_IMAGE=(hd0,gpt3)/boot/ostree/default-73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/vmlinuz-6.12.0-279.el10.x86_64 root=UUID=2b861345-635b-44a7-8f02-0a0f23c6c040 rw boot=UUID=c3ded7f9-764b-44f2-81a5-6899cf210ef8 quiet rhgb ostree=/ostree/boot.1/default/73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/0
[    0.924474] localhost kernel: Unknown kernel command line parameters "rhgb BOOT_IMAGE=(hd0,gpt3)/boot/ostree/default-73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/vmlinuz-6.12.0-279.el10.x86_64 boot=UUID=c3ded7f9-764b-44f2-81a5-6899cf210ef8 ostree=/ostree/boot.1/default/73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/0", will be passed to user space.
[    0.931190] localhost kernel: EDAC MC: Ver: 3.0.0
[    0.934560] localhost kernel:     BOOT_IMAGE=(hd0,gpt3)/boot/ostree/default-73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/vmlinuz-6.12.0-279.el10.x86_64
[    0.934572] localhost kernel:     ostree=/ostree/boot.1/default/73b88bcdc39cf20b5eb756aac367cafedac3743dd5fd512257b6e83eb453b39a/0
```

Journal excerpt: journal.txt. Frames: frames/ and summary.md.
