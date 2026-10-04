# Installing DeskOS

How to get a DeskOS **QCOW2 disk** or **installer ISO**, from the
published CentOS Stream 10 image or from a private RHEL 10 build.

> [!NOTE]
> Downloadable QCOW2 and ISO images are not published yet. Until they
> are, create them from the container image with `bootc-image-builder`.
> You need **Linux with rootful Podman** and about **20 GB free**.

## CentOS Stream 10

DeskOS Core for CentOS Stream 10 is published as a bootable container
image. Each published image is **the one that passed the boot and session
checks** for that commit; it is tagged with the commit ID and `latest`.

**1. Pull the image:**

```bash
sudo podman pull quay.io/deskos/deskos-core:latest
```

**2. Make a QCOW2 disk** for a virtual machine:

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    quay.io/deskos/deskos-core:latest
```

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
reboot (see [Updates](architecture.md#updates)).

## RHEL 10 (private builds only)

> [!IMPORTANT]
> DeskOS publishes **only CentOS Stream 10** artifacts. RHEL-based images,
> disks and ISOs must **never be published**: the RHEL EULA forbids public
> redistribution. An organization with RHEL subscriptions builds its own
> and keeps the result inside the organization.

Build on a **registered RHEL 10 host** logged in to `registry.redhat.io`,
as root (the build runs as root).

**1. Get `deskosctl` and the DeskOS resources** of the same release from
[GitHub Releases](https://github.com/deskosproject/deskos-core/releases),
both covered by `SHA256SUMS`. No Go toolchain or git is needed:

```bash
VERSION=v0.5.1
base=https://github.com/deskosproject/deskos-core/releases/download/$VERSION
mkdir deskos && cd deskos
curl -fL -O "$base/deskosctl-$VERSION-linux-amd64" \
     -O "$base/deskos-resources-$VERSION.tar.gz" -O "$base/SHA256SUMS"
sha256sum -c SHA256SUMS
install -D -m 0755 "deskosctl-$VERSION-linux-amd64" bin/deskosctl
tar -xzf "deskos-resources-$VERSION.tar.gz"     # creates ./resources
```

> [!NOTE]
> Use `v0.2.1` or later: it is the first release that ships the resources
> archive, and binaries before `v0.2.0` render a schema step that fails on
> RHEL 10.2.

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
./bin/deskosctl render ./resources ./my-org \
    --workstation deskos-core-rhel10 --output ./dist/deskos-core-rhel10
sudo podman build -t registry.example.internal/deskos/core-rhel10:latest \
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
    registry.example.internal/deskos/core-rhel10:latest
```

Every layer of the build is free of the build host's subscription state
(see [validation status](architecture.md#validation-status)), but the
image, disks and ISOs are **still RHEL derivatives**: push them only to
registries and storage that are private to your organization.
