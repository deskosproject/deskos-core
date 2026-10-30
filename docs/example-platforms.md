# Example platform: AlmaLinux 10

AlmaLinux 10 is not a shipped platform. Its example root keeps composition and
rendering exercised without its build, boot and publication cost in every pull
request.

| Example | Platform resource | Why it is an example |
|---|---|---|
| [`examples/almalinux-10`](../examples/almalinux-10/) | [`platforms/almalinux-10.yaml`](../examples/almalinux-10/platforms/almalinux-10.yaml) | The vendor labels its bootc base experimental. It is useful for EL10 composition work, but it is not a support commitment. |

It composes and renders in `ci.yml`; it is not in `resources/platforms/`,
`supply-chain.yml`, or the image legs of `vm-bootcheck.yml`.

Fedora 45 was an example until 2026-10-30 and is now a shipped platform: its
mirrored base, pin and status are in [Fedora 45](fedora.md).

## Weekly check

`.github/workflows/examples-weekly.yml` runs every Wednesday at 06:00 UTC and can
be dispatched. For the example it copies the root, resolves the vendor's current
digest, replaces the copied Platform's digest, builds the image, writes a QCOW2,
runs the boot, session and first-boot checks, and uploads the results for 14 days.

It never publishes, signs or pushes an image, and a red weekly run blocks no pull
request or release.

## Build an example locally

You need `git`, Go 1.26.9 or the `deskosctl` release binary, rootful `podman`,
`skopeo`, `qemu-img` and about 20 GB free.

### 1. Get the compiler

Get `deskosctl` as in [install.md](install.md#option-2-build-your-own-then-install-it), or
run `make build` in the checkout.

### 2. Track the current vendor digest

The committed example carries a documented pin; refresh it locally before a manual
build.

```bash
digest=$(skopeo inspect --format '{{.Digest}}' \
  docker://quay.io/almalinuxorg/almalinux-bootc:10)
sed -i "s|^    digest: .*|    digest: $digest|" \
  examples/almalinux-10/platforms/almalinux-10.yaml
```

### 3. Compose, render and build

AlmaLinux:

```bash
./bin/deskosctl validate ./examples/almalinux-10
./bin/deskosctl render ./examples/almalinux-10 \
  --workstation example-almalinux-10 --output ./dist/alma
sudo podman build -t localhost/example-almalinux-10:test ./dist/alma
```

### 4. Write a QCOW2

```bash
mkdir -p output
sudo podman run --rm -it --privileged --pull=missing \
  --security-opt label=type:unconfined_t \
  -v "$PWD/output:/output" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  quay.io/centos-bootc/bootc-image-builder:latest \
  build --type qcow2 --rootfs xfs --no-default-kernel-args \
  --chown "$(id -u):$(id -g)" localhost/example-almalinux-10:test
```

The disk is `output/qcow2/disk.qcow2`; boot it with `virt-install`/`virt-manager`
or QEMU. There is no published image and no update source outside your local test.
Fedora's own disk is `--rootfs btrfs` on `deskos-core-fedora-45`, documented in
[Fedora 45](fedora.md).

### 5. What a successful check means

`validate` and `render` prove the composition and build context, `podman build` the
build against the base, the QCOW2 boot that the image starts, and the session and
first-boot checks the desktop and wizard. These are example checks, not a support
promise.

## Re-promoting an example platform

1. **Make the base durable first.** For Fedora, copy the exact digest into a
   DeskOS-controlled registry, for example
   `quay.io/deskos/fedora-silverblue:45-YYYYMMDD`, and retain the composes that
   should remain buildable. AlmaLinux has the same retention concern.
2. Move the Platform resource from the example root to
   `resources/platforms/<name>.yaml` and update its comments to say what was
   measured against the mirrored digest.
3. Add a shipped reference workstation under `resources/workstations/` if the
   platform will publish an image.
4. Add it to `resources/README.md`, `docs/platforms.md`, `docs/install.md`,
   `docs/architecture.md`, `docs/roadmap.md` and the `README.md` platform matrix.
5. Re-add its image leg to `vm-bootcheck.yml` and its publication leg to
   `supply-chain.yml`. Do not make a new platform inherit another platform's disk
   path; declare its own.
6. Regenerate goldens with `go test ./internal/compiler -update`, review the diff,
   run `make check`, build and boot the image, run the session and first-boot
   checks, and only then publish.
