# Installer ISO

An installer ISO is built locally from a DeskOS image with bootc-image-builder's
`anaconda-iso` type (older builder releases called the same output `iso`). It
embeds the image and installs it offline with the kickstart below.

**Requirements:** Linux with rootful Podman and about 15 GB free.

## Build

### 1. Choose the image reference

The installer runs `bootc switch` to the reference the ISO is built from, so that
reference is where installed machines update from.

- For the CentOS Stream 10 image: use
  `quay.io/deskos/deskos-core-centos-stream-10:latest`, or the image you built
  locally. [architecture.md](architecture.md#validation-status) carries the digest
  `:stable` serves.
- For your own image, tag it with the reference they will update from and push it
  before handing out the ISO; a `localhost/` tag leaves them without an update
  source:

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
> disk and LUKS, and then erases the selected disk. It no longer wipes every disk
> it sees; read the destination screen.

- `rootpw --lock` and no `user`: GNOME Initial Setup creates the first account, so
  there is no root password to guess.
- No `clearpart` and no `autopart`: the disk, the partitioning and LUKS belong to
  whoever installs.
- No `lang`, `keyboard`, `timezone`, `network` or hostname: the installer asks, and
  the ISO carries the image so the install works offline. The `Localization` and
  `Timezone` modules must be enabled for those spokes to appear.
- `xconfig --startxonboot`: without a target Anaconda sets `multi-user.target`, and
  the installed workstation boots to a text login.
- `graphical` runs the graphical UI. Without `--non-interactive`, a complete
  kickstart installs without a click and an incomplete one waits on the summary
  hub instead of aborting. RHEL 10.2's Anaconda aborts a graphical non-interactive
  install at the progress screen (`User interaction required on standalone spoke
  ProgressSpoke`), so do not add the flag back. With no display, use `text`.

### What you can change

The kickstart is Anaconda's, and `config.toml` is bootc-image-builder's. These are
the options DeskOS uses, and the ones to avoid:

| Goal | Option | Notes |
|---|---|---|
| Show the language, keyboard and time zone screens | `[customizations.installer.modules] enable` with `Localization` and `Timezone` | bootc-image-builder enables only Network, Payloads and Storage, so without this those spokes are missing from the summary |
| Answer language, keyboard or time zone instead of asking | `lang`, `keyboard`, `timezone` in the kickstart | the ISO carries the image, so this works offline |
| Set the hostname or bring up the network | `network` in the kickstart | optional; a wired install works without it |
| Leave no root password | `rootpw --lock` | with no `user` line, GNOME Initial Setup creates the first account |
| Boot into a graphical session after install | `xconfig --startxonboot` | without a target, Anaconda sets `multi-user.target` and the machine boots to a text login |
| Graphical or text installer | `graphical` or `text` | `text` is the choice on a host with no display |
| Partitioning, disk selection | `clearpart` and `autopart` | **avoid**: the person installing chooses the disk and LUKS, and `clearpart --all` erases every disk the installer sees |
| Encryption | `autopart --encrypted --passphrase=...` | **avoid**: the passphrase would sit in the ISO in clear text |
| Extra packages or files | `%post` | **avoid**: that is image content, and it belongs in the image; secrets in a kickstart are visible to anyone with the ISO |
| Unattended install | `--non-interactive` | **avoid on RHEL 10**: measured abort at the progress screen (`User interaction required on standalone spoke ProgressSpoke`) |

Without a `config.toml`, bootc-image-builder still produces an ISO, and the
installer still asks for the disk, but the language and time zone spokes are
missing and the installed system lands on `multi-user.target`. That combination has
not been installed end to end here; the configuration above is what the published
preview uses.

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
organization's repository, on an entitled host logged in to `registry.redhat.io`,
with `registry.redhat.io/rhel10/bootc-image-builder` instead of the CentOS
builder.

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

**Not tested in CI:** no check installs the ISO; it is built, signed and published,
and only a person has installed one. That gap reached a published ISO: a downstream
organization's RHEL ISO carried `graphical --non-interactive`, which RHEL 10.2's
Anaconda aborts. The kickstart here does not use the flag. A test-only ISO was
installed unattended in KVM and reached GNOME Initial Setup; that is a manual run.

Storage is interactive, so this ISO cannot finish an install without a person: a
QEMU check can assert that the installer reaches the hub with an editable
Installation Destination, but not that the install completes. Automated install
tests use the test-only ISO in [tests/installer](../tests/installer/README.md): the
same image with a kickstart that answers storage, locale and network. It is never
published; do not put those answers into the shipped ISO.
