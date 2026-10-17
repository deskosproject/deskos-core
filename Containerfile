# The DeskOS tool image: deskosctl, the compiler, as a container.
#
# The binary is static and CGO-free and compilation needs no network, so the
# runtime stage is scratch. The compiler materializes the embedded DeskOS Core
# into a temporary directory on every run: /tmp is not writable in a rootless
# container, but the runtime mounts /dev/shm as a world-writable tmpfs, so
# TMPDIR points there.
#
# Build it from the repository root:
#
#   podman build -t quay.io/deskos/deskosctl:dev \
#       --build-arg VERSION=dev --build-arg COMMIT="$(git rev-parse HEAD)" .
#
# Run it with the resource root and the output directory mounted. The two extra
# flags are host facts, not the image's: --userns=keep-id maps your uid so the
# tool can write the output, and :z relabels the writable mount on an SELinux
# host (rootful Podman already runs as root and needs neither):
#
#   podman run --rm --userns=keep-id \
#       -v "$PWD":/src:ro -v "$PWD/out":/out:z \
#       quay.io/deskos/deskosctl:dev render /src --workstation X --output /out

FROM docker.io/library/golang:1.26 AS build

ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /deskosctl ./cmd/deskosctl

FROM scratch

COPY --from=build /deskosctl /deskosctl

# The embedded Core is materialized per run; /dev/shm is a writable tmpfs.
ENV TMPDIR=/dev/shm

ENTRYPOINT ["/deskosctl"]
