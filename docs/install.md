# Installing DeskOS

Two paths, and you only need the first: **install a published image**, or
**build your own** and then install that. Most people never need the second.

> [!NOTE]
> You need **Linux with rootful Podman** and about **20 GB free**. No
> downloadable QCOW2 or ISO files are published — the **container images** are,
> and `bootc-image-builder` turns one into a disk on your machine.

## Install a published image

Two platforms publish an image that the gate built, scanned and booted:

| Platform | Image |
|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10` |
| AlmaLinux 10 | `quay.io/deskos/deskos-core-almalinux-10` |

Each has `:latest` (the newest image that passed the boot and session checks) and
a `:<commit>` tag, moved together by a manual `publish=true` dispatch. For
anything beyond a lab, **pin the digest you verified**:
[architecture.md](architecture.md#validation-status) carries the digest and the
run that published it.

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

The disk is `output/qcow2/disk.qcow2` (*10.5 GiB virtual* by default).

- **No user account is baked in:** GNOME Initial Setup creates the first one at
  first boot.
- `--no-default-kernel-args` keeps the image's own boot arguments (quiet
  graphical splash) instead of the builder's serial console.
- The CI pins this builder by digest for reproducibility, and `:latest` has
  already moved past it. To build exactly what CI built, replace the builder
  reference with
  `quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b`.

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

For AlmaLinux 10, use the same command with its image reference.

### Or make an installer ISO

> [!WARNING]
> **An installer ISO erases every disk it is pointed at.** Read
> [installer-iso.md](installer-iso.md) before you build one: it carries the
> kickstart DeskOS needs and the warning in full.

Use `--type anaconda-iso` with that kickstart in `config.toml`.

**The ISO bakes in the reference it is built from, and that is what the installed
machine follows for every later `bootc upgrade`.** So build the ISO from a
*floating* reference you control — `quay.io/deskos/deskos-core-centos-stream-10:latest`
for a DeskOS image — and not from a digest: a digest reference never moves, and
the machine would stop receiving updates. An ISO built from a `localhost/` image
leaves the machine with **no update source** at all. Core stages the published
image daily, on AC power, and it applies at the next reboot
([Updates](operations.md)).

## Build your own, then install it

Everything below compiles an image yourself. It is the path for an organization
that authors its own workstation, for RHEL (which DeskOS never publishes), and
for anyone who wants to understand the whole pipeline. It needs **git**, because
the example resources it renders come from a checkout.

**1. Get `deskosctl`** from
[GitHub Releases](https://github.com/deskosproject/deskos-core/releases). The
version-less asset names track the latest release; use a `vX.Y.Z` name to pin
one. No Go toolchain is needed:

```bash
# The checkout, and the tool beside it, so every command below is relative.
git clone https://github.com/deskosproject/deskos-core
cd deskos-core

base=https://github.com/deskosproject/deskos-core/releases/latest/download
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl
```

**2. Render and build.** Render a shipped example, or a resource root of your own,
and build it:

```bash
./bin/deskosctl render ./examples/almalinux-10 \
    --workstation example-almalinux-10 --output ./dist/alma
sudo podman build -t localhost/deskos-alma:test ./dist/alma
```

**3. Write the disk** with the same `bootc-image-builder` command as above,
passing your `localhost/` image:

```bash
image=localhost/deskos-alma:test

mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" "$image"
```

> A `localhost/` image is **not an update source**: an installed machine updates
> from the reference it was built from, so anything beyond a lab disk needs a
> published reference those machines can reach.

The complete pipeline, including the determinism guarantees and the CI flow, is
in [development.md](development.md#build-the-image-yourself).

## RHEL 10 (private builds only)

RHEL 10 is never published **by DeskOS**: the EULA forbids redistributing it, so
an organization with a subscription builds its own and pushes it only to a
registry private to that organization.

You need a **RHEL 10 host or build machine registered with your subscription and
logged in to `registry.redhat.io`** — the base and the RHEL builder are entitled
content — plus rootful Podman.

**1. Get `deskosctl`** as in step 1 above.

**2. Declare a RHEL 10 workstation** in a resource root of your own, for example
`my-org/workstations/core-rhel-10.yaml`:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: deskos-core-rhel-10
spec:
  displayName: DeskOS Core (RHEL 10)
  platformRef: rhel-10
  profiles:
    - deskos-core        # packages and the update policy
    - deskos-gnome       # DeskOS's GNOME defaults
```

