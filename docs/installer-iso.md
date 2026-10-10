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

- With the published CentOS Stream 10 image, pull it and use
  `quay.io/deskos/deskos-core:latest` in the build command below.
- For your own image, tag it with the reference your machines will update
  from and **push it before handing out the ISO**; a `localhost/` tag, or a
  tag you never pushed, leaves them *without an update source*:

  ```bash
  deskosctl render --workstation deskos-core-centos10 --output ctx
  sudo podman build -t registry.example.internal/deskos/core:stable ctx
  sudo podman push registry.example.internal/deskos/core:stable
  ```

  `bootc-image-builder` builds the ISO from the local image, but the
  reference baked into it is the one above, so the push is what makes future
  updates possible.

### 2. Write the kickstart

Write `config.toml`:

```toml
[customizations.installer.kickstart]
contents = """
text --non-interactive
lang en_US.UTF-8
keyboard us
timezone UTC --utc
zerombr
clearpart --all --initlabel --disklabel=gpt
autopart --noswap --type=lvm
network --bootproto=dhcp --device=link --activate --onboot=on
rootpw --lock
xconfig --startxonboot
reboot
"""
```

> [!WARNING]
> `zerombr` and `clearpart --all` erase **every disk** the installer
> sees, without asking. Adjust partitioning before handing the ISO to
> anyone.

- `rootpw --lock` and no `user`: GNOME Initial Setup creates the first
  account at first boot. Anaconda does not install non-interactively
  without either a root password decision or a user.
- `xconfig --startxonboot`: without an explicit target Anaconda sets
  `multi-user.target`, and the installed workstation boots to a text
  login.
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
    --chown "$(id -u):$(id -g)" registry.example.internal/deskos/core:stable
```

The ISO is `output/bootiso/install.iso`.

## RHEL

The same steps apply to a RHEL workstation (*untested*) on an entitled
host logged in to `registry.redhat.io`, with
`registry.redhat.io/rhel10/bootc-image-builder` instead of the CentOS
builder.

> [!IMPORTANT]
> RHEL ISOs **must never be published**: keep them inside the
> organization that builds them.

## Validation status

CentOS Stream 10 Core at commit `6d95f25`, with this kickstart and the
pinned builder: the 2.9 GB ISO **installed unattended** to a blank 40 GB
virtio disk in QEMU/UEFI in 4.5 minutes, and `tests/vm/bootcheck.py` on
the installed disk passed (DeskOS splash, then GNOME Initial Setup over
the DeskOS wallpaper).

**Not tested:** bare metal, BIOS boot, RHEL ISOs, and updates through the
`bootc switch` reference.
