# Installer ISO

An installer ISO is built locally from a DeskOS image with
bootc-image-builder's `anaconda-iso` type. The ISO **embeds the image and
installs it offline, unattended**, with the kickstart below.

**Requirements:** Linux with rootful Podman and about 15 GB free.

## Build

### 1. Choose the image reference

The installer runs `bootc switch` to the reference the ISO is built from, so
**that reference is where installed machines update from**: it must be one
they can pull.

- For the CentOS Stream 10 image: pull it from
  `quay.io/deskos/deskos-core-centos-stream-10:latest`, or use the one you built locally,
  and use that reference in the build command below.
  [architecture.md](architecture.md#validation-status) carries the digest the last
  `publish=true` dispatch published.
- For your own image, tag it with the reference your machines will update
  from and **push it before handing out the ISO**; a `localhost/` tag, or a
  tag you never pushed, leaves them *without an update source*:

  ```bash
  deskosctl render --workstation deskos-core-centos-stream-10 --output ctx
  sudo podman build -t registry.example.com/deskos/core:stable ctx
  sudo podman push registry.example.com/deskos/core:stable
  ```

  `bootc-image-builder` builds the ISO from the local image, but the
  reference baked into it is the one above, so the push is what makes future
  updates possible.

### 2. Write the kickstart

Write `config.toml`:

```toml
# bootc-image-builder enables only the Network, Payloads and Storage modules,
# which leaves the Language/Keyboard and Time & Date spokes out of the
# summary. Enable both so the person installing can choose them; the
# kickstart below answers neither.
[customizations.installer.modules]
enable = [
  "org.fedoraproject.Anaconda.Modules.Localization",
  "org.fedoraproject.Anaconda.Modules.Timezone",
]

[customizations.installer.kickstart]
contents = """
# Graphical Anaconda, and nothing else is answered. Language, keyboard,
# timezone, network, the disk and LUKS are the person's decisions. A
# kickstart with clearpart/autopart auto-selects every disk, disables the
# Installation Destination spoke and gives no way to encrypt; a preseeded
# locale or network hides choices the person may need to change. The ISO
# carries the image, so the install needs no network.
#
# Do not add --non-interactive either: in Anaconda 40.22 (RHEL 10.2) the
# progress screen is a standalone spoke, and the non-interactive check aborts
# the install with "User interaction required on standalone spoke
# ProgressSpoke".
graphical
rootpw --lock
xconfig --startxonboot
reboot
"""
```

> [!WARNING]
> The installer asks for language, keyboard, timezone, network (optional),
> the disk and LUKS, and then erases **the selected disk**. It no longer
> prescribes storage, so it no longer wipes every disk it sees; but the person
> installing must still read the destination screen and be told that the
> installation destroys the selected disk. LUKS is opt-in on the same screen.

- `rootpw --lock` and no `user`: GNOME Initial Setup creates the first
  account at first boot. Anaconda requires either a root password decision or
  a user before it will install; the lock satisfies that while leaving no root
  password to guess.
- No `clearpart` and no `autopart`: the installation is interactive by
  design. Storage — the disk, the partitioning and LUKS — is the decision of
  the person installing, and an unattended kickstart cannot both choose a disk
  safely and let that person encrypt it.
- No `lang`, `keyboard`, `timezone` or `network`: the installer asks. The ISO
  carries the image, so the install works offline; the hostname is not preset
  either. Preseeding these hides choices a person may need to change (a
  different keyboard, a different timezone, Wi-Fi). The `Localization` and
  `Timezone` Anaconda modules must be enabled for those spokes to appear at
  all; BIB does not enable them by default.
- `xconfig --startxonboot`: without an explicit target Anaconda sets
  `multi-user.target`, and the installed workstation boots to a text
  login.
- `graphical` runs Anaconda's graphical UI. Without `--non-interactive`,
  a complete kickstart still installs without a click; an incomplete one
  waits on the summary hub, visible, instead of aborting. RHEL 10.2's
  Anaconda aborts a graphical non-interactive install at the progress
  screen (`User interaction required on standalone spoke ProgressSpoke`),
  then exits and the machine reboots; do not add the flag back. On a host
  with no display, `text` is the headless option.
- bootc-image-builder adds the `ostreecontainer` line itself.

### 3. Build the ISO

```bash
mkdir -p output
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output -v ./config.toml:/config.toml:ro \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type anaconda-iso --config /config.toml \
    --chown "$(id -u):$(id -g)" registry.example.com/deskos/core:stable
```

The ISO is `output/bootiso/install.iso`.

## RHEL

This repository does not build RHEL. The same steps apply in a downstream
organization's repository on an entitled host logged in to
`registry.redhat.io`, with
`registry.redhat.io/rhel10/bootc-image-builder` instead of the CentOS
builder. The DayTwo organization has built and installed a private RHEL 10
ISO in KVM; that evidence belongs to its repository.

> [!IMPORTANT]
> RHEL ISOs **must never be published**: keep them inside the
> organization that builds them.

## Validation status

The CentOS Stream 10 preview ISO at <https://download.deskos.org/> was
installed and booted in KVM: DeskOS splash, then GNOME Initial Setup over
the DeskOS wallpaper. It is a preview built from a published image, not
from the current `main`; verify the checksum published beside it.

**Not tested here:** bare metal, BIOS boot, RHEL ISOs (downstream), and
updates through the `bootc switch` reference.

**Not tested in CI:** no check installs the ISO; it is built, signed and
published, and only a person has installed one. That gap reached a published
ISO: on 2026-10-23 the DayTwo RHEL ISO carried `graphical --non-interactive`,
which RHEL 10.2's Anaconda aborts at the progress screen, and the machine
rebooted. The kickstart in this document no longer uses the flag.

Because storage is interactive, this ISO cannot finish an install without a
person: a QEMU check can assert that the installer reaches the hub with an
editable Installation Destination (the screen that used to be locked), but
completing the install does not happen on its own. Automated install tests use
the second, test-only ISO in [tests/installer](../tests/installer/README.md):
the same image with a kickstart that answers storage, locale and network. It is
never published; do not put those answers back into the shipped ISO.
