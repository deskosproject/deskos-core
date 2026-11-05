# Quick start

The shortest path to a running DeskOS machine: take an image the pipeline
already built, scanned and booted, write a disk from it, and boot the disk. No
resources, no compiler, no build.

**You need:** Linux x86_64 with rootful Podman and QEMU, about 20 GB free, and
KVM for the virtual machine.

## 1. Pick the image

| Platform | Image |
|---|---|
| CentOS Stream 10 | `quay.io/deskos/deskos-core-centos-stream-10:stable` |
| AlmaLinux 10 | `quay.io/deskos/deskos-core-almalinux-10:stable` |
| Fedora 45 | `quay.io/deskos/deskos-core-fedora-45:stable` |

`:stable` is the digest a promotion verified and recorded; `:latest` is the fast
lane with the newest digest the weekly build passed; `:candidate` is that same
digest before promotion. The table uses `:stable`; pin the digest you verified
for production ([install.md](install.md#option-1-install-the-published-image)).

## 2. Write a QCOW2

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

## 3. Boot it

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

## Where to go next

- **Install on a machine**, or build an ISO from the same image:
  [install.md](install.md) and [installer-iso.md](installer-iso.md).
- **Build your own image** from resources instead of taking ours:
  [development.md](development.md).
- **Understand what you booted**: [architecture.md](architecture.md) and the
  [resource reference](resources.md).
