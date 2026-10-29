# Installing DeskOS

Two ways to end up with a machine:

- **Option 1: install the published image.** Nothing is compiled: take the
  CentOS Stream 10 image the pipeline built, scanned and booted, write a disk or an
  ISO from it, install it. **This is for evaluating DeskOS, not for a fleet.**
- **Option 2: build your own image, then install it.** Author a `Workstation` in a
  resource root, render it, build it, and install the result. RHEL 10 is this
  option on an entitled base. **This is the point of DeskOS:** your image, your
  desktop, your ISO.

Two template repositories do Option 2 for you:

| Template | Runner and registry |
|---|---|
| [`deskos-template-cs10`](https://github.com/deskosproject/deskos-template-cs10) | GitHub-hosted runner, image to `ghcr.io`, public |
| [`deskos-template-rhel10`](https://github.com/deskosproject/deskos-template-rhel10) | your entitled RHEL runner, image to your private registry |

Fork one, edit the resources, run the workflow, and pull the image. The template's
README then builds the ISO with the same command as step 3 of Option 1; the
templates stop at the image on purpose, because where an ISO lives is your
decision.

Either way the machine has **no user account**: GNOME Initial Setup creates the
first one at first boot, and that account is the administrator.

**You need:** Linux x86_64 with rootful Podman, about **20 GB** free, and either
KVM for a virtual machine or a spare disk for a machine.

## Option 1: install the published image

### 1. Pick the reference

| Platform | Image | Tags |
|---|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10` | `:stable`, `:latest`, `:<commit>` |

`:latest` is the newest image that passed the weekly build and `:stable` is the digest a
promotion accepted. Outside a lab, **pin the verified digest**:
[validation status](architecture.md#validation-status) names the current one.

### 2. Resolve it to a digest

A publish between the pull and the build would otherwise change what you install.

```bash
image=quay.io/deskos/deskos-core-centos-stream-10:stable
sudo podman pull "$image"
digest="$(sudo podman image inspect --format '{{.Digest}}' "$image")"
echo "$digest"
```

### 3. Write a disk, or make an installer ISO

**A QCOW2 for a virtual machine:**

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    "quay.io/deskos/deskos-core-centos-stream-10@$digest"
```

The disk is `output/qcow2/disk.qcow2` (10.5 GiB virtual). `--no-default-kernel-args`
keeps the image's own quiet graphical boot. CI pins this builder by digest, in the
workflow; here `:latest` is fine, and a pinned digest is a line you add when you
need to build exactly what a past run built.

**An installer ISO, if you want the Anaconda flow.** `anaconda-iso` is the current
name for this output; older builder releases called it `iso`.

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

sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output -v ./config.toml:/config.toml:ro \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type anaconda-iso --config /config.toml \
    --chown "$(id -u):$(id -g)" \
    "quay.io/deskos/deskos-core-centos-stream-10@$digest"
```

The ISO is `output/bootiso/install.iso`. The kickstart answers nothing about
storage: Anaconda asks for language, keyboard, timezone, network (optional), the
disk and LUKS, and erases **the disk you select**. Read that screen, and note
that encryption is opt-in. The test-only ISO under `tests/installer/` is a
different fixture: its kickstart runs `clearpart --all`, which erases every disk
the installer sees.

> [!IMPORTANT]
> The disk and the ISO both bake in the reference they were built from, and **that
> is what the installed machine follows for every later `bootc upgrade`**. The
> kickstarts and the reasoning behind them are in
> [installer-iso.md](installer-iso.md).

A **preview ISO** is also published at <https://download.deskos.org/>; verify its
`SHA256SUMS` before booting it.

### 4. Boot it

Attach the disk to a VM with UEFI firmware, or write the ISO to a USB stick. The
machine reaches GNOME Initial Setup; create the first user there. `deskos report`
then shows the Plan the image carries, and `bootc status` the deployment it
follows.

## Option 2: build your own, then install it

Use this when you author your own resource root: an organization's packages,
desktop defaults and update policy. The example below lives in a checkout, so this
option starts with **git**; `deskosctl` itself is a binary.

### 1. Get `deskosctl`, or fork a template

The two templates above already carry the binary download, the resources and the
workflow. Read on if you want the compiler in your hands.

Download the binary from
[GitHub Releases](https://github.com/deskosproject/deskos-core/releases):
version-less asset names track the latest release, a `vX.Y.Z` name pins one. The
exact commands, including the checksum check, are in
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
    --workstation example-centos-stream-10 --output ./dist/centos
sudo podman build -t localhost/deskos-centos:test ./dist/centos
```

### 4. Install it

A locally built image has no registry digest, so pass the tag itself as the last
argument:

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    localhost/deskos-centos:test
```

For an ISO, use the `config.toml` and `--type anaconda-iso` command from Option 1
with the same last argument.

A `localhost/` image is a lab artifact, not an update source: a machine installed
from it follows a reference no registry has, so it cannot update. To go further,
tag it with the reference the machines will follow and push it where they can
reach it:

```bash
sudo podman tag localhost/deskos-centos:test registry.example.com/deskos/core:stable
sudo podman push registry.example.com/deskos/core:stable
```

Then build from that reference (step 2's `image=` line, or the tag as the last
argument), and changing the tag is how the next version reaches installed
machines.

## RHEL 10: private builds only

RHEL 10 is **never published by DeskOS**: the EULA forbids redistributing it. An
organization with a subscription builds it on a **host registered with that
subscription and logged in to `registry.redhat.io`**, and pushes only to a
registry private to the organization.

The steps are Option 2 with two changes: the workstation uses `platformRef:
rhel-10`, and the disk or ISO is built with the RHEL builder
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

In the image this repository builds, every layer is free of the build host's
subscription state ([validation status](architecture.md#validation-status)); a
build you make is your own to check. These are still RHEL derivatives: keep them
in private registries and storage.

A private registry means the installed machine needs credentials for the first
`bootc upgrade`: `/etc/ostree/auth.json`, supplied by provisioning, never by the
image or the ISO. [Installer ISO](installer-iso.md#a-private-registry) has the file
and the warning.

No QCOW2 and no RHEL ISO are published. The platform matrix is
[platforms.md](platforms.md).
