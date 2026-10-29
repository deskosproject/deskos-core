# Installer ISO

An installer ISO is built locally from a DeskOS image with bootc-image-builder's
`anaconda-iso` type. The ISO **embeds the image and installs it offline** with the
kickstart below.

**Requirements:** Linux with rootful Podman and about 15 GB free.

## Build

### 1. Choose the image reference

The installer runs `bootc switch` to the reference the ISO is built from, so
**that reference is where installed machines update from**.

- For the CentOS Stream 10 image: use
  `quay.io/deskos/deskos-core-centos-stream-10:latest`, or the image you built
  locally, as the reference below. [architecture.md](architecture.md#validation-status)
  carries the digest `:stable` serves.
- For your own image, tag it with the reference they will update from and **push
  it before handing out the ISO**; a `localhost/` tag leaves them *without an
  update source*:

  ```bash
  deskosctl render --workstation deskos-core-centos-stream-10 --output ctx
  sudo podman build -t registry.example.com/deskos/core:stable ctx
  sudo podman push registry.example.com/deskos/core:stable
  ```

### 2. Write the kickstart

Write `config.toml`:

```toml
# bootc-image-builder enables only the Network, Payloads and Storage modules,
# which leaves the Language/Keyboard and Time & Date spokes out of the summary.
[customizations.installer.modules]
enable = [
  "org.fedoraproject.Anaconda.Modules.Localization",
  "org.fedoraproject.Anaconda.Modules.Timezone",
]

[customizations.installer.kickstart]
contents = """
# Graphical Anaconda, and nothing else is answered.
graphical
rootpw --lock
xconfig --startxonboot
reboot
"""
```

> [!WARNING]
> The installer asks for language, keyboard, timezone, network (optional), the
> disk and LUKS, and then erases **the selected disk**. It no longer wipes every
> disk it sees; read the destination screen. LUKS is opt-in there.

- `rootpw --lock` and no `user`: GNOME Initial Setup creates the first account.
  Anaconda needs a root password decision or a user; the lock leaves no root
  password to guess.
- No `clearpart` and no `autopart`: the disk, the partitioning and LUKS are the
  decision of the person installing, and an unattended kickstart cannot both
  choose a disk safely and let that person encrypt it.
- No `lang`, `keyboard`, `timezone`, `network` or hostname: the installer asks,
  and the ISO carries the image so the install works offline. The `Localization`
  and `Timezone` modules must be enabled for those spokes to appear; BIB does not
  enable them by default.
- `xconfig --startxonboot`: without a target Anaconda sets `multi-user.target`,
  and the installed workstation boots to a text login.
- `graphical` runs Anaconda's graphical UI. Without `--non-interactive`, a
  complete kickstart still installs without a click; an incomplete one waits on
  the summary hub, visible, instead of aborting. RHEL 10.2's Anaconda aborts a
  graphical non-interactive install at the progress screen (`User interaction
  required on standalone spoke ProgressSpoke`) and the machine reboots; do not add
  the flag back. On a host with no display, `text` is the headless option.
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
organization's repository on an entitled host logged in to `registry.redhat.io`,
with `registry.redhat.io/rhel10/bootc-image-builder` instead of the CentOS
builder. A downstream organization has built and installed a private RHEL 10 ISO
in KVM; that evidence belongs to its repository.

> [!IMPORTANT]
> RHEL ISOs **must never be published**: keep them inside the organization that
> builds them.

## Validation status

The CentOS Stream 10 preview ISO at <https://download.deskos.org/> was installed
and booted in KVM: DeskOS splash, then GNOME Initial Setup over the DeskOS
wallpaper. It is a preview built from a published image, not from the current
`main`; verify its published checksum.

**Not tested here:** bare metal, BIOS boot, RHEL ISOs (downstream), and updates
through the `bootc switch` reference.

**Not tested in CI:** no check installs the ISO; it is built, signed and
published, and only a person has installed one. That gap reached a published ISO:
on 2026-10-23 a downstream organization's RHEL ISO carried `graphical
--non-interactive`, which RHEL 10.2's Anaconda aborts. The kickstart here no
longer uses the flag. The test-only ISO was installed unattended in KVM on
2026-10-24 and reached GNOME Initial Setup; that is a manual run, not a CI check.

Because storage is interactive, this ISO cannot finish an install without a
person: a QEMU check can assert that the installer reaches the hub with an
editable Installation Destination, but the install does not complete on its own.
Automated install tests use the test-only ISO in
[tests/installer](../tests/installer/README.md): the same image with a kickstart
that answers storage, locale and network. It is never published; do not put those
answers into the shipped ISO.
