# Installing DeskOS

How to get a DeskOS **QCOW2 disk** or **installer ISO**, from the
published CentOS Stream 10 image, a private RHEL 10 build, an AlmaLinux 10
build, or an Ubuntu 26.04 build.

> [!NOTE]
> Downloadable QCOW2 and ISO images are not published yet. Until they
> are, create an EL disk from the container image with
> `bootc-image-builder`, and an Ubuntu disk with `bootc install to-disk`
> ([Ubuntu 26.04](#ubuntu-2604) below). You need **Linux with rootful
> Podman** and about **20 GB free**.

## CentOS Stream 10

DeskOS Core for CentOS Stream 10 is published as a bootable container
image. `:latest` is the newest published image — the one that passed the
boot and session checks. Publishing is manual (the `supply-chain` workflow
with `publish=true`) and moves `:latest` and the `:<commit>` tag together;
for production, pin the digest you verified.

**1. Pull the image and make a QCOW2 disk** for a virtual machine. Resolve
`:latest` once and build from that digest, so a publish between the pull and
the build cannot change what you install:

```bash
image=quay.io/deskos/deskos-core:latest
sudo podman pull "$image"
digest="$(sudo podman image inspect --format '{{.Digest}}' "$image")"

mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    "quay.io/deskos/deskos-core@$digest"
```

> The CI pins this builder by digest for reproducibility; `:latest` is the
> same image today.

The disk is `output/qcow2/disk.qcow2` (*10.5 GiB virtual* by default).

- **No user account is baked in:** GNOME Initial Setup creates the first
  one at first boot.
- `--no-default-kernel-args` keeps the image's own boot arguments (quiet
  graphical splash) instead of the builder's serial console.

> [!TIP]
> For a larger root filesystem, mount a `config.toml`
> (`-v ./config.toml:/config.toml:ro` and `--config /config.toml` after
> `build`) containing:
>
> ```toml
> [[customizations.filesystem]]
> mountpoint = "/"
> minsize = "40 GiB"
> ```

**Or make an installer ISO:** use `--type anaconda-iso` with a kickstart
in `config.toml`. The kickstart DeskOS needs is in
[installer-iso.md](installer-iso.md), together with the warning that it
**erases every disk**. Machines installed from it update from the image
reference it was built from (`quay.io/deskos/deskos-core:latest`): Core
stages the new image daily, on AC power, and it applies at the next
reboot (see [Updates](operations.md)).

## RHEL 10 (private builds only)

> [!IMPORTANT]
> DeskOS publishes **only CentOS Stream 10** artifacts. RHEL-based images,
> disks and ISOs must **never be published**: the RHEL EULA forbids public
> redistribution. An organization with RHEL subscriptions builds its own
> and keeps the result inside the organization.

Build on a **registered RHEL 10 host** logged in to `registry.redhat.io`,
as root (the build runs as root).

**1. Get `deskosctl`** from
[GitHub Releases](https://github.com/deskosproject/deskos-core/releases).
The version-less asset names track the latest release; use a `vX.Y.Z` name
to pin one. No Go toolchain or git is needed:

```bash
base=https://github.com/deskosproject/deskos-core/releases/latest/download
mkdir deskos && cd deskos
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl
```

**2. Declare a RHEL 10 workstation** in a resource root of your own, for
example `my-org/workstations/core-rhel10.yaml`:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: deskos-core-rhel10
spec:
  displayName: DeskOS Core (RHEL 10)
  platformRef: rhel-10
  profiles:
    - deskos-core
```

**3. Render and build it.** Tag it with the reference your machines
should update from, in a registry that is **private to your
organization**:

```bash
./bin/deskosctl render ./my-org \
    --workstation deskos-core-rhel10 --output ./dist/deskos-core-rhel10
sudo podman build -t registry.example.com/deskos/core-rhel10:latest \
    ./dist/deskos-core-rhel10
```

**4. Make the QCOW2** (or the ISO, as above) with the **RHEL** builder:

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    registry.redhat.io/rhel10/bootc-image-builder@sha256:7f5baead2d4ac2a1035900ced31e4e7600fc98f69aa45ee5d05639bca028e00b \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    registry.example.com/deskos/core-rhel10:latest
```

In the image DeskOS builds and tests, every layer is free of the build
host's subscription state (see [validation status](architecture.md#validation-status));
a build you make is your own to check. The image, disks and ISOs are
**still RHEL derivatives**: push them only to registries and storage that
are private to your organization.

## AlmaLinux 10

AlmaLinux 10 is a public, subscription-free EL10 base — the AlmaLinux project's
own, which that project labels experimental. DeskOS publishes no AlmaLinux image
and has **not built or booted** one: the steps below have the same shape as the
CentOS Stream 10 ones and are **proposed, not measured**. The platform's facts
are EL10 conventions modeled on RHEL 10, not verified against AlmaLinux
([platforms.md](platforms.md)).

Get `deskosctl` as in step 1 of the RHEL section above, then render the shipped
example from a checkout of this repository and build it:

```bash
./bin/deskosctl render ./examples/core-almalinux-10 \
    --workstation example-core-almalinux-10 --output ./dist/alma
sudo podman build -t localhost/deskos-alma:test ./dist/alma
```

Then write the QCOW2 with the public `bootc-image-builder`, as in the CentOS
Stream 10 steps above.

## Ubuntu 26.04

DeskOS publishes no Ubuntu image and implements no installer ISO for it — there is
no ISO path here, built or validated. The base is owned by the DeskOS project and
pinned by digest in `resources/platforms/ubuntu-2604.yaml`. Get `deskosctl` (step
1 of the RHEL section above), then build the image and write a disk with
`bootc install to-disk`:

**1. Render, build and push the image**, tagged with the reference your machines
will update from:

```bash
./bin/deskosctl render ./examples/core-ubuntu-2604 \
    --workstation example-core-ubuntu-2604 --output ./dist/ubuntu
sudo podman build -t registry.example.com/deskos/ubuntu:latest ./dist/ubuntu
sudo podman push registry.example.com/deskos/ubuntu:latest
```

The installed machine updates from that reference, so it must be able to reach it:
a registry your organization hosts, or one you make public. DeskOS does not
publish an Ubuntu image for you.

**2. Recompress it into an OCI layout**, which the install mounts as its source
(the composefs import rejects uncompressed layers):

```bash
sudo skopeo copy \
    containers-storage:registry.example.com/deskos/ubuntu:latest \
    oci:/tmp/oci:test
```

**3. Write a disk with `bootc install to-disk`**, pointing `--target-imgref` at
the reference from step 1:

```bash
truncate -s 32G disk.raw
sudo podman run --rm --privileged --pid=host --security-opt label=type:unconfined_t \
    -v /dev:/dev -v /var/lib/containers:/var/lib/containers \
    -v /tmp/oci:/src-oci:ro -v "$PWD":/output \
    registry.example.com/deskos/ubuntu:latest \
    bootc install to-disk --via-loopback /output/disk.raw \
    --source-imgref oci:/src-oci:test \
    --target-imgref registry.example.com/deskos/ubuntu:latest \
    --composefs-backend --bootloader systemd --wipe --filesystem ext4 --allow-missing-verity
```

The disk is `disk.raw`.

Three things here that must not be conflated:

- **CI makes a lab disk, not a delivery.** The workflow installs from a local OCI
  layout with **no `--target-imgref`**; that proves the image builds, installs and
  boots, and says nothing about where the machine updates from afterwards.
- **A real install points `--target-imgref` at a derivative you publish** — a
  private registry, or one you make public — never at the local layout and never
  at the base: pointing it at the base would replace your derivative with the base
  at the next update.
- **Publishing the derivative is your step, and CI does not test it.** DeskOS
  publishes no Ubuntu image; that the image is built, installed and booted in CI
  says nothing about a registry you publish to.

After that the disk updates with `bootc` like any other platform
([operations.md](operations.md)).
