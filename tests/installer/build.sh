#!/usr/bin/env bash
# Build the test-only installer ISO from an image reference.
#
# Needs Linux with rootful Podman. The ISO lands in
# tests/installer/output/bootiso/install.iso. It is not published: the
# published ISO is built from docs/installer-iso.md and keeps storage
# interactive.
set -euo pipefail

if [ $# -ne 1 ]; then
    echo "usage: $0 IMAGE_REF" >&2
    exit 2
fi

image=$1
here=$(cd "$(dirname "$0")" && pwd)

mkdir -p "$here/output"
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v "$here/output:/output" \
    -v "$here/config.toml:/config.toml:ro" \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type anaconda-iso --config /config.toml \
    --chown "$(id -u):$(id -g)" "$image"
