# DeskOS documentation

New here? Start with the [README](../README.md), then
[Platforms](platforms.md) for what each platform can do today.

## The words this repository uses

They are few, and most confusion comes from two of them meaning more than one thing.

- **Core**: what this repository ships — the compiler (`deskosctl`), the resource model, the
  profiles and reference workstations a flavor builds on.
- **Platform**: an operating-system target and the facts that come with it. One resource per
  platform (`centos-stream-10`, `rhel-10`), holding its base image, its package manager,
  its package groups, its boot splash, and what it can and cannot do. Everything that is
  specific to one platform lives in its `Platform` resource and nowhere else.
- **Base image**: the OS image a platform starts `FROM` — the vendor's own
  (`quay.io/centos-bootc/centos-bootc`) or, where the vendor publishes no bootc base, one
  the project owns and supports itself (ADR 0014).
- **Vendor**: the supplier of that base — Canonical for Ubuntu, Red Hat for RHEL, the CentOS project. A **software** vendor whose repository an
  organization installs from is a different party and a different decision; the example in
  the Ubuntu repository is one of those.
- **Workstation**: the thing a person installs. One platform plus the profiles to select.
  `deskos-core-<platform>` are the ones this repository ships; `example-<platform>` are the
  examples that show how they are put together.
- **Profile**: a layer of intent — `foundation`, `organization`, `role`, `workstation`. A
  profile selects resources; it does not describe a machine on its own.
- **Baseline**: the boundary of what DeskOS manages at all — machine-wide software an
  organization requires for any authorized user, complete before any user's home directory
  exists (ADR 0005). It is **not** a layer and not a file: what an organization's own layer
  carries is a profile with layer `organization`, and the example that shows it is
  [`examples/organization-and-role/`](../examples/organization-and-role/).
- **Resource**: the declarative unit you write, one of fourteen kinds ([Resources](resources.md)
  lists them all with an example).
- **Desktop**: the GNOME part of a workstation, and the word covers three things that are
  easy to mix up: the **session** (what starts, what the user sees), its **defaults** (dconf
  and GSettings values, `GnomeProfile`), and its **look** (wallpaper, login logo, icons,
  extensions, `Theme` and the `deskos-look` profile). It is also a *platform fact*: a
  platform that declares `gnome.keepsVendorDefaults` keeps the desktop its vendor ships, and
  DeskOS writes no default there (ADR 0016). Apps are neither: they are `PackageSet`, a
  Flatpak remote, or a signed vendor repository.
- **Flavor**: an organization's own resource root, built on Core's profiles and styles —
  its own package sources, its own desktop choices, its own workstations. It references
  Core's resources by name and copies none of them.
- **Artifact**: what composition and rendering produce — a deterministic **plan** and a
  build **context**. The image a `podman build` makes from that context is the thing that
  boots.

| Page | What it is |
|---|---|
| [Platforms](platforms.md) | **The canonical matrix**: base owner, what is verified, and how to install each platform |
| [Installing DeskOS](install.md) | QCOW2, the Anaconda ISO, and private RHEL builds. An Ubuntu install is documented in that platform's own repository; AlmaLinux and Fedora are example platforms |
| [Architecture](architecture.md) | The pipeline and boundary, the resource model, composition, the Plan, the Containerfile backend, and the evidence behind the claims |
| [Resources](resources.md) | Every one of the fourteen kinds, with an example |
| [Compatibility](compatibility.md) | What stays stable between releases, the fixture matrix, and beta/1.0 criteria |
| [Operations](operations.md) | Updates, inspect, apply, roll back, change the update source |
| [Recovery](recovery.md) | Install, first boot, registry, update and rollback failures, with evidence to collect |
| [Diagnostics](diagnostics.md) | What a resource, input or machine failure looks like, which class it is, and the next action |
| [Supply chain](supply-chain.md) | SBOM, scan, exceptions, signing, verifying a digest |
| [Checks](checks.md) | What each workflow proves, what evidence it keeps, and how to read a red run |
| [Threat model](threat-model.md) | Assets, trust boundaries, verified controls vs gaps, and the publication-incident tabletop |
| [Development](development.md) | Build `deskosctl`, run the tests, cut a release |
| [Installer ISO](installer-iso.md) | The Anaconda kickstart, and the warning that it erases every disk |
| [Design: visual identity and boot](design/visual-identity-and-boot.md) | Branding, splash, first-boot appearance |
| [Design: durable evidence](design/evidence-retention.md) | Digest-keyed evidence storage: options, recommendation, retrieval and open decisions |
| [Design: promotion](design/promotion.md) | One tested digest through candidate/canary/stable, with evidence and human approval, without rebuilding |
| [Design: authenticity](design/authenticity.md) | Consumer-side signature policy: mechanisms, identity limits, recovery and the lab plan |
| [VM harness](../tests/vm/README.md) | The boot, session and first-boot checks, and their transport limits |
| [Roadmap](roadmap.md) · [Research notes](research-notes.md) | What is done, planned, and measured — the notes are a dated archive |
| [ADRs](adr/README.md) | The decisions behind all of it |

## Where the rest is

- [Migrations](migrations.md): what to edit when moving from an older release.
- [Contributing](../CONTRIBUTING.md): what belongs here, how to run the checks,
  and where a bug, a question, a security report or a conduct concern goes.
- [Support](support.md) and [Security](../SECURITY.md): where to ask, what is
  promised, and how to report a vulnerability privately.
- [Governance](governance.md): who decides, who owns what, and what happens if
  that person is unavailable.
- [Checks](checks.md): what each workflow asserts, and which claims are still
  measured by hand.
- [Session E2E](design/session-e2e.md): the design for driving a GNOME session
  through accessibility APIs, and what it will not prove.
- [How other desktops test a session](design/session-e2e-prior-art.md): the three
  families of practice, what each proves, and what this repository takes from
  them.
- [Upstream findings](upstream.md): where a measured finding belongs, what
  evidence goes with it, and which ones we have not filed.
- [Roadmap](roadmap.md): milestones, and the closeout of the P0/P1 items with
  the evidence behind each.
