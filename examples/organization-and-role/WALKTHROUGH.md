# Follow the example end to end

One pinned exercise: resources → booted UEFI VM → inspect → update and roll back.
No founder-only infrastructure: the public CentOS Stream 10 base, a released
`deskosctl`, rootful Podman and KVM.

| Item | Pin |
|---|---|
| `deskosctl` | `v0.18.0`, sha256 `4fdfa82bc003b3f63624ceca0ba72d3dcfa0763f1f1314ff7c3edc1b2f464d57` |
| Disk builder | `quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b` |
| Platform base | pinned in `resources/platforms/centos-stream-10.yaml`; the Plan records the digest |

**Host:** Linux x86_64, 4 vCPU, 8 GiB RAM, 40 GiB free, `/dev/kvm`, OVMF, `git`,
`curl`, `jq` and rootful Podman. Secure Boot stays off for the lab VM.

## 1. Get the compiler that renders these resources

```bash
git clone https://github.com/deskosproject/deskos-core
cd deskos-core
git checkout v0.18.0

base=https://github.com/deskosproject/deskos-core/releases/download/v0.18.0
curl -fLO "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl
./bin/deskosctl version      # deskosctl v0.18.0 (commit …)
```

The checkout and the binary are the same tag on purpose: the compiler embeds Core
from that release, and the example is rendered against it.

## 2. Validate, then plan

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
```

`validate` is deterministic and offline. `plan.json` is the effective intent for
the CentOS target, merged by [ADR 0003](../../docs/adr/0003-explicit-composition-precedence.md).
Look at one value and where it came from:

```bash
jq '.artifact.packages[] | select(.name == "buildah") | .provenance' plan.json
```

```json
[{"resource":"PackageSet/example-containers","profile":"example-devops",
  "layer":"role","source":"organization-and-role/software/example-devops.yaml:1"}]
```

The **declared** SBOM is not an inventory of a built image:

```bash
./bin/deskosctl sbom ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --output declared.cdx.json
jq -r '.bomFormat, (.components | length)' declared.cdx.json
```

## 3. Customize baseline, role and update policy

In your own copy: add a package to `PackageSet/example-containers` in
`software/example-devops.yaml` (`- htop`), then create
`system/example-updates.yaml`:

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: UpdatePolicy
metadata:
  name: example-updates
spec:
  image:
    schedule: weekly
```

and select it from `profiles/example-organization.yaml`:

```yaml
    - kind: UpdatePolicy
      name: example-updates
```

The higher layer now wins, and provenance follows:

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
jq '.artifact.packages[] | select(.name == "htop") | .provenance' plan.json
jq '.artifact.scheduledUpdates[] | select(.kind == "image") |
    {schedule, provenance}' plan.json
```

The schedule reads `weekly` with both policies in provenance; the package says
`role`.

## 4. Render and build

```bash
./bin/deskosctl render ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --output ctx
sudo podman build -t localhost/example-devops-centos-stream-10:test ctx
```

`localhost/` is enough for a lab image and is **not** an update source: a machine
installed from it follows a reference no registry has. See step 7.

## 5. Make a UEFI QCOW2

```bash
mkdir -p out
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./out:/output -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args --chown "$(id -u):$(id -g)" \
    localhost/example-devops-centos-stream-10:test
```

The disk is `out/qcow2/disk.qcow2`; record `sha256sum out/qcow2/disk.qcow2`.

## 6. Boot it and create the first user

UEFI firmware paths differ by host: Fedora and RHEL use
`/usr/share/edk2/ovmf/`, Ubuntu `/usr/share/OVMF/OVMF_CODE_4M.fd`.

```bash
cp /usr/share/edk2/ovmf/OVMF_VARS.fd ./OVMF_VARS.fd
qemu-system-x86_64 -machine q35,accel=kvm -cpu host -m 4096 -smp 4 \
    -drive if=pflash,format=raw,readonly=on,file=/usr/share/edk2/ovmf/OVMF_CODE.fd \
    -drive if=pflash,format=raw,file=./OVMF_VARS.fd \
    -drive if=virtio,format=qcow2,file=out/qcow2/disk.qcow2 \
    -nic user,model=virtio-net-pci -display gtk
```

No account is baked in: GNOME Initial Setup creates the first one, which is the
administrator. Then inspect the machine:

```bash
deskos report        # the plan the image carries
deskos check         # what the declared plan measures, and what it leaves out
bootc status         # booted deployment, digest and followed reference
```

`deskos check` is not an audit of everything the machine has become, and `report`
says the SBOM is CI evidence, not an installed inventory.

## 7. Optional: a reachable update source

Use your own registry and credentials; the names are placeholders.

```bash
registry=registry.example.com/example/devops
sudo podman login "$registry"
sudo podman tag localhost/example-devops-centos-stream-10:test "$registry:v1"
sudo podman push "$registry:v1"
```

Build the QCOW2 from `$registry:v1` instead of `localhost/…`; the machine then
follows that reference, and changing the tag offers the next version.

For A → B → A, change the image (for example add `tmux`), push it to **the same
tag**, and on the machine:

```bash
sudo deskos update          # stages the next image; images only
sudo reboot
bootc status                # booted digest is B; A is the rollback
sudo bootc rollback
sudo reboot
bootc status                # A again
```

`deskos update` updates no Flatpaks, firmware or user tools: system Flatpaks have
their own service, and a user-level manager runs as the person, never under
`sudo`. A rollback reverts image-owned `/etc` files and keeps local edits and
`/var`, but does not undo data a newer version migrated.

## 8. Trust and the lab boundary

- `TrustAnchor/example-root-ca` makes the organization's **services** trusted for
  TLS. It is not image authenticity.
- A published Core image is signed and attested; verify with
  [supply-chain.md](../../docs/supply-chain.md#signing-and-provenance). An image
  built under `localhost/` is not signed, and this walkthrough does not claim it.
- A `localhost/` reference is a lab artifact. A private registry makes updates
  reachable and is still your trust decision.

## 9. What this exercise exposed

- Image, system Flatpaks and user tools are three update surfaces on purpose.
- The example organization had no update policy; one higher-layer override is
  enough, and `plan` shows both provenance entries.
- A lab image is not an update source: the registry step is required to test
  updates or rollback.
- Bare metal, Secure Boot and a non-booting candidate are outside this exercise
  and are recorded where they are tested.
