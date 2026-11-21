# Quick start

Two ways to a running DeskOS machine.

**Download** an image the pipeline already built, scanned and booted. Nothing to
compile and no resources to write: pull it, write a disk, boot it.

**Compile** your own from YAML with the compiler in this repository, then build
and boot it the same way.

Start with the download: it is the fastest way to see the desktop. Compile when
the published layers are not the ones you need.

**You need:** Linux x86_64, rootful Podman, QEMU with KVM, and about 20 GB free.

## Path A: download a published image

### 1. Pick the image

| Platform | Image |
|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10:stable` |
| AlmaLinux 10 | `quay.io/deskos/deskos-core-almalinux-10:stable` |
| Fedora 45 | `quay.io/deskos/deskos-core-fedora-45:stable` |

`:stable` is the digest a promotion verified and recorded; `:latest` is the fast
lane with the newest digest the weekly build passed; `:candidate` is that same
digest before promotion. The table uses `:stable`; pin the digest you verified
for production ([install.md](install.md#option-1-install-the-published-image)).

### 2. Write a QCOW2

```bash
mkdir -p output
sudo podman run \
  --rm \
  --interactive \
  --tty \
  --privileged \
  --pull=missing \
  --security-opt label=type:unconfined_t \
  --volume "$PWD/output:/output" \
  --volume /var/lib/containers/storage:/var/lib/containers/storage \
  quay.io/centos-bootc/bootc-image-builder:latest \
  build \
  --type qcow2 \
  --rootfs xfs \
  quay.io/deskos/deskos-core-centos-stream-10:stable
```

The disk lands at `output/qcow2/disk.qcow2`. Fedora's root filesystem is
`btrfs`: pass `--rootfs btrfs` and the Fedora image
([Fedora 45](fedora.md)). CentOS Stream 10 and AlmaLinux 10 use `xfs`, which is
what the command already asks for.

### 3. Boot it

```bash
sudo qemu-system-x86_64 \
  -enable-kvm \
  -m 4096 \
  -smp 2 \
  -drive file=output/qcow2/disk.qcow2,if=virtio,format=qcow2 \
  -nic user,model=virtio-net-pci
```

Or hand the disk to `virt-manager`/`virt-install` if you prefer.

## What you see

Plymouth's DeskOS splash, then GNOME Initial Setup, because the image ships **no
user account**: the wizard creates the first one, and that account is the
administrator. The desktop is the DeskOS one: the dock, the tray and the
applications the workstation declares.

## Path B: compile your own from YAML

The compiler is `deskosctl`, in the checkout under `bin/`. It validates every
resource, merges the layers into a typed Plan and writes one build context. It
does not use the network.

### 1. Read the workstation

The shipped example is one file,
`examples/centos-stream-10/workstations/example-centos-stream-10.yaml`:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-centos-stream-10
spec:
  displayName: DeskOS Core (CentOS Stream 10)
  platformRef: centos-stream-10
  profiles:
    - deskos-core
    - deskos-dock
    - deskos-gnome
    - deskos-look
    - deskos-reference-apps
    - firefox
    - rhel-family
```

A workstation is one platform plus the profiles it is made of. Each profile
names resources; every kind and its fields are in [resources.md](resources.md).

### 2. Render and build

```bash
./bin/deskosctl validate ./examples/centos-stream-10
./bin/deskosctl render ./examples/centos-stream-10 \
    --workstation example-centos-stream-10 \
    --output ./dist/centos
sudo podman build -t localhost/deskos-centos:test ./dist/centos
```

`render` writes `dist/centos/Containerfile` and the files it copies. Read it and
diff it before you build: it is the whole output of the compiler.

### 3. Write the disk

Run the command in step A2 with `localhost/deskos-centos:test` as the last
argument. A `localhost/` tag is a lab artifact, not an update source: push the
image to a registry the machine can reach before you rely on updates
([install.md](install.md#option-2-build-your-own-then-install-it)).

## Where to go next

- **Install on a machine**, or build an ISO from an image:
  [install.md](install.md) and [installer-iso.md](installer-iso.md).
- **Author your own layers** instead of the example:
  [development.md](development.md) and [resources.md](resources.md).
- **Understand what you booted**: [architecture.md](architecture.md).
