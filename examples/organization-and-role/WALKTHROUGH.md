# Follow the example end to end

One pinned exercise: resources → booted UEFI VM → inspect → update and roll
back.

| Item | Pin |
|---|---|
| `deskosctl` | `v0.18.0`, sha256 `4fdfa82bc003b3f63624ceca0ba72d3dcfa0763f1f1314ff7c3edc1b2f464d57` |
| Disk builder | `quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b` |
| Platform base | pinned in `resources/platforms/centos-stream-10.yaml`; the Plan records the digest |

**Host:** Linux x86_64, 4 vCPU, 8 GiB RAM, 40 GiB free, `/dev/kvm`, OVMF, `git`,
`curl`, `jq`, rootful Podman; Secure Boot off.

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

## 2. Validate, then plan

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
```

`validate` is offline and deterministic; `plan.json` is the effective intent,
merged by [ADR 0003](../../docs/adr/0003-explicit-composition-precedence.md):

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

Add `- htop` to `PackageSet/example-containers` in
`software/example-devops.yaml`, then create `system/example-updates.yaml`:

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: UpdatePolicy
metadata:
  name: example-updates
spec:
  image:
    schedule: weekly
```

```yaml
# profiles/example-organization.yaml
    - kind: UpdatePolicy
      name: example-updates
```

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --format json > plan.json
jq '.artifact.packages[] | select(.name == "htop") | .provenance' plan.json
jq '.artifact.scheduledUpdates[] | select(.kind == "image") |
    {schedule, provenance}' plan.json
```

The schedule reads `weekly`; the package says `role`.

## 4. Render and build

```bash
./bin/deskosctl render ./examples/organization-and-role \
    --workstation example-devops-centos-stream-10 --output ctx
sudo podman build -t localhost/example-devops-centos-stream-10:test ctx
```

`localhost/` is enough for a lab image, **not** an update source. See step 7.

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

UEFI paths: Fedora and RHEL `/usr/share/edk2/ovmf/`, Ubuntu
`/usr/share/OVMF/OVMF_CODE_4M.fd`.

```bash
cp /usr/share/edk2/ovmf/OVMF_VARS.fd ./OVMF_VARS.fd
qemu-system-x86_64 -machine q35,accel=kvm -cpu host -m 4096 -smp 4 \
    -drive if=pflash,format=raw,readonly=on,file=/usr/share/edk2/ovmf/OVMF_CODE.fd \
    -drive if=pflash,format=raw,file=./OVMF_VARS.fd \
    -drive if=virtio,format=qcow2,file=out/qcow2/disk.qcow2 \
    -nic user,model=virtio-net-pci -display gtk
```

GNOME Initial Setup creates the first account, the administrator. Then inspect:

```bash
deskos report        # the plan the image carries
deskos check         # what the declared plan measures, and what it leaves out
bootc status         # booted deployment, digest and followed reference
```

## 7. Optional: a reachable update source

```bash
registry=registry.example.com/example/devops
sudo podman login "$registry"
sudo podman tag localhost/example-devops-centos-stream-10:test "$registry:v1"
sudo podman push "$registry:v1"
```

Build the QCOW2 from `$registry:v1`; a changed tag offers the next version. For
A → B → A, push **the same tag**, and run:

```bash
sudo deskos update          # stages the next image; images only
sudo reboot
bootc status                # booted digest is B; A is the rollback
sudo bootc rollback
sudo reboot
bootc status                # A again
```

`deskos update` updates only images; rollback keeps `/var`.

## 8. Trust and the lab boundary

- `TrustAnchor/example-root-ca` trusts the organization's **services** for TLS,
  not image authenticity.
- A published Core image is signed and attested
  ([supply-chain.md](../../docs/supply-chain.md#signing-and-provenance)); a
  `localhost/` one is not.
- A `localhost/` reference is a lab artifact.

## 9. What this exercise exposed

- Image, system Flatpaks and user tools are three update surfaces on purpose.
- Test updates through a registry; a lab image is not an update source.
