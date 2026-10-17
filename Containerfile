# The DeskOS tool image: deskosctl, the compiler, as a container.
#
# The binary is static and CGO-free and compilation needs no network, so the
# runtime stage is scratch. The compiler materializes the embedded DeskOS Core
# into a temporary directory on every run; /tmp is not writable in a rootless
# container, so TMPDIR points at /dev/shm, the world-writable tmpfs Podman
# mounts by default. That default is what the image relies on: a runtime that
# does not provide /dev/shm should pass `-e TMPDIR=<a writable path>`.
#
# Build it from the repository root:
#
#   podman build -t quay.io/deskos/deskosctl:dev \
#       --build-arg VERSION=dev --build-arg COMMIT="$(git rev-parse HEAD)" .
#
# The embedded Core travels inside the image, so a workstation that ships with
# DeskOS Core renders with only the output directory mounted. The two extra
# flags are host facts, not the image's: --userns=keep-id maps your uid so the
# tool can write the output, and :z relabels the mount — an SELinux host refuses
# an unrelabeled root even to read it. Your own resources go in as an extra
# root; it must add resources, not repeat what the embedded Core defines:
#
#   mkdir -p out
#   podman run --rm --userns=keep-id -v "$PWD/out":/out:z \
#       quay.io/deskos/deskosctl:dev render \
#       --workstation deskos-core-centos10 --output /out

# Pinned by the tag's index digest so the toolchain is the one that was tested;
# bump the tag and the digest together.
FROM docker.io/library/golang:1.26@sha256:37cfcca63222ec499bec5e8ad7aad52dbf7529db36d5812b8eae2f0621aa92d6 AS build

ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /deskosctl ./cmd/deskosctl

FROM scratch

COPY --from=build /deskosctl /deskosctl

# The embedded Core is materialized per run; /dev/shm is the default writable
# tmpfs. Override with `-e TMPDIR=…` on a runtime that does not provide it.
ENV TMPDIR=/dev/shm

ENTRYPOINT ["/deskosctl"]