> [!IMPORTANT]
> `deskos-core` carries the packages and the update policy, and `deskos-gnome`
> carries DeskOS's GNOME defaults (ADR 0016). A workstation that wants them
> selects both; one whose platform ships its own desktop, like Ubuntu, selects
> only `deskos-core` and keeps its vendor's desktop.

**3. Render and build it.** Tag it with the reference your machines should update
from, in a registry that is **private to your organization**:

```bash
./bin/deskosctl render ./my-org \
    --workstation deskos-core-rhel-10 --output ./dist/deskos-core-rhel-10
sudo podman build -t registry.example.com/deskos/core-rhel-10:latest \
    ./dist/deskos-core-rhel-10
```

**4. Make the QCOW2** (or the ISO) with the **RHEL** builder:

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    registry.redhat.io/rhel10/bootc-image-builder@sha256:3087a30a2891cd63505b5585f7f154a4eaaa8221aa1afcd50ae80cbecfdc5f97 \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    registry.example.com/deskos/core-rhel-10:latest
```

In the image DeskOS builds and tests, every layer is free of the build host's
subscription state (see [validation status](architecture.md#validation-status));
a build you make is your own to check. The image, disks and ISOs are **still RHEL
derivatives**: push them only to registries and storage that are private to your
organization.

## Fedora 45

Fedora is a public, subscription-free base published by the Fedora Project
itself, so a build needs no entitlement. **DeskOS publishes no Fedora image.**
Get `deskosctl` as in step 1 above, then build the shipped reference workstation
from a checkout of this repository:

```bash
./bin/deskosctl render \
    --workstation deskos-core-fedora-45 --output ./dist/fedora-45
sudo podman build -t localhost/deskos-core-fedora-45:test ./dist/fedora-45
```

`examples/fedora-45/` is the minimal example (Core and the family profile only).
Then write the QCOW2 as above.

If you publish it, give it its **own repository** —
`quay.io/deskos/deskos-core-fedora-45` — and point the installed machine at that
reference. A shared `:latest` repository would let a Fedora release change under
a machine at `bootc upgrade` ([platforms.md](platforms.md)).

> [!WARNING]
> **This path is proposed, not measured, past the build.** The platform's facts
> are measured against the pinned base (one comment per fact in
> `resources/platforms/fedora-45.yaml`) and the image builds; CI also boots a
> QCOW2 of the example and runs the session and first-boot checks. What has **not**
> happened is an installation on hardware: there is no published Fedora image and
> no Fedora disk has been installed by a user. See [platforms.md](platforms.md).

> [!NOTE]
> **The pin is a prerelease compose.** The `:45` tag still resolves to
> `45.20261019.n.0`, whose base enables Fedora's `updates-testing` repository, so
> a build can take packages from testing. `resources/platforms/fedora-45.yaml`
> says which compose the pin is and what was measured against it; the released
> compose is re-pinned when it appears. Fedora 44 is retired: DeskOS ships one
> Fedora release at a time ([roadmap.md](roadmap.md)).

## Ubuntu 26.04

An Ubuntu disk is not built here any more: the platform, its base and its
workstations live in
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu),
and that repository carries the steps — the image, the compressed OCI layout,
`bootc install to-disk` and the `--target-imgref` that decides what the machine
updates from afterwards
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

What this repository still does with a Debian-family platform is compose and
render it, and the tests are what say so.
