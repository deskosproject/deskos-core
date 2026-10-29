<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/deskos-lockup-dark.svg">
    <img alt="DeskOS" src="docs/images/deskos-lockup-light.svg" width="360">
  </picture>
</p>

<p align="center"><strong>Managed Linux workstations, built like infrastructure.</strong></p>

<p align="center">
  <a href="https://github.com/deskosproject/deskos-core/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/deskosproject/deskos-core/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/deskosproject/deskos-core/actions/workflows/vm-bootcheck.yml"><img alt="vm-bootcheck" src="https://github.com/deskosproject/deskos-core/actions/workflows/vm-bootcheck.yml/badge.svg?branch=main"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
</p>

DeskOS compiles a workstation from text. You describe the machine (applications,
desktop settings, branding, update policy) in YAML resources, and DeskOS produces
a bootable Linux image. The image is built, scanned, booted and, on CentOS Stream
10, signed and published.

A workstation is one file:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-devops-centos-stream-10
spec:
  platformRef: centos-stream-10
  profiles: [deskos-core, deskos-gnome, example-organization, example-devops]
```

Profiles sit at fixed layers: `foundation`, `organization`, `role`, `workstation`.
A higher layer overrides a lower one on purpose, never because of file order. Sets
union, keyed definitions must agree, and a scalar takes the highest layer. The
resulting plan records where every value came from.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/deskos-flow-dark.svg">
    <img alt="Resource roots pass through validation, composition and a typed plan, become a build context, and build into one bootc image per platform" src="docs/images/deskos-flow-light.svg" width="900">
  </picture>
</p>

## Try it

The published image is `quay.io/deskos/deskos-core-centos-stream-10`. On a machine
that already runs bootc:

```bash
sudo bootc switch quay.io/deskos/deskos-core-centos-stream-10:stable
sudo reboot
```

To install onto a disk, turn the same image into a QCOW2 or an ISO and install it:
[Installing DeskOS](docs/install.md) has both commands, complete.

There is also a preview installer ISO at <https://download.deskos.org/>. Check its
`SHA256SUMS` before booting it.

## What the project checks

Every weekly build runs the same sequence, and publishes nothing unless all of it
passes:

| Check | What it proves | Where |
|---|---|---|
| Compile and render | the same resources produce the same plan and build context | [ci.yml](.github/workflows/ci.yml) |
| Image build and lint | the image builds from a pinned base and passes `bootc container lint` | [supply-chain.yml](.github/workflows/supply-chain.yml) |
| Boot | QEMU boots the disk to a graphical screen and shuts down | `tests/vm/bootcheck.py` |
| Session | every dconf default, extension, favorite, wallpaper and installed binary matches the plan; the desktop exposes its applications through AT-SPI and opens one | `tests/vm/sessioncheck.py` |
| First boot | the GNOME Initial Setup wizard is the session that comes up, and no account exists yet | `tests/vm/firstbootcheck.py` |
| Update and rollback | a machine moves A to B and back, including a candidate that cannot boot | [lifecycle.yml](.github/workflows/lifecycle.yml) |
| Domain login | a throwaway FreeIPA realm and a Samba AD domain, joined at first boot, with a session as the directory user | [identity-freeipa.yml](.github/workflows/identity-freeipa.yml) |
| Supply chain | installed SBOM, CVE scan, keyless signature and provenance on the digest | [supply-chain.md](docs/supply-chain.md) |

The results of each published digest are in `evidence.deskos.org`, keyed by
digest, and summarised at [deskos.org/evidence](https://deskos.org/evidence/).

## Platforms

| Platform | Today |
|---|---|
| CentOS Stream 10 | published image, installer ISO, and the checks above on every weekly build |
| RHEL 10 | composes and renders here; the entitled build runs in the organization that owns the subscription, and is never published |
| AlmaLinux 10, Fedora | example platforms: they compose, render and boot weekly, and publish nothing |

The full matrix, with what is and is not verified on each platform, is
[platforms.md](docs/platforms.md).

## Build your own

```bash
./bin/deskosctl validate ./examples/centos-stream-10
./bin/deskosctl render ./examples/centos-stream-10 \
    --workstation example-centos-stream-10 --output ./dist/centos
sudo podman build -t localhost/deskos-centos:test ./dist/centos
```

Then write a disk from it, or push it to a registry your machines can reach, as
[Installing DeskOS](docs/install.md) describes. `deskosctl` is a single binary
from [Releases](https://github.com/deskosproject/deskos-core/releases), with no Go
toolchain needed. Every resource kind, with an example, is in
[resources.md](docs/resources.md).

## What DeskOS is not

- **Not a distribution with editions.** Organizations compose their own
  workstations from Core, their baseline and their roles.
- **Not configuration management.** DeskOS owns the image, not the running machine.
  Ansible, Satellite, MDM and EDR keep managing endpoints.
- **Not a developer environment manager.** Personal tooling stays outside the
  managed baseline.
- **RHEL images are never published.** Organizations with a subscription build them
  privately; the EULA forbids redistribution.

## Read more

| | |
|---|---|
| Install a machine | [install.md](docs/install.md) |
| Operate one | [operations.md](docs/operations.md), [recovery.md](docs/recovery.md) |
| Fix a machine | [troubleshooting.md](docs/troubleshooting.md) |
| Choose a platform | [platforms.md](docs/platforms.md) |
| Understand the model | [architecture.md](docs/architecture.md), [runtime.md](docs/runtime.md) |
| Write resources | [resources.md](docs/resources.md) |
| Audit the supply chain | [supply-chain.md](docs/supply-chain.md) |
| Contribute | [CONTRIBUTING.md](CONTRIBUTING.md), [development.md](docs/development.md) |
| What is next | [roadmap.md](docs/roadmap.md), [ADRs](docs/adr/README.md) |

## License

Code: [Apache License 2.0](LICENSE). DeskOS artwork (`resources/assets/deskos/`,
`docs/images/`): CC BY-SA 4.0, see
[PROVENANCE.md](resources/assets/deskos/PROVENANCE.md).
