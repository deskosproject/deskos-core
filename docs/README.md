# DeskOS documentation

Start with the [README](../README.md), then [Platforms](platforms.md).

## The words this repository uses

- **Core**: what this repository ships: the compiler, the resource model, the
  profiles and the reference workstations.
- **Platform**: one operating-system target and the facts that come with it
  (`centos-stream-10`, `rhel-10`). Platform differences live there and nowhere
  else.
- **Base image**: what a platform starts `FROM`: the vendor's, or a project-owned
  base where the vendor publishes none (ADR 0014).
- **Workstation**: the thing a person installs: one platform plus the profiles to
  select.
- **Profile**: a layer of intent (`foundation`, `organization`, `role`,
  `workstation`) that selects resources.
- **Baseline**: what DeskOS manages at all: machine-wide state an organization
  requires for any authorized user, before any `$HOME` exists (ADR 0005).
- **Resource**: the declarative unit you write; one of fourteen kinds
  ([Resources](resources.md) lists all with an example).
- **Desktop**: the session, its defaults (`GnomeProfile`) and its look (`Theme`).
  It is also a platform fact: `gnome.keepsVendorDefaults` means DeskOS writes no
  desktop default (ADR 0016).
- **Flavor**: an organization's own resource root built on Core, referencing it by
  name and copying nothing.
- **Artifact**: the plan and build context; the image built from that context is
  what boots.

## Pages

| Page | What it is |
|---|---|
| [Platforms](platforms.md) | the matrix: base owner, what is verified, how to install |
| [Installing](install.md) | a published image, your own image, private RHEL builds |
| Templates | [CentOS Stream 10](https://github.com/deskosproject/deskos-template-cs10), [RHEL 10](https://github.com/deskosproject/deskos-template-rhel10): fork, build your image, make your ISO |
| [Architecture](architecture.md) | pipeline and boundary, model, composition, plan, backend |
| [Runtime](runtime.md) | baseline, Flatpaks, GNOME, boot, trust, updates, `/etc` |
| [Resources](resources.md) | the fourteen kinds, with examples |
| [GNOME coverage](gnome-coverage.md) | each GNOME guide item, and whether a field carries it |
| [Compatibility](compatibility.md) | what stays stable, the fixture matrix, 1.0 criteria |
| [Operations](operations.md) | update, inspect, roll back, change the update source |
| [Recovery](recovery.md) | install, first boot, registry, update and rollback failures |
| [Diagnostics](diagnostics.md) | what a failure looks like and what to do next |
| [Supply chain](supply-chain.md) | SBOM, scan, exceptions, signing, verifying a digest |
| [Checks](checks.md) | what each workflow proves and how to read a red run |
| [Threat model](threat-model.md) | assets, boundaries, controls, the incident exercise |
| [Development](development.md) | build `deskosctl`, run the tests, cut a release |
| [Installer ISO](installer-iso.md) | the kickstart, the build command, the storage warning |
| [Design](design/) | visual identity, what the image takes from the GNOME HIG, evidence retention, promotion, authenticity, session E2E |
| [VM harness](../tests/vm/README.md) | the boot, session and first-boot checks |
| [Roadmap](roadmap.md), [ADRs](adr/README.md) | what is done and planned, and the decisions |
| [Research notes](research-notes.md) | a dated archive of what was measured |
| [Style](style.md) | how these pages are written; `make style` and `make links` check it |

## Where the rest is

- [Migrations](migrations.md): what to edit when moving from an older release.
- [Contributing](../CONTRIBUTING.md): scope, checks, where a bug, question,
  security report or conduct concern goes.
- [Support](support.md) and [Security](../SECURITY.md): where to ask, what is
  promised, how to report privately.
- [Governance](governance.md): who decides, who owns what, what happens if they
  cannot.
- [Upstream findings](upstream.md): where a measured finding belongs and which
  ones are unfiled.
