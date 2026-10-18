# Installing DeskOS

How to get a DeskOS **QCOW2 disk** or **installer ISO**, from a CentOS Stream 10
image you build (or pull, once it is published), a private RHEL 10 build, an AlmaLinux 10
build, or a Fedora 44 build. An Ubuntu disk is written with `bootc install to-disk`, and the
repository that owns that platform documents it.

> [!NOTE]
> Downloadable QCOW2 and ISO images are not published yet; the container image
> is. Create an EL or Fedora disk from it with
> `bootc-image-builder`. An Ubuntu disk is written differently, and that platform's own
> repository documents how. You need **Linux with rootful
> Podman** and about **20 GB free**.

## CentOS Stream 10

DeskOS Core for CentOS Stream 10 is published at
`quay.io/deskos/deskos-core-centos-stream-10`: pull it, or build it locally with
`deskosctl render` and `podman build` (see the [README](../README.md)). The
published `:latest` is the newest image that passed the boot and session checks.
Publishing is manual (the `supply-chain` workflow with `publish=true`) and moves
`:latest` and the `:<commit>` tag together; for production, pin the digest you
verified — [architecture.md](architecture.md#validation-status) carries the one
the last dispatch published.

**1. Get the image, then make a QCOW2 disk** for a virtual machine. Pull the
published image, or use the one you built locally — a `localhost/` reference,
which is what the builder takes here:

```bash
# The path available today: the image you built, local to this host.
image=localhost/deskos-core-centos-stream-10:test

mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" "$image"
```

Once the first `publish=true` dispatch runs, take the published image instead,
and resolve `:latest` to a digest first so a publish between the pull and the
build cannot change what you install:

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

> A `localhost/` image is **not an update source**: an installed machine updates
> from the reference it was built from, so anything beyond a lab disk needs a
> published reference those machines can reach.
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
reference it was built from, so an ISO built from a `localhost/` image leaves them
**without an update source**: publish your own reachable reference before handing
out the ISO ([installer-iso.md](installer-iso.md)). Once DeskOS publishes, that
reference is `quay.io/deskos/deskos-core-centos-stream-10:latest`: Core
stages the new image daily, on AC power, and it applies at the next
reboot (see [Updates](operations.md)).

## RHEL 10 (private builds only)

> [!IMPORTANT]
> DeskOS publishes by platform, one repository each. RHEL-based images,
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
example `my-org/workstations/core-rhel-10.yaml`:

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
> `deskos-core` carries the packages and the update policy, and `deskos-gnome` carries
> DeskOS's GNOME defaults (ADR 0016). A workstation that wants them selects both; one
> whose platform ships its own desktop, like Ubuntu, selects only `deskos-core` and
> keeps its vendor's desktop. If you pin a release and select `deskos-core` alone, you
> stop inheriting the session lock, the idle timeout and the software-update policy
> until you add the profile — that is the one breaking change of v0.12.0, and the
> release notes repeat it.

**3. Render and build it.** Tag it with the reference your machines
should update from, in a registry that is **private to your
organization**:

```bash
./bin/deskosctl render ./my-org \
    --workstation deskos-core-rhel-10 --output ./dist/deskos-core-rhel-10
sudo podman build -t registry.example.com/deskos/core-rhel-10:latest \
    ./dist/deskos-core-rhel-10
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
    registry.example.com/deskos/core-rhel-10:latest
```

In the image DeskOS builds and tests, every layer is free of the build
host's subscription state (see [validation status](architecture.md#validation-status));
a build you make is your own to check. The image, disks and ISOs are
**still RHEL derivatives**: push them only to registries and storage that
are private to your organization.

## AlmaLinux 10

AlmaLinux 10 is a public, subscription-free EL10 base — the AlmaLinux project's
own, which that project labels experimental. The image is published at
`quay.io/deskos/deskos-core-almalinux-10`, and CI builds and boots
`example-almalinux-10` and runs the boot, session and first-boot checks on it,
so the steps below are exercised. The platform's facts
are EL10 conventions modeled on RHEL 10 ([platforms.md](platforms.md)).

Get `deskosctl` as in step 1 of the RHEL section above, then render the shipped
example from a checkout of this repository and build it:

```bash
./bin/deskosctl render ./examples/almalinux-10 \
    --workstation example-almalinux-10 --output ./dist/alma
sudo podman build -t localhost/deskos-alma:test ./dist/alma
```

Then write the QCOW2 with the public `bootc-image-builder`, as in the CentOS
Stream 10 steps above.

## Fedora 44

Fedora is a public, subscription-free base published by the Fedora Project
itself, so a build needs no entitlement. DeskOS publishes no Fedora image. Get
`deskosctl` as in step 1 of the RHEL section above, then build the shipped
**reference workstation** from a checkout of this repository:

```bash
./bin/deskosctl render \
    --workstation deskos-core-fedora-44 --output ./dist/fedora-44
sudo podman build -t localhost/deskos-core-fedora-44:test ./dist/fedora-44
```

`examples/fedora-44/` is the minimal example (Core and the family profile
only). Then write the QCOW2 with the public `bootc-image-builder` as in the
CentOS Stream 10 steps above, or `bootc install to-disk` from a compressed OCI
layout, which is the recipe the Ubuntu platform documents in its own repository.

If you publish it, give it its **own repository** —
`quay.io/deskos/deskos-core-fedora-44` — and point the installed machine at that
reference. A shared `:latest` repository would let a Fedora release change under
a machine at `bootc upgrade` ([platforms.md](platforms.md)).

> [!WARNING]
> **This path is proposed, not measured, past the build.** The platform's facts
> are measured against the pinned base (one comment per fact in
> `resources/platforms/fedora-44.yaml`) and the image builds, but DeskOS has
> **not booted** a Fedora disk: there is no boot, session or first-boot evidence
> for this platform. See [platforms.md](platforms.md).

> [!NOTE]
> **Fedora 45 is not offered.** It was a prerelease when it was measured and its
> base enables `updates-testing`, so DeskOS offers 44 only. The 45 evaluation is
> kept as an example-only root that carries its **own** Platform resource —
> [`examples/fedora-45/`](../examples/fedora-45/README.md) — with no
> reference workstation and no image; its platform is a starting point for
> whoever evaluates the next Fedora release, not something to install.

## Ubuntu 26.04

An Ubuntu disk is not built here any more: the platform, its base and its workstations live
in [`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu), and that
repository carries the steps — the image, the compressed OCI layout, `bootc install to-disk`
and the `--target-imgref` that decides what the machine updates from afterwards
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

What this repository still does with a Debian-family platform is compose and render it, and
the tests are what say so.
