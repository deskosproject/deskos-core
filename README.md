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

**DeskOS compiles a workstation from text.** You describe the machine — its
applications, desktop settings, branding, update policy — in declarative
resources, and DeskOS turns them into a bootable operating-system image — built,
scanned, and, where the platform allows it, signed and published like any other
artifact.

```text
YAML → validate → compose → plan → Containerfile → bootc image → disk
```

Most people should never run that pipeline by hand: **two platforms publish a
finished image you can install today.** The rest of this page starts there, and
leaves the compiler for whoever wants to understand everything.

## Use it

`quay.io/deskos/deskos-core-centos-stream-10` and
`quay.io/deskos/deskos-core-almalinux-10` are the published images, each
`:latest` plus a `:<commit>` tag, built and booted by the gate before they were
pushed. On a machine that already runs bootc, move it to DeskOS:

```bash
sudo bootc switch quay.io/deskos/deskos-core-centos-stream-10:latest
sudo reboot
```

That follows `:latest`, which is what you want while evaluating. For a machine
you intend to keep, pin a digest you have verified instead — the reference a
machine follows is also what `bootc upgrade` moves it to.

To install onto a disk instead, make a QCOW2 or an ISO from the same published
image — no compiler involved — as
[**Installing DeskOS**](docs/install.md#install-a-published-image) describes.

Four platforms are compiled from this repository; two publish an image, and the
table says what has actually been verified on each:

| Platform | Base | Today |
|---|---|---|
| CentOS Stream 10 | vendor | **published image**, built and booted in CI |
| AlmaLinux 10 | vendor (experimental) | **published image**, built and booted in CI |
| RHEL 10 | vendor (entitled) | composes and renders here; **built and booted by the organization** — its build and boot were validated once on a retired host, and are not re-run here |
| Fedora 45 | vendor | composes and builds; boot check in CI, not yet green; pinned to a prerelease compose, no published image |

Ubuntu 26.04 is not in this table. It has a base of its own and lives in
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu),
which builds, boots and publishes it. Core keeps the language — `deb`, the apt
rendering and the `AptRepository` kind — and tests that it can express and render
a Debian-family workstation, but no longer builds, boots or publishes one
([ADR 0020](docs/adr/0020-ubuntu-lives-in-its-own-repository.md)).
[**docs/platforms.md**](docs/platforms.md) is the full matrix, with the evidence
behind every cell.

## Make it yours

An organization with more than a handful of Linux desktops ends up with
**snowflakes**: each machine was installed once, by hand, and then drifted.
Nobody can say what is on them, reproduce one, or prove one is safe to ship.
DeskOS takes the machine definition out of each installer's hands.

You author one file, a `Workstation`: a platform plus the profiles it is made of.

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-devops-centos-stream-10
spec:
  platformRef: centos-stream-10
  profiles:
    - deskos-core            # foundation: packages and the update policy
    - deskos-gnome           # foundation: DeskOS's GNOME defaults
    - example-organization   # organization
    - example-devops         # role
```

Profiles sit at explicit layers — `foundation` < `organization` < `role` <
`workstation` — so an organization overrides DeskOS Core **on purpose**, never by
file order. Sets (packages, units) are unioned; keyed definitions (repositories,
Flatpaks, binaries) must agree; scalars (desktop settings) take the highest layer,
and two different values at the same layer are an error. Every value in the
resulting plan keeps its provenance: the resource, the layer and the file it came
from.

The [example organization](examples/organization-and-role) is a complete one:
copy it, point `deskosctl` at that directory, and edit. Every resource kind, with
an example, is in [**docs/resources.md**](docs/resources.md).

## Compile it

This is the part for whoever wants the whole pipeline. `deskosctl` is one binary
with no Go toolchain needed:

```bash
curl -fLO https://github.com/deskosproject/deskos-core/releases/latest/download/deskosctl-linux-amd64
curl -fLO https://github.com/deskosproject/deskos-core/releases/latest/download/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS
mv deskosctl-linux-amd64 deskosctl && chmod +x deskosctl
```

Then see what the repository already ships, and what the image will contain:

```console
$ ./deskosctl validate
ok: 3 workstation(s) compose cleanly: deskos-core-almalinux-10,
    deskos-core-centos-stream-10, deskos-core-fedora-45

$ ./deskosctl plan --workstation deskos-core-centos-stream-10
WORKSTATION
  deskos-core-centos-stream-10 (DeskOS Core (CentOS Stream 10))

PLATFORM
  CentOS Stream 10 (centos-stream-10)
  base image: quay.io/centos-bootc/centos-bootc:stream10 (pinned sha256:98cb9980…)
  publication: redistributable

PROFILES
  foundation:  deskos-core
  foundation:  deskos-gnome
  foundation:  deskos-look
  foundation:  deskos-reference-apps
  foundation:  rhel-family
```

`plan` prints what the image will contain and **where each value came from**.
`render` writes the deterministic build context — the same inputs always give the
same bytes, and `generated-manifest.json` records their SHA-256 — which `podman
build` turns into an image, and `bootc-image-builder` into a disk. That whole
flow, with the exact commands, is in
[**docs/development.md**](docs/development.md#build-the-image-yourself).

> [!NOTE]
> **Pre-1.0.** The resource APIs are `v1alpha1` and will change.
> The image build is not yet bit-for-bit reproducible: the base tag and the
> repository content can move between builds
> ([supply-chain inputs](docs/architecture.md#supply-chain-inputs)).

## What DeskOS is not

- **Not a distribution with editions.** Organizations compose their own
  workstations from DeskOS Core, their baseline and their roles.
- **Not configuration management.** DeskOS owns the *image*, not the running
  machine; Ansible, Satellite, MDM and EDR keep managing endpoints.
- **Not a developer environment manager.** Personal tooling (Homebrew, mise,
  dotfiles, Toolbox contents) stays outside the managed baseline.

> [!IMPORTANT]
> **RHEL 10 images are never published.** Organizations with RHEL subscriptions
> build them privately and keep them inside the organization — the RHEL EULA
> forbids public redistribution. See
> [docs/install.md](docs/install.md#rhel-10-private-builds-only).

## Where to go next

| | |
|---|---|
| **Install a machine** | [**Installing DeskOS**](docs/install.md): QCOW2 and ISO from a published image; private RHEL 10 builds; Fedora 45 |
| **Operate one** | [**Operations**](docs/operations.md): how updates stage, apply and roll back |
| **Choose a platform** | [**Platforms**](docs/platforms.md): the canonical matrix and what is verified on each |
| **Author resources** | [**Resources**](docs/resources.md): every kind with an example |
| **Understand the design** | [**Architecture**](docs/architecture.md): the pipeline, the Plan, the backend, and the evidence behind the claims |
| **Audit the supply chain** | [**Supply chain**](docs/supply-chain.md): SBOM, scan, exceptions, signing |
| **Contribute** | [**Development**](docs/development.md): build `deskosctl`, run the tests, cut a release |
| **Track** | [Roadmap](docs/roadmap.md) · [Research notes](docs/research-notes.md) (a dated archive) · [ADRs](docs/adr/README.md) |

## License

**Code:** [Apache License 2.0](LICENSE).
**DeskOS artwork** (`resources/assets/deskos/`, `docs/images/`): CC BY-SA
4.0; see [PROVENANCE.md](resources/assets/deskos/PROVENANCE.md).
