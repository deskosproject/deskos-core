# Test-only installer ISO

The published ISO answers nothing: language, keyboard, timezone, network, the
disk and LUKS are the person's decisions ([installer-iso.md](../../docs/installer-iso.md)).
That makes it the right installer to hand out and the wrong one for an
automated install test, which cannot click.

This directory holds the second ISO. Its kickstart answers storage, locale and
network, so an install runs unattended in a disposable VM.

| ISO | Build configuration | Published |
| --- | --- | --- |
| Installer | [docs/installer-iso.md](../../docs/installer-iso.md) | Yes |
| Test-only | `tests/installer/config.toml` | **Never** |

Do not put the test answers back into the published ISO, and do not hand out an
ISO built here: `clearpart --all` erases every disk the installer sees.

## Build

On a Linux host with rootful Podman:

```bash
tests/installer/build.sh quay.io/deskos/deskos-core-centos-stream-10:latest
```

The ISO is `tests/installer/output/bootiso/install.iso`. Use an image reference
the test does not need to update from; unlike the published ISO, this one is
not pinned to an update source.

## Run

```bash
tests/installer/boot.py \
    --iso tests/installer/output/bootiso/install.iso \
    --workdir /tmp/deskos-install-test \
    --marker "Install finished"
```

The builder's own kickstart writes `Install finished` to the serial console, so
`--marker` turns "the install completed" into an exit status instead of a
screenshot to inspect. Without `--marker`, only the frames are evidence.

The harness creates a throwaway qcow2, boots the ISO with UEFI and captures a
screendump every 20 seconds for 25 minutes. The CD boots once (`-boot once=d`),
so the kickstart's reboot lands on the installed disk instead of starting the
installer again. The last screenshots should show GNOME Initial Setup. The
kickstart writes `/etc/deskos-installer-test`, so an installed disk can be
traced to this ISO once you can read or mount it.

QEMU writes the frames as PPM bytes even though the old filenames said `.png`;
they are now named `.ppm`, and `magick shot-01.ppm shot-01.png` converts one for
viewing.

A run on 2026-10-24 with the published CentOS Stream 10 image installed
unattended and reached GNOME Initial Setup. It has not run in CI.

A screenshot that shows the Installation Summary with a disk step waiting means
the kickstart did not answer storage; do not reuse that ISO for the test.

## What it answers

- `clearpart --all --initlabel` and `autopart`: one disk, wiped. Test only.
- `lang`, `keyboard`, `timezone`: fixed test values, not preferences.
- `network --bootproto=dhcp`: the installer does not wait for a person, and the
  installed system has no network configuration from the kickstart.
- `rootpw --lock`: no root password; GNOME Initial Setup creates the first
  account, as on the published ISO.
- `xconfig --startxonboot`: the installed system boots to a graphical login.

The ISO still carries the image and installs offline, like the published one.
