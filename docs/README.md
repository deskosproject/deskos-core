# DeskOS documentation

New here? Read the [README](../README.md), then [Platforms](platforms.md).

## The words this repository uses

- **Core**: what this repository ships — the compiler, the resource model, the
  profiles and reference workstations.
- **Platform**: one operating-system target and every fact that comes with it
  (`centos-stream-10`, `rhel-10`). Platform differences live here and nowhere
  else.
- **Base image**: what a platform starts `FROM` — the vendor's, or a project-owned
  base where the vendor publishes none (ADR 0014).
- **Workstation**: the thing a person installs: one platform plus the profiles to
  select.
- **Profile**: a layer of intent (`foundation`, `organization`, `role`,
  `workstation`) that selects resources; not a machine on its own.
- **Baseline**: the boundary of what DeskOS manages — machine-wide state an
  organization requires for any authorized user, before any `$HOME` exists
  (ADR 0005). Not a layer, not a file.
- **Resource**: the declarative unit you write; one of fourteen kinds
  ([Resources](resources.md) lists all with an example).
- **Desktop**: three things that are easy to mix up — the **session**, its
  **defaults** (`GnomeProfile`), and its **look** (`Theme`, `deskos-look`). It is
  also a platform fact: `gnome.keepsVendorDefaults` means DeskOS writes no
  desktop default (ADR 0016).
- **Flavor**: an organization's own resource root built on Core, referencing it by
  name and copying nothing.
- **Artifact**: the deterministic **plan** and build **context**; the image
  `podman build` makes from that context is what boots.

## Pages

| Page | What it is |
|---|---|
| [Platforms](platforms.md) | the canonical matrix: base owner, what is verified, how to install |
| [Installing](install.md) | QCOW2, the Anaconda ISO, private RHEL builds |
| [Architecture](architecture.md) | pipeline and boundary, model, composition, Plan, backend, evidence |
| [Runtime behaviour](runtime.md) | baseline, Flatpaks, GNOME, boot, trust, updates, `/etc` drift, provisioning |
| [Resources](resources.md) | the fourteen kinds, with examples |
| [Compatibility](compatibility.md) | what stays stable, the fixture matrix, beta and 1.0 criteria |
| [Operations](operations.md) | update, inspect, roll back, change the update source |
| [Recovery](recovery.md) | install, first boot, registry, update and rollback failures |
| [Diagnostics](diagnostics.md) | what a failure looks like, which class it is, next action |
| [GNOME coverage](gnome-coverage.md) | every GNOME system-administration item, and whether a field carries it |
| [Supply chain](supply-chain.md) | SBOM, scan, exceptions, signing, verifying a digest |
| [Checks](checks.md) | what each workflow proves and how to read a red run |
| [Threat model](threat-model.md) | assets, boundaries, controls and gaps, the incident tabletop |
| [Development](development.md) | build `deskosctl`, run the tests, cut a release |
| [Installer ISO](installer-iso.md) | the kickstart, and the warning that it erases every disk |
| [Design](design/) | visual identity, evidence retention, promotion, authenticity, session E2E, prior art |
| [VM harness](../tests/vm/README.md) | the boot, session and first-boot checks and their limits |
| [Roadmap](roadmap.md) · [ADRs](adr/README.md) | what is done and planned; the decisions behind it |
| [Research notes](research-notes.md) | a dated archive of what was measured |

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
- [Style](style.md): how these pages are written; `make check` runs the link
  checker.
