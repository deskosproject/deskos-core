# Example platforms: AlmaLinux and Fedora

AlmaLinux 10 and Fedora 45 are **not shipped platforms**. They live in their own
example roots so their composition and rendering stay exercised without carrying
their build, boot and publication cost in every pull request.

| Example | Platform resource | Why it is an example |
|---|---|---|
| [`examples/almalinux-10`](../examples/almalinux-10/) | [`platforms/almalinux-10.yaml`](../examples/almalinux-10/platforms/almalinux-10.yaml) | The vendor labels its bootc base experimental. It is useful for EL10 composition work, but it is not a support commitment. |
| [`examples/fedora-45`](../examples/fedora-45/) | [`platforms/fedora-45.yaml`](../examples/fedora-45/platforms/fedora-45.yaml) | Fedora Silverblue's `:45` tag moves daily; a durable published platform needs a mirrored digest first. |

Both examples still compose and render in `ci.yml`. They are **not** in
`resources/platforms/`, `supply-chain.yml`, or the image legs of
`vm-bootcheck.yml`.

## Weekly check

`.github/workflows/examples-weekly.yml` runs every Wednesday at 06:00 UTC and can
be dispatched manually. For each example it:

1. copies the example root to a temporary directory;
2. resolves the vendor's current digest for the example's base tag;
3. replaces the copied Platform's digest, so the test follows the current
   Fedora/GNOME or AlmaLinux base;
4. builds the image, writes a QCOW2, and runs boot, session and first-boot checks;
5. uploads only the evidence, kept for 14 days.

It never publishes, signs or pushes an image. A red weekly run is a signal to
investigate the example; it does not block a pull request or a release.

## Build an example locally

This is the same path the weekly job uses. You need:

- `git`
- Go 1.26.9 (or the `deskosctl` release binary, see below)
- rootful `podman`
- `skopeo` and `qemu-img` for the digest resolution and disk check
- about 20 GB free

### 1. Get the compiler

Either use a release:

```bash
base=https://github.com/deskosproject/deskos-core/releases/latest/download
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl
```

or build from the checkout:

```bash
git clone https://github.com/deskosproject/deskos-core
cd deskos-core
make build
```

### 2. Track the current vendor digest

The committed example carries a documented pin. The Fedora tag moves daily, so
refresh it locally before a manual build:

```bash
# AlmaLinux
digest=$(skopeo inspect --format '{{.Digest}}' \
  docker://quay.io/almalinuxorg/almalinux-bootc:10)
sed -i "s|^    digest: .*|    digest: $digest|" \
  examples/almalinux-10/platforms/almalinux-10.yaml

# Fedora
digest=$(skopeo inspect --format '{{.Digest}}' \
  docker://quay.io/fedora/fedora-silverblue:45)
sed -i "s|^    digest: .*|    digest: $digest|" \
  examples/fedora-45/platforms/fedora-45.yaml
```

Do not commit this refresh unless you are deliberately updating the example pin;
the weekly job does the same replacement in a disposable copy.

### 3. Compose, render and build

AlmaLinux:

```bash
./bin/deskosctl validate ./examples/almalinux-10
./bin/deskosctl render ./examples/almalinux-10 \
  --workstation example-almalinux-10 --output ./dist/alma
sudo podman build -t localhost/example-almalinux-10:test ./dist/alma
```

Fedora:

```bash
./bin/deskosctl validate ./examples/fedora-45
./bin/deskosctl render ./examples/fedora-45 \
  --workstation example-fedora-45 --output ./dist/fedora
sudo podman build -t localhost/example-fedora-45:test ./dist/fedora
```

The Fedora example is deliberately Silverblue, not the minimal
`fedora-bootc` base: Silverblue carries GNOME, GDM, Flatpak, Plymouth and the
vendor desktop, which is what the example is testing.

### 4. Write a QCOW2

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
  --security-opt label=type:unconfined_t \
  -v "$PWD/output:/output" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
  build --type qcow2 --rootfs xfs --no-default-kernel-args \
  --chown "$(id -u):$(id -g)" localhost/example-almalinux-10:test
```

For Fedora, use `--rootfs btrfs` and
`localhost/example-fedora-45:test`. The disk is
`output/qcow2/disk.qcow2`.

Boot it with `virt-install`/`virt-manager` or QEMU. There is no published image
and no update source outside your local test; do not hand this disk to a user.

### 5. What a successful check means

- `validate` and `render` prove the composition and build context.
- `podman build` proves the image builds against the base.
- the QCOW2 boot proves the image starts;
- the session and first-boot checks prove the desktop and wizard come up.

Those are example checks, not a support promise for a published AlmaLinux or
Fedora platform.

## Re-promoting an example platform

1. **Make the base durable first.** For Fedora, copy the exact digest into a
   DeskOS-controlled registry, for example
   `quay.io/deskos/fedora-silverblue:45-YYYYMMDD`, and retain the composes that
   should remain buildable. AlmaLinux has the same retention concern; the mirror
   rule is the same.
2. Move the Platform resource from the example root to
   `resources/platforms/<name>.yaml` and update its comments to say what was
   measured against the mirrored digest.
3. Add a shipped reference workstation under `resources/workstations/` if the
   platform will publish an image.
4. Add it to `resources/README.md`, `docs/platforms.md`, `docs/install.md`,
   `docs/architecture.md`, `docs/roadmap.md` and the platform matrix in
   `README.md`.
5. Re-add its image leg to `vm-bootcheck.yml` and its publication leg to
   `supply-chain.yml`. Do not make a new platform inherit another platform's disk
   path; declare its own.
6. Regenerate goldens with `go test ./internal/compiler -update`, review the diff,
   run `make check`, build the image, boot it, run the session and first-boot
   checks, and only then publish.

The example roots and their Platform resources are intentionally self-contained:
promotion is a move plus CI/docs edits, not a rewrite.
