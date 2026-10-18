# Example: DeskOS Core on Ubuntu 26.04

DeskOS Core on Ubuntu 26.04 LTS, the platform whose vendor publishes no bootc
base: the base is owned by the DeskOS project and pinned by digest in
`resources/platforms/ubuntu-2604.yaml`
([ADR 0014](../../docs/adr/0014-platform-base-ownership.md)). It is a
**separate resource root** that references the shipped `deskos-core` profile
and the `ubuntu-2604` Platform by name:

```bash
deskosctl plan ./examples/ubuntu-2604 \
  --workstation example-ubuntu-2604
```

| Resource | Contents |
|---|---|
| `example-ubuntu-2604` | the target: `ubuntu-2604` + the `deskos-core` profile |
| `example-ubuntu-2604-org-packages` | the same Core plus an organization profile that installs `code` from a declared, signed APT source |

This example does not include the `rhel-family` profile: its workstation is
Core's foundation plus the organization profile, without firewalld, an
RPM-named browser or an RPM application channel.

`example-ubuntu-2604-org-packages` declares a signed `AptRepository` and a `PackageSet`
that names it. The build writes the deb822 source and its key under `/etc/apt/`,
and `apt-get update` verifies the signature before the packages install; the
`PackageSet` does not bind a package to one source, so this does not claim which
source each installed package came from. The CI job that builds and boots it is
in `.github/workflows/vm-bootcheck.yml`. Both workstations boot to GNOME Initial
Setup, which `tests/vm/firstbootcheck.py` asserts by asking the guest.

DeskOS implements no ISO path for this platform; what is validated is a disk
written by `bootc install to-disk`; see
[docs/install.md](../../docs/install.md#ubuntu-2604).

## How you would build it

The same four steps as on any platform — get the compiler, validate, render, build — and
then one more, because this platform has no Anaconda path: the disk is written directly.

```bash
# 1. the compiler, from the release this repository pins
base=https://github.com/deskosproject/deskos-core/releases/latest/download
mkdir -p deskos/bin && cd deskos
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
install -D -m 0755 deskosctl-linux-amd64 bin/deskosctl

# 2. your resources. This example is one: copy it and change it
./bin/deskosctl validate /path/to/your/resources

# 3. the build context
./bin/deskosctl render /path/to/your/resources \
  --workstation example-ubuntu-2604 --output ./dist/ubuntu

# 4. the image
sudo podman build -t registry.example.com/deskos/ubuntu:latest ./dist/ubuntu

# 5. the disk, from a compressed OCI layout: this base documents `bootc install to-disk`
sudo skopeo copy containers-storage:registry.example.com/deskos/ubuntu:latest \
  oci-archive:/var/tmp/ubuntu-oci:latest
sudo bootc install to-disk --target-imgref registry.example.com/deskos/ubuntu:latest ...
```

The full commands, including what `--target-imgref` must point at and why pointing it at the
base is wrong, are in [install.md § Ubuntu 26.04](../../docs/install.md#ubuntu-2604).

> [!NOTE]
> **This is the platform, not the flavor.** DeskOS's own Ubuntu workstations are moving to a
> repository of their own — `deskosproject/deskos-ubuntu`, on its own base
> `deskosproject/deskos-ubuntu-bootc` — and that is where a person who wants a ready Ubuntu
> workstation should look. What this example documents is the platform underneath it: the
> facts and the composition.

Today this platform *is* built and booted here: CI writes a disk with `bootc install
to-disk`, boots it and runs the session and first-boot checks, and the reference workstation
is published ([platforms.md](../../docs/platforms.md) has the evidence). That is the part
that moves with the flavor, and this page will say so on the day it does.

