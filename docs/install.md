# Installing DeskOS

Install a published image, or build your own and install that. You need **Linux
with rootful Podman** and about **20 GB free**.

> [!NOTE]
> The **container images** are published, and `bootc-image-builder` turns one into
> a disk on your machine. A **CentOS Stream 10 preview ISO** is also published at
> <https://download.deskos.org/>; verify its `SHA256SUMS` before booting it. No
> QCOW2 or RHEL ISO is published by this repository.

## Install a published image

One platform publishes an image that the gate built, scanned and booted:

| Platform | Image |
|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10` |

Each has `:latest` (the newest image that passed the boot and session checks) and
a `:<commit>` tag, moved together by the weekly gate (or a `publish=true`
dispatch). Beyond a lab, **pin the verified digest**:
[architecture.md](architecture.md#validation-status) has it and its run.

### Write a QCOW2 disk

Resolve `:latest` to a digest first, so a publish between the pull and the build
cannot change what you install:

```bash
image=quay.io/deskos/deskos-core-centos-stream-10:latest
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
    "quay.io/deskos/deskos-core-centos-stream-10@$digest"
```

The disk is `output/qcow2/disk.qcow2` (*10.5 GiB virtual* by default), with no
user account baked in: GNOME Initial Setup creates the first one at first boot.

- `--no-default-kernel-args` keeps the image's own boot arguments (quiet
  graphical splash) instead of the builder's serial console.
- The CI pins this builder by digest, and `:latest` has moved past it. To build
  exactly what CI built, replace the builder reference with
  `quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b`.

### Or make an installer ISO

> [!WARNING]
> The installer asks for language, keyboard, timezone, network (optional), the
> disk and LUKS, and then erases **the disk you select** — read the destination
> screen. LUKS is opt-in. The test-only ISO under `tests/installer/` is different:
> its kickstart runs `clearpart --all`, which erases every disk the installer
> sees. [installer-iso.md](installer-iso.md) explains both.

```bash
image=quay.io/deskos/deskos-core-centos-stream-10:latest
sudo podman pull "$image"
digest="$(sudo podman image inspect --format '{{.Digest}}' "$image")"

mkdir -p output
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
xconfig --startxonboot
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

The ISO is `output/bootiso/install.iso`. `anaconda-iso` is the current name for
this output: older builder releases called it `iso`. **The ISO bakes in the
reference it is built from, and that is what the installed machine follows for
every later `bootc upgrade`.**

## Build your own, then install it

For a resource root of your own: author the workstation, render it, build it, and
install the result with the steps above. It needs **git**. RHEL is the same flow on
an entitled base, in its own section below.

1. Get `deskosctl` from
   [GitHub Releases](https://github.com/deskosproject/deskos-core/releases):
   version-less asset names track the latest release, a `vX.Y.Z` name pins one,
   and no Go toolchain is needed. The checkout and the download commands are in
   [development.md](development.md#releases).

2. Render a shipped example, or a resource root of your own, and build it:

   ```bash
   ./bin/deskosctl render ./examples/centos-stream-10 \
       --workstation example-centos-stream-10 --output ./dist/centos
   sudo podman build -t localhost/deskos-centos:test ./dist/centos
   ```

3. Write the disk with the `bootc-image-builder` command above, with
   `image=localhost/deskos-centos:test` and `"$image"` as its last argument. A
   `localhost/` image is **not an update source**: an installed machine updates
   from the reference it was built from, so anything beyond a lab disk needs a
   published reference those machines can reach.

## RHEL 10 (private builds only)

RHEL 10 is never published **by DeskOS**: the EULA forbids redistributing it, so
an organization with a subscription builds its own and pushes it only to a
registry private to that organization. You need a **RHEL 10 host or build machine
registered with your subscription and logged in to `registry.redhat.io`** (the
base and the builder are entitled content), plus rootful Podman.

1. Get `deskosctl` as in step 1 above.

2. Declare a RHEL 10 workstation in a resource root of your own, for example
   `my-org/workstations/core-rhel-10.yaml`:

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

   > [!IMPORTANT]
   > `deskos-core` carries the packages and the update policy, and `deskos-gnome`
   > carries DeskOS's GNOME defaults (ADR 0016). A workstation that wants them
   > selects both; one whose platform keeps its vendor's desktop selects only
   > `deskos-core`.

3. Render and build it, tagged with the reference your machines should update
   from, in a registry that is **private to your organization**:

   ```bash
   ./bin/deskosctl render ./my-org \
       --workstation deskos-core-rhel-10 --output ./dist/deskos-core-rhel-10
   sudo podman build -t registry.example.com/deskos/core-rhel-10:latest \
       ./dist/deskos-core-rhel-10
   ```

4. Make the QCOW2 (or the ISO) with the `bootc-image-builder` command above,
   using the **RHEL** builder
   `registry.redhat.io/rhel10/bootc-image-builder@sha256:3087a30a2891cd63505b5585f7f154a4eaaa8221aa1afcd50ae80cbecfdc5f97`
   and the image `registry.example.com/deskos/core-rhel-10:latest`.

In the image DeskOS builds and tests, every layer is free of the build host's
subscription state (see [validation status](architecture.md#validation-status));
a build you make is your own to check. The image, disks and ISOs are **still RHEL
derivatives**: push them only to private registries and storage.
