# Example: DeskOS Core on RHEL 10

The minimal DeskOS Core composition on Red Hat Enterprise Linux 10. It is
a **separate resource root** that references the shipped `deskos-core` and
`deskos-gnome`
profile and the `rhel-10` Platform by name:

```bash
deskosctl plan ./examples/rhel-10 \
  --workstation example-rhel-10
```

| Resource | Contents |
|---|---|
| `example-rhel-10` | the target: `rhel-10` + the `deskos-core` and `deskos-gnome` profiles |

> [!IMPORTANT]
> RHEL-derived images are covered by the RHEL EULA and are **never
> published**. `rhel-10` marks them `redistributable: false`.

## How you would build it

This repository does not build or boot this one, and it never publishes it: the base is
`registry.redhat.io/rhel10/rhel-bootc`, behind a subscription that lives with the
organization that has it, not here. What this repository does check is that the example
**composes and renders** deterministically, like any other.

On a RHEL 10 machine registered to your subscription, as root (the build runs as root):

```bash
# 1. the compiler, from the release this repository pins
base=https://github.com/deskosproject/deskos-core/releases/latest/download
mkdir -p deskos/bin && cd deskos
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl

# 2. your resources. This example is one: copy it and change it
./bin/deskosctl validate /path/to/your/resources

# 3. the build context, as a plan you can read and diff before anything runs
./bin/deskosctl render /path/to/your/resources \
  --workstation example-rhel-10 --output ./dist/rhel

# 4. the image
sudo podman build -t registry.example.com/deskos/rhel10:latest ./dist/rhel
```

A disk or an ISO comes from that image with `bootc-image-builder`. The whole path, with the
flags that matter, is
[install.md § RHEL 10](../../docs/install.md#rhel-10-private-builds-only).

The build context is the same shape as any other platform's, which is the point: what
changes between platforms is what the `Platform` resource declares, not what you type.
