# Installing DeskOS

Two ways to get a machine. The first evaluates DeskOS; the second is the point:
your image, your desktop, your ISO.

| Way | What you do | Use it when |
|---|---|---|
| Install the published image | pull, write a disk or ISO, boot | evaluation; CentOS has a Core ISO, Fedora and Alma a disk image |
| Build your own, then install it | author a `Workstation`, render, build, install | your packages and policy; RHEL needs an entitlement |

Two templates do the second way for you:

| Template | Runner and registry |
|---|---|
| [`deskos-template-cs10`](https://github.com/deskosproject/deskos-template-cs10) | GitHub-hosted runner, image to `ghcr.io`, public |
| [`deskos-template-rhel10`](https://github.com/deskosproject/deskos-template-rhel10) | your entitled RHEL runner, image to your private registry |

Fork one, edit the resources, run the workflow, pull the image. The templates
stop at the image on purpose, because where an ISO lives is your decision.

Neither way creates a user. GNOME Initial Setup does at first boot, and that
account is the administrator.

**You need:** Linux x86_64 with rootful Podman, about 20 GB free, and KVM or a
spare disk.

## Option 1: install the published image

### 1. Pick the reference

| Platform | Image | Tags |
|---|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10` | `:stable`, `:latest`, `:<commit>` |
| Fedora 45 | `quay.io/deskos/deskos-core-fedora-45` | `:latest`, `:candidate`, `:<commit>` |
| AlmaLinux 10 | `quay.io/deskos/deskos-core-almalinux-10` | `:latest`, `:candidate`, `:<commit>` |

`:latest` is the newest image that passed the weekly build. `:stable` is the
digest a promotion accepted. Outside a lab, pin the digest:
[validation status](architecture.md#validation-status) names the current one.

The commands use CentOS Stream 10. For Fedora 45 or AlmaLinux 10, replace the
reference. Fedora also needs `--rootfs btrfs` when writing the disk, and Core
publishes no ISO for either.

### 2. Resolve it to a digest

A publish between the pull and the build would change what you install.

```bash
image=quay.io/deskos/deskos-core-centos-stream-10:stable
sudo podman pull "$image"
digest="$(sudo podman image inspect --format '{{.Digest}}' "$image")"
echo "$digest"
```

### 3. Write a disk or an ISO

A QCOW2 for a virtual machine:

```bash
mkdir -p output
sudo podman run \
    --rm --interactive --tty --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    --volume ./output:/output \
    --volume /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    "quay.io/deskos/deskos-core-centos-stream-10@$digest"
```

The disk is `output/qcow2/disk.qcow2` (10.5 GiB virtual). The image's own quiet
graphical boot stays. CI pins this builder by digest; here `:latest` is fine.

An installer ISO, for the Anaconda flow:

```bash
cat > config.toml <<'EOF'
[customizations.installer.modules]
enable = [
  "org.fedoraproject.Anaconda.Modules.Localization",
  "org.fedoraproject.Anaconda.Modules.Timezone",
]

[customizations.installer.kickstart]
contents = """
graphical
rootpw --lock
reboot
"""
EOF

sudo podman run \
    --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    --volume ./output:/output \
    --volume ./config.toml:/config.toml:ro \
    --volume /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type anaconda-iso --config /config.toml \
    --chown "$(id -u):$(id -g)" \
    "quay.io/deskos/deskos-core-centos-stream-10@$digest"
```

The ISO is `output/bootiso/install.iso`. Anaconda asks for language, keyboard,
timezone, network (optional), disk and LUKS, and erases the disk you select.
Encryption is opt-in. The test-only ISO under `tests/installer/` is different:
its kickstart runs `clearpart --all` and erases every disk it sees.

> **Important:**
> The disk and the ISO bake in the reference they were built from, and the
> installed machine follows it for every later `bootc upgrade`. The kickstarts are
> in [installer-iso.md](installer-iso.md).
A preview ISO is at <https://download.deskos.org/>; verify its `SHA256SUMS`
before booting.

### 4. Boot it

Attach the disk to a UEFI VM, or write the ISO to a USB stick. GNOME Initial
Setup creates the first user. `deskos report` shows the Plan the image carries;
`bootc status` shows the deployment it follows.

## Option 2: build your own, then install it

Use this when you author your own resource root. The example lives in a checkout,
so this way starts with git; `deskosctl` itself is a binary.

### 1. Get `deskosctl`, or fork a template

The templates already carry the binary, the resources and the workflow. To keep
the compiler in your hands, download it from
[Releases](https://github.com/deskosproject/deskos-core/releases).

Version-less asset names track the latest release; a `vX.Y.Z` name pins one. The
exact commands, with the checksum check, are in
[development.md](development.md#releases).

### 2. Author the workstation

A `Workstation` is one platform plus the profiles it is made of. The shipped
example is complete:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-centos-stream-10
spec:
  platformRef: centos-stream-10
  profiles: [deskos-core, deskos-gnome, deskos-dock]
```

`deskos-core` carries packages and the update policy; `deskos-gnome` and
`deskos-dock` carry the desktop defaults (ADR 0016). Every kind is in
[resources.md](resources.md).

### 3. Render and build

```bash
./bin/deskosctl validate ./examples/centos-stream-10
./bin/deskosctl render ./examples/centos-stream-10 \
    --workstation example-centos-stream-10 \
    --output ./dist/centos
sudo podman build -t localhost/deskos-centos:test ./dist/centos
```

### 4. Install it

Use the disk or ISO command from Option 1, with the local tag as the last
argument:

```bash
localhost/deskos-centos:test
```

A `localhost/` image is a lab artifact, not an update source. The installed
machine follows a reference no registry has, so it cannot update. To go further,
push it where machines can reach it:

```bash
sudo podman tag localhost/deskos-centos:test registry.example.com/deskos/core:stable
sudo podman push registry.example.com/deskos/core:stable
```

Build the disk or ISO from that reference. Moving the tag is how the next
version reaches installed machines.

## RHEL 10: private builds only

DeskOS never publishes RHEL 10: the EULA forbids redistribution. An organization
with a subscription builds on a host registered with that subscription and
logged in to `registry.redhat.io`, and pushes only to its private registry.

The steps are Option 2 with two changes: the workstation uses `platformRef:
rhel-10`, and the disk or ISO uses
`registry.redhat.io/rhel10/bootc-image-builder:latest`.

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: deskos-core-rhel-10
spec:
  displayName: DeskOS Core (RHEL 10)
  platformRef: rhel-10
  profiles: [deskos-core, deskos-gnome]
```

Every layer of the image this repository builds is free of the build host's
subscription state; a build you make is your own to check. Keep it in private
registries and storage.

A private registry means the installed machine needs credentials for the first
`bootc upgrade`: `/etc/ostree/auth.json`, supplied by provisioning, never by the
image or the ISO. [Installer ISO](installer-iso.md#a-private-registry) has the
file and the warning.

No QCOW2 and no RHEL ISO are published. The platform matrix is
[platforms.md](platforms.md).
