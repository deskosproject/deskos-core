# Roadmap

No dates are implied. Fleet configuration management is outside the
scope of every milestone.

## Milestone 0: resource-model proof (current)

Resources, schemas, layered composition, typed Plan, Containerfile
backend, DeskOS Core on CentOS Stream 10 and RHEL 10, the example organization.

## Milestone 1: entitled RHEL 10 build and boot validation

Done: entitled build of `example-devops-rhel10` on a pinned base (image
`0e65bf4b...`), lint passed, all layers scanned free of build-host
identity, `bootcheck.py` and `sessioncheck.py` passed on its QCOW2 in the
factory VM. Left: whether RHEL's flatpak 1.16 reads
`/usr/share/flatpak/preinstall.d`, which needs a RHEL workstation that
declares Flatpaks.

## Milestone 1b: visual identity and boot

Proposal in `docs/design/visual-identity-and-boot.md`; each slice is
reviewed on its own:

1. First-boot (GIS) wallpaper from the effective wallpaper (vendor
   GSettings override). No API change. Implemented; seen in a CS10 Core
   VM.
2. `appearance.loginLogo` for GDM. Implemented, with provisional marks;
   seen in a CS10 Core VM. Dock semantics (masking, `dock.showTrash`)
   implemented; Core enables Dash to Dock by default, active in the CS10
   instrumented session check. A session `desktopLogo` (background-logo
   extension) is separate and not started.
3. Decide where boot intent lives. Decided: BootProfile (ADR 0006).
4. Quiet graphical boot with the stock Plymouth theme (kargs.d, Plymouth
   in the initramfs). Implemented; seen on CS10 Core UEFI VMs
   (`bootcheck.py`). Not booted on RHEL with the current Core.
5. DeskOS and organization splash watermark (`BootProfile.watermark`,
   DeskOS-owned theme), with a provisional DeskOS mark. Seen at boot and
   shutdown on UEFI VMs. Organization marks only with recorded provenance.

## Milestone 2: independent CentOS CI build

- A manual GitHub workflow (`vm-bootcheck.yml`, `workflow_dispatch` only)
  builds the CS10 Core image, a test QCOW2 that is not uploaded, and runs
  the boot and session checks; it passed on commit `3876414` (base pinned to the 2026-09-28 stream10 build). Triggering
  it automatically on changes to image inputs is not decided.
- Publishing: the same workflow, run with `publish` on `main`, pushes the
  image that passed its checks to `quay.io/deskos/deskos-core` as
  `:<commit>` and `:latest`. CentOS Stream 10 only.
- Digest-pinned base image, updated deliberately. Done for CentOS Stream
  10 and RHEL 10.
- Decide how to treat `bootc container lint` warnings from packages.

## Milestone 3: VM boot and desktop tests

Details in `docs/design/visual-identity-and-boot.md` ("VM QA").

Existing, CS10 only (`tests/vm/`):

- Tier A, unmodified artifact: `bootcheck.py` boots the exact QCOW2
  through an overlay, classifies screendumps (splash, graphical) and
  checks ACPI shutdown. It does not match screens against references.
- Tier B subset, instrumented session: `sessioncheck.py` adds a test user
  and GDM autologin through systemd credentials (SMBIOS type 11); a
  report unit writes the results and journal to the serial port. No SSH
  or qecore. Checks the dock, favorites, wallpaper, headless Firefox and
  failed units against the Plan; required checks fail closed.

Remaining:

- Check that every declared favorite desktop id exists in
  `/usr/share/applications` (the compiler cannot know package contents),
  and that favorites launch.
- Browser against harness-served content, offline login, GDM logo check.
- Reboot and shutdown splash in the session run; Tier C lifecycle
  (upgrade, rollback).
- RHEL: no automated boot or session check.

## Later

- Lockfiles resolving binary artifacts, keys and base digests, designed
  after real release experience shows which inputs need them.
- Immutable identity for RPM repository GPG keys (mechanism undecided).
- A decision by the example organization on the `virt-manager` gap.

## Milestone 4: workstation E2E

- QEMU/KVM GNOME sessions tested through accessibility APIs, following
  Bluefin's factory practice.
- Promotion gates: only digests that passed E2E move forward.

## Milestone 5: identity and provisioning

- Standalone (installer-created local user) and managed (FreeIPA or AD)
  models, per-machine emergency account.
- Installer and kickstart integration; enrollment kept out of the image.

## Milestone 6: compliance and supply chain

- OpenSCAP evidence, SBOM, provenance attestations, keyless signing.
- Candidate resources: TrustPolicy, ComplianceProfile.

## Milestone 7: release promotion and update UX

- Channels (candidate, canary, pilot, stable) promoting one digest.
- A local endpoint command (`deskos status`, `deskos update`,
  `deskos rollback`) that a user or administrator runs on one machine to
  inspect the deployment and invoke bootc's own staged update and
  rollback, system Flatpak updates and fwupd checks. No controller,
  reconciliation loop or automatic enforcement (ADR 0001).
- Candidate resources: UpdatePolicy, TimeSyncPolicy, PerformancePolicy,
  FirmwarePolicy, IdentityProvider, WebApplication.
