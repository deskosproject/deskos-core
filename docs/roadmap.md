# Roadmap

No dates are implied. Fleet configuration management is outside the
scope of every milestone.

## Milestone 0: resource-model proof (current)

Resources, schemas, layered composition, typed Plan, Containerfile
backend, DeskOS Core on CentOS Stream 10 and RHEL 10, the example organization.

## Milestone 1: entitled RHEL 10 build and boot validation

Done: clean entitled build of `example-devops-rhel10` (image `81bdfeec...`),
lint passed, all layers scanned free of build-host identity, QCOW2 booted
manually. Left: rebuild with the corrected VS Code favorite, and whether
RHEL's flatpak 1.16 reads `/usr/share/flatpak/preinstall.d`.

## Milestone 1b: visual identity and boot

Proposal in `docs/design/visual-identity-and-boot.md`; each slice is
reviewed on its own:

1. First-boot (GIS) wallpaper from the effective wallpaper (vendor
   GSettings override). No API change. Implemented.
2. `appearance.loginLogo` for GDM. Implemented, with provisional marks.
   Dock semantics (masking, `dock.showTrash`) implemented; Core enables
   Dash to Dock by default (render level; not yet seen in a VM).
   A session `desktopLogo` (background-logo extension) is separate and
   not started.
3. Decide where boot intent lives. Decided: BootProfile (ADR 0006).
4. Quiet graphical boot with the stock Plymouth theme (kargs.d, Plymouth
   in the initramfs). Implemented at render level; needs an image build
   and VM boot.
5. DeskOS and organization splash watermark (`BootProfile.watermark`,
   DeskOS-owned theme), with a provisional DeskOS mark. Seen at boot and
   shutdown on UEFI VMs. Organization marks only with recorded provenance.

## Milestone 2: independent CentOS CI build

- CI builds the public CentOS image from the rendered context on changes
  to image inputs, independent of any developer machine.
- Digest-pinned base image, updated deliberately.
- Decide how to treat `bootc container lint` warnings from packages.

## Milestone 3: VM boot and desktop tests

Details in `docs/design/visual-identity-and-boot.md` ("VM QA"):

- Tier A, unmodified artifact: boot the exact QCOW2, screendumps of
  splash and first boot, shutdown/reboot; the only proof of untouched
  first boot.
- Tier B, instrumented session: disposable test user, autologin, SSH and
  qecore, perturbations listed; GDM logo, wallpaper, favorites launch,
  browser against harness-served content, offline login, reboot splash.
- Evidence tied to image digest, QCOW2 SHA-256 and harness revisions;
  finite timeouts; required checks fail closed.
- Check in the built image that every declared favorite desktop id exists
  in `/usr/share/applications` (the compiler cannot know package contents).

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
- Endpoint `deskos status`, `deskos update`, `deskos rollback`: staged OS
  updates without forced reboots, system Flatpak updates, fwupd checks.
- Candidate resources: UpdatePolicy, TimeSyncPolicy, PerformancePolicy,
  FirmwarePolicy, IdentityProvider, WebApplication.
