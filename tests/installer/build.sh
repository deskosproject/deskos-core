#!/usr/bin/env bash
# Build the test-only installer ISO from an image reference.
#
# Needs Linux with rootful Podman. The ISO lands in
# tests/installer/output/bootiso/install.iso. It is not published: the
# published ISO is built from docs/installer-iso.md and keeps storage
# interactive.
#
# The second argument is the image's root filesystem (xfs or btrfs). Fedora
# Silverblue is btrfs; bootc cannot always infer it from the image.
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "usage: $0 IMAGE_REF [ROOTFS]" >&2
    exit 2
fi

image=$1
rootfs=${2:-}
here=$(cd "$(dirname "$0")" && pwd)
rootfs_arg=()
[ -n "$rootfs" ] && rootfs_arg=(--rootfs "$rootfs")

mkdir -p "$here/output"
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v "$here/output:/output" \
    -v "$here/config.toml:/config.toml:ro" \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type anaconda-iso --config /config.toml "${rootfs_arg[@]}" \
    --chown "$(id -u):$(id -g)" "$image"
