# aarch64: an inventory, and a decline with conditions

P3-04 asks whether DeskOS can build and boot on aarch64, and accepts "declines"
as an answer. This is the inventory that supports the decline, and the conditions
that would turn it into work.

## What the inventory found

| Piece | State today | What aarch64 would need |
|---|---|---|
| Base images | `quay.io/centos-bootc/centos-bootc:stream10` and `bootc-image-builder` are multi-arch: `amd64`, `arm64`, `ppc64le`, `s390x` (measured 2026-10-30) | nothing |
| `deskosctl` and `deskos` | released for `linux/amd64` only; the release workflow sets `GOARCH=amd64` and publishes two amd64 assets | an arm64 build, and an arm64 runner to test it |
| The runtime pin | `resources/software/deskos.yaml` names `deskos-v0.18.0-linux-amd64` | a per-architecture pin, which the model does not have |
| Platform resources | every `Platform` declares `architectures: [x86_64]` | an arm64 platform resource, and the fields it would change |
| Example artifacts | `openshift-client` pins `openshift-client-linux-amd64-rhel9` | an arm64 asset, or an example that does not use it |
| Runners | 23 jobs, all `ubuntu-24.04` or `ubuntu-latest`, all x86_64 | arm64 runners, or QEMU emulation for the VM checks |
| Boot harness | QEMU with x86_64 OVMF firmware paths | aarch64 firmware (AAVMF), and a boot check that knows the difference |
| Hardware | none | a machine to prove the firmware and the disk |

Everything that is architecture-neutral already works: the resources, the
composition, the Plan, the container build (Buildah and Podman build for the target
platform), and the generated files.

## The two real blockers

1. **The runtime pin has no architecture.** `BinaryArtifact` names one URL and one
   checksum, and the runtime binary is architecture-specific. An arm64 workstation
   would either fail to find its binary or carry an amd64 one. Solving it means a
   per-architecture selection somewhere: in the platform, in the artifact, or in
   the release (one multi-arch index instead of two files). That is a model
   decision, not a build flag.
2. **Nothing would test it.** An arm64 image built on an x86 runner can be built
   and inspected, but not booted at speed: QEMU TCG can boot it slowly, and the
   session check needs a session. An arm64 hosted runner plus aarch64 firmware in
   the harness is the minimum for the checks this repository already runs.

## Decision

**Declined for now, with conditions.** No user has asked for it, no owner has
volunteered, and the two blockers are a model decision and a test platform. Adding
an architecture without either would produce an image nobody boots.

It becomes work when all four exist:

- a named user with a machine to install on, not a hypothesis;
- an owner for the arm64 platform resource and its matrix;
- a decision on per-architecture artifact pins, written as an ADR;
- an arm64 runner and aarch64 firmware in the VM harness, so the boot and session
  checks run as they do on x86_64.

The order matters: the artifact decision first, because it changes the model, and
the runner second, because without it the first cannot be proved.

## What would not need a decision

- **Cross-compiling the two binaries** is a line in the release workflow, and it
  is worth doing when someone asks, even before the rest.
- **An arm64 example platform** can wait; an example that cannot be booted is a
  configuration file, not evidence.
