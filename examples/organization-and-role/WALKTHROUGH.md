# Follow the example end to end

One pinned exercise: take this example from resources to a booted UEFI VM, then
inspect and recover it. It needs no founder-only infrastructure: the public
CentOS Stream 10 base, a released `deskosctl`, and a Linux host with rootful
Podman and KVM.

Pinned versions used below:

| Item | Pin |
|---|---|
| `deskosctl` | `v0.18.0`, sha256 `4fdfa82bc003b3f63624ceca0ba72d3dcfa0763f1f1314ff7c3edc1b2f464d57` |
| ISO/QCOW2 builder | `quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b` |
| Platform base | pinned in `resources/platforms/centos-stream-10.yaml`; the Plan records the digest |

**Host:** Linux x86_64, 4 vCPU, 8 GiB RAM, 40 GiB free disk, `/dev/kvm`, UEFI
firmware (`ovmf`), `git`, `curl`, `jq`, and rootful Podman. Secure Boot is off
for the lab VM.

## 1. Get the compiler that renders these resources

```bash
git clone https://github.com/deskosproject/deskos-core
cd deskos-core
git checkout v0.17.0

base=https://github.com/deskosproject/deskos-core/releases/download/v0.17.0
curl -fLO "$base/deskosctl-v0.17.0-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-v0.17.0-linux-amd64 bin/deskosctl
./bin/deskosctl version      # deskosctl v0.17.0 (commit …)
```

The checkout and the binary are the same tag on purpose: the compiler embeds
Core from that release, and the example is rendered against it.

## 2. Validate, then plan

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
```

`validate` is deterministic and offline. `plan.json` is the effective intent
for the CentOS target: Core foundation, the example organization and the DevOps
role, merged by the rules in ADR 0003.

Look at one value and where it came from:

```bash
jq '.artifact.packages[] | select(.name == "buildah") | .provenance' plan.json
```

Expected shape:

```json
[{"resource":"PackageSet/example-containers","profile":"example-devops",
  "layer":"role","source":"organization-and-role/software/example-devops.yaml:1"}]
```

That is the sample provenance explanation: the package, the resource that
declared it, the profile and layer that selected it, and the file and line.
Nothing here is a fleet controller; the Plan is the artifact.

The **declared** SBOM is not an inventory of a built image:

```bash
./bin/deskosctl sbom ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --output declared.cdx.json
jq -r '.bomFormat, (.components | length)' declared.cdx.json
```

## 3. Customize the baseline, the role and the update policy

Do this in your own copy of the example (or a new resource root). Two small,
deliberate edits:

Add a package to the role in `software/example-devops.yaml`, under
`PackageSet/example-containers`:

```yaml
  packages:
    - git
    - podman
    - buildah
    - skopeo
    - htop
```

Add an update policy and select it from the organization profile. Create
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

Add the resource to `profiles/example-organization.yaml`:

```yaml
    - kind: UpdatePolicy
      name: example-updates
```

Then confirm that the higher layer wins and that provenance follows:

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
jq '.artifact.packages[] | select(.name == "htop") | .provenance' plan.json
jq '.artifact.scheduledUpdates[] | select(.kind == "image") |
    {schedule, provenance}' plan.json
```

The image schedule reads `weekly` with both Core's and your policy in the
provenance; the highest layer supplied the value. The package says `role`.

## 4. Render and build the image

```bash
./bin/deskosctl render ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --output ctx
sudo podman build -t localhost/example-devops-centos-stream-10:test ctx
```

For a lab image, `localhost/` is enough. It is **not** an update source: a
machine installed from it tracks a reference that does not exist in a registry.
See section 7 before expecting updates.

## 5. Make a UEFI QCOW2

```bash
mkdir -p out
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./out:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args \
    --chown "$(id -u):$(id -g)" \
    localhost/example-devops-centos-stream-10:test
```

The disk is `out/qcow2/disk.qcow2`; record its hash:

```bash
sha256sum out/qcow2/disk.qcow2
```

## 6. Boot it and create the first user

Use UEFI and a writable copy of the firmware variables. Paths differ by host:
Fedora/RHEL use `/usr/share/edk2/ovmf/`, Ubuntu uses
`/usr/share/OVMF/OVMF_CODE_4M.fd` and `OVMF_VARS_4M.fd`.

```bash
cp /usr/share/edk2/ovmf/OVMF_VARS.fd ./OVMF_VARS.fd
qemu-system-x86_64 -machine q35,accel=kvm -cpu host -m 4096 -smp 4 \
    -drive if=pflash,format=raw,readonly=on,file=/usr/share/edk2/ovmf/OVMF_CODE.fd \
    -drive if=pflash,format=raw,file=./OVMF_VARS.fd \
    -drive if=virtio,format=qcow2,file=out/qcow2/disk.qcow2 \
    -nic user,model=virtio-net-pci -display gtk
```

No account is baked in. GNOME Initial Setup creates the first one; that account
is the administrator. Log in and inspect the machine:

```bash
deskos report        # the plan the image carries
deskos check         # what the declared plan measures, and what it leaves out
bootc status         # booted deployment, digest and followed reference
```

`deskos check` is not an audit of everything the machine has become.
The image's SBOM is CI evidence; `report` says it is not installed.

## 7. Optional: a reachable update source

An image reference must exist in a registry for the installed machine to update.
Use your own registry and credentials; the names here are placeholders.

```bash
registry=registry.example.com/example/devops
sudo podman login "$registry"              # your credential, not one in this repo
sudo podman tag localhost/example-devops-centos-stream-10:test "$registry:v1"
sudo podman push "$registry:v1"
```

Build the QCOW2 from `$registry:v1` instead of `localhost/…`. The installed
machine then follows `$registry:v1`; changing the tag later is how the next
version is offered.

To exercise A → B → A, add another package (for example `tmux`), rebuild the
image, and push it to **the same tag**:

```bash
# after rebuilding with the second change
sudo podman tag localhost/example-devops-centos-stream-10:test "$registry:v1"
sudo podman push "$registry:v1"

# on the installed machine
sudo deskos update          # stages the next image; images only
sudo reboot
bootc status                # the booted digest is B; A is the rollback
sudo bootc rollback
sudo reboot
bootc status                # A again
```

`deskos update` does not update Flatpaks, firmware or user-level tools. System
Flatpaks have their own service, and `mise upgrade` (or another user-level
manager) runs as the person, never under `sudo`.

A rollback reverts image-owned `/etc` files and keeps local `/etc` edits and
`/var`, but it does not undo data a newer version already migrated. Restore
application data from your own backup.

## 8. Trust, signatures and the lab boundary

- `TrustAnchor/example-root-ca` makes the organization's **services** trusted
  for TLS. It is not image authenticity.
- A published Core image is signed and attested; verify it with the commands in
  [supply-chain.md](../../docs/supply-chain.md#verify-a-digest). An image you
  built under `localhost/` is not signed, and this walkthrough does not claim it
  is.
- A local `localhost/` reference is a lab artifact: without pushing it, the
  installed machine has no update source. A private registry makes updates
  reachable but remains your trust decision.

## 9. What this exercise exposed

- The two-command update surface is deliberate: image, system Flatpaks and
  user-level tools are separate; `deskos update` is image-only.
- The example organization had no update policy of its own; overriding one
  layer is enough, and `plan` shows both provenance entries.
- A lab image is not an update source; the registry section is not optional for
  testing updates or rollback.
- This exercise does not test bare metal, Secure Boot, or a non-booting
  candidate; those are outside it and recorded where they are tested.
