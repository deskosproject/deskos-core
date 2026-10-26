# Roadmap

DeskOS milestones and what each one has done or left. **No dates are
implied.** Fleet configuration management is outside the scope of every
milestone.

## Milestone 0: resource-model proof (done)

Resources, schemas, layered composition, typed Plan, Containerfile
backend, DeskOS Core on CentOS Stream 10 and RHEL 10, the example
organization.

## Milestone 1: entitled RHEL 10 build and boot validation

- **Done (retired):** entitled build of `example-devops-rhel-10` on a pinned
  base (image `0e65bf4b...`), lint passed, all layers scanned free of
  build-host identity, `bootcheck.py` and `sessioncheck.py` passed on its
  QCOW2 in the factory VM. The RHEL factory host was retired; the RHEL build
  now runs in a downstream organization's repository on GitHub-hosted
  runners, and this repository builds CentOS Stream 10, not RHEL. Ubuntu 26.04 is built in a repository of its own
  ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).
- RHEL's flatpak 1.16 reads `/usr/share/flatpak/preinstall.d`, checked
  with a private workstation that declares Flathub apps.

## Milestone 1b: visual identity and boot

Proposal in
[`docs/design/visual-identity-and-boot.md`](design/visual-identity-and-boot.md);
each slice is reviewed on its own:

1. First-boot (GIS) wallpaper from the effective wallpaper (vendor
   GSettings override). No API change. **Implemented**; seen in a CS10
   Core VM.
2. `appearance.loginLogo` for GDM. **Implemented**, with the DeskOS
   lockup; seen in a CS10 Core VM. Dock semantics (masking,
   `dock.showTrash`) **implemented**; the reference workstations select
   `Profile/deskos-dock`, and the dock is active in the CS10 instrumented
   session check. A session
   `desktopLogo` (background-logo extension) is separate and *not
   started*.
3. Decide where boot intent lives. **Decided:** `BootProfile`
   ([ADR 0006](adr/0006-bootprofile-tenth-kind.md)).
4. Quiet graphical boot with the stock Plymouth theme (kargs.d, Plymouth
   in the initramfs). **Implemented**; seen on CS10 Core UEFI VMs
   (`bootcheck.py`), and on RHEL 10.2 through the retired factory host.
5. DeskOS and organization splash watermark (`BootProfile.watermark`,
   DeskOS-owned theme), with the DeskOS lockup. Seen at boot and
   shutdown on UEFI VMs. Organization marks only with recorded
   provenance.

## Milestone 2: independent CentOS CI build

- **CI build:** `vm-bootcheck.yml` builds the CentOS Stream 10 workstations
  (Core and the organization example) into QCOW2 disks and runs the boot,
  session and first-boot checks on them. Until the platform moved ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md))
  it built the two Ubuntu 26.04 workstations into raw disks (`bootc install to-disk`) and ran
  the boot and first-boot checks, not the session check; its own repository does all three
  now. It runs on pull requests and on
  `main` for paths that can change an image, and on demand
  (`workflow_dispatch`).
- **Publishing:** `supply-chain.yml`, run manually with `publish=true` on
  `main`, builds, scans and boots each platform's image and then pushes and
  signs it to that platform's repository
  (`quay.io/deskos/deskos-core-centos-stream-10`) as `:latest` and
  `:<commit>`. CentOS Stream 10 today, with the
  digests and the run in [architecture.md](architecture.md#validation-status), and
  the older single repository
  (`quay.io/deskos/deskos-core`) has been withdrawn. `vm-bootcheck.yml` is a
  pure check.
- **Pinned base:** digest-pinned base image, updated deliberately. Done for
  CentOS Stream 10 and RHEL 10; the Ubuntu base is pinned in its own repository.
- **Example AlmaLinux and Fedora platforms.** Neither is a shipped platform. Their
  self-contained roots compose and render in CI, and `examples-weekly.yml` builds
  and boots them weekly without publishing. Re-promotion requires a durable base
  (for Fedora, a mirrored digest in DeskOS Quay) and the checklist in
  [example-platforms.md](example-platforms.md).
- **Open:** decide how to treat `bootc container lint` warnings from
  packages.

## Milestone 3: VM boot and desktop tests

Details in
[`docs/design/visual-identity-and-boot.md`](design/visual-identity-and-boot.md)
("VM QA").

**Existing for CentOS Stream 10**
([`tests/vm/`](../tests/vm/README.md)):

- **Tier A, unmodified artifact:** `bootcheck.py` boots the exact test disk —
  a QCOW2 on the EL platforms, a raw disk where a platform has no Anaconda path — through an overlay,
  classifies screendumps (splash, graphical) and
  checks ACPI shutdown. It does not match screens against references.
- **Tier B subset, instrumented session (CentOS Stream 10):** `sessioncheck.py`
  adds a test
  user and GDM autologin through systemd credentials (SMBIOS type 11); a
  report unit writes the results and journal to the serial port. No SSH
  or qecore. Checks the dock, favorites, wallpaper, headless Firefox and
  failed units against the Plan; required checks *fail closed*.
- **First boot, instrumented, no account:** `firstbootcheck.py` adds no
  account and no session, and asks the guest whether the first-boot wizard is
  the session that came up, whether an account could log in, and whether a
  service failed.

**Remaining:**

- Check that every declared favorite desktop id exists in
  `/usr/share/applications` (the compiler cannot know package contents),
  and that favorites launch.
- Browser against harness-served content, offline login, GDM logo check.
- **Reboot and shutdown splash:** the harness records every frame during
  shutdown, and the session run powers off in about **0.4 s** with a single
  frame captured. An assertion would depend on catching that frame, so it is
  recorded evidence, not a check; a reboot splash needs a boot that reboots
  instead of powering off.
- **Tier C lifecycle (A -> B -> A):** designed and implemented
  (`tests/lifecycle/driver.py`), and it ran end to end on CS10 on 2026-10-24
  (install and stage, boot B, roll back, boot A; power loss mid-upgrade
  recovered). On 2026-10-25 it also staged a candidate that cannot boot,
  confirmed it never reached SSH, and recovered the previous deployment by
  its composefs verity. `lifecycle.yml` ran the full path on GitHub-hosted
  runners on 2026-10-26 and passed: 7 steps, A → B → A, power loss and the
  failed candidate. It runs weekly on `main` and on dispatch, not on pull
  requests: it is too slow for every change. The ostree-write interruption
  window is still untested.
- **Installer ISO:** no check installs one. The ISO is built, signed and
  published, but nothing boots Anaconda and asserts it reaches the progress
  screen and installs. On 2026-10-23 an organization's RHEL ISO shipped a kickstart
  with `graphical --non-interactive`; RHEL 10.2's Anaconda aborted it at the
  progress screen (`User interaction required on standalone spoke
  ProgressSpoke`), and the machine rebooted, which reads as a hardware
  failure. Boot the ISO in QEMU and follow the install, at least for the
  CentOS Stream 10 preview.
- **Installer tree pinning:** the installer's own payload (Anaconda,
  lorax) comes from the distribution repositories when the ISO is built; it
  is not pinned the way the image is, so the same kickstart can behave
  differently between builds. Decide whether to pin it or to record that it
  floats.
- RHEL: the build, boot and session checks run in
  a downstream organization's repository (GitHub-hosted); no reference
  matching or E2E.

## Later

- Lockfiles resolving binary artifacts and base digests, designed after
  real release experience shows which inputs need them.
- **Tracked resource pins.** Renovate keeps base-image digests and GitHub
  Actions current, but it cannot resolve the pins that live in resources:
  `BinaryArtifact` and `ExtensionSource` carry a URL, a version and a
  sha256, and nothing updates them. Add a `deskosctl track` maintenance
  command — resolve the declared pattern, download, verify, hash, rewrite
  the resource and regenerate the goldens — plus a scheduled workflow with
  a curated list, review notes for core components, and **one bundle pull
  request** with one commit per resource. A person merges; nothing is
  published. The compiler stays offline and deterministic: tracking is a
  maintenance step, not part of compilation. Modeled on BuildStream's
  `track:` and `bst source track`, which Bluefin's Dakota runs daily in
  `.github/workflows/track-bst-sources.yml`; Renovate covers the rest.
- ~~Immutable identity for RPM repository GPG keys~~ done: `RpmRepository`
  keys are local assets read into the plan (see
  [resources.md](resources.md#rpmrepository)).
- ~~APT sources as their own kind~~ done: `AptRepository` renders a signed
  deb822 source and its keyring, and a `PackageSet` names it through
  `aptRepositories` (ADR 0015).
- A decision by the example organization on the `virt-manager` gap.
- **Example AlmaLinux and Fedora platforms:** their image builds move to the
  weekly `examples-weekly.yml` check, with no publication. This is not a support
  promise; [example-platforms.md](example-platforms.md) records the re-promotion
  path.
- ~~A **session check for Ubuntu**~~ done: `vm-bootcheck.yml` runs
  `sessioncheck.py` on the Ubuntu disk beside the boot and first-boot checks
  ([platforms.md](platforms.md)). ~~Whether DeskOS publishes the Ubuntu
  derivative, and where~~ decided: it does, as
  `quay.io/deskos/deskos-core-ubuntu-2604`, for a person who asked for Ubuntu
  support -- and it moved to a repository of its own afterwards, which publishes and checks
  it ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).
- ~~**Fedora's rebuilt initramfs dropped ostree and bootc**~~ fixed: the
  regenerated initramfs carried neither module (Fedora's `50ostree` says "only
  when asked" and `51bootc` "never installed by default"), so the disk stopped in
  dracut's emergency shell on btrfs and ext4 alike (measured 2026-10-20, runs
  39436779300, 39448084854 and 39452191007). The backend now adds both modules
  when the base carries them and asserts `ostree-prepare-root` in every
  regenerated initramfs. The btrfs disk boots with that fix (run 39462806315:
  graphical screen in 30 s, session and first-boot checks green). The published
  CentOS image was rebuilt after the fix
  ([research-notes.md](research-notes.md#fedoras-rebuilt-initramfs-dropped-ostree-and-bootc-2026-10-20)).
- The **splash race**: on one disk, a first boot showed text for the whole boot
  and a second drew the splash; the retry in `vm-bootcheck.yml` absorbs it but
  not always, so `main` can go red on its own. Cause still unexplained.
- **Fedora example:** the weekly examples workflow builds and boots it from the
  current Fedora tag. A published Fedora platform waits for a mirrored base and
  the promotion checklist in [example-platforms.md](example-platforms.md).
- RHEL's Red Hat logo in the Activities button (see
  [research-notes.md](research-notes.md#gnome-shell-on-el10-2026-10-04)):
  replacing `fedora-logo-icon` through an icon theme is untried.
- Applying the rest of a [Theme](adr/0013-theme.md) palette: a **Ghostty**
  terminal theme and a best-effort **GNOME Shell** recolor are implemented in
  `deskos-theme`. Still open: installing a third-party palette from a git URL
  (`deskos-theme install <url>`, reading only `colors.toml` and
  `backgrounds/` and never running the repository's code) and vendoring one
  into resources pinned by commit.
- **Skip the GNOME Initial Setup software page.** The first-boot wizard still
  offers the "enable third-party repositories" step, which means nothing on an
  image-based workstation whose store is Bazaar. Ship
  `/etc/gnome-initial-setup/vendor.conf` with `skip=software`, as Bluefin does,
  and assert in the first-boot check that the page is gone.

## Milestone 4: workstation E2E

- QEMU/KVM GNOME sessions tested through accessibility APIs, following
  Bluefin's factory practice.
- Promotion gates: only digests that passed E2E move forward.

## Milestone 5: identity and provisioning

- Standalone (installer-created local user) and managed (FreeIPA or AD)
  models, per-machine emergency account.
- Installer and kickstart integration; enrollment kept out of the image.

## Milestone 6: compliance and supply chain

Plan and decision: [ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md);
how-to: [supply-chain.md](supply-chain.md).

- **Done (v0.8.0):** `deskosctl sbom`, the deterministic declared
  (CycloneDX) SBOM of a Plan.
- **Done:** `TrustAnchor.blocklist` and the `deskos-trust` profile. A CA
  whose keys are compromised is distrusted as image intent: p11-kit reads
  the platform blocklist and `update-ca-trust` leaves the certificate out
  of every extracted bundle. Core ships the October 2026 eMudhra roots
  (RHSB-2026-012) and organizations can add their own at any layer. The
  entries go when the platforms' `ca-certificates` carries the vendor fix.
- **Ran and passed (2026-10-06, 0 Critical):** `.github/workflows/supply-chain.yml`
  builds the CentOS Stream 10 image, produces the installed SBOM with
  Syft and scans it with Grype, compares the declared SBOM against the
  installed one (`tests/supply-chain/compare.py`), and uploads the
  evidence. With `publish` on `main` it also pushes the scanned image and
  signs it **keyless** with `cosign`, attaching the installed SBOM as a
  CycloneDX attestation.
- Remaining for stage 3: channel promotion (candidate, canary, pilot, stable).
  Key-based signing for RHEL moved to a downstream organization's pipeline,
  which owns the entitled build host and signs with no OIDC and no transparency
  log (2026-10-14).
  The boot-check and scan gates are already unified in `supply-chain.yml`:
  one digest is built, scanned, booted and then signed.
- **Policy:**
  [ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md) — the gate
  blocks on Critical, High is tracked, and every exception is an explicit
  OpenVEX statement or an entry in the reviewed
  `tests/supply-chain/exceptions.json` record.
- Later: OpenSCAP evidence; candidate resources `TrustPolicy`,
  `ComplianceProfile`.

## Milestone 7: release promotion and update UX

- Channels (candidate, canary, pilot, stable) promoting one digest.
- **Done:** `UpdatePolicy`
  ([ADR 0007](adr/0007-updatepolicy.md)). Images stage the next image from a
  DeskOS timer, daily and on AC power in Core, and the bootc timer that reboots
  is masked; system Flatpak updates are the EL family's policy. Validated on a CentOS
  Stream 10 VM: the image service staged a new image without rebooting,
  and a manual reboot booted it with the previous one as rollback.
- **Done:** the `GnomeProfile` shell, clock and keyboard settings
  ([ADR 0008](adr/0008-gnome-shell-clock-keyboard.md)) and `TrustAnchor`
  ([ADR 0009](adr/0009-trustanchor.md)).
- **Done:** each image carries `/usr/share/deskos/image-info.json`, a small
  identity summary (workstation, platform, base image, contributing
  profiles) generated from the Plan. It is a reader's view, not a second
  authoritative record: `plan.json` stays that, and the distribution
  identity in `os-release` stays the vendor's.
- **This release (v0.9.0):** the `Theme` kind and the `deskos-theme` applier
  ([ADR 0013](adr/0013-theme.md)), and repository signing keys as **local
  assets** instead of URLs (`gpgKeyFiles`; a deliberate break of v1alpha1).
- A staged update skipped on battery waits for the next schedule; a
  catch-up run on AC connect (Bluefin's `uupd-on-ac.service`) is open.
- A local endpoint command (`deskos status`, `deskos update`,
  `deskos rollback`) that a user or administrator runs on one machine to
  inspect the deployment and invoke bootc's own staged update and
  rollback, system Flatpak updates and fwupd checks. **No controller,
  reconciliation loop or automatic enforcement**
  ([ADR 0001](adr/0001-artifact-factory-not-configuration-management.md)). It
  drives the units and reads the files listed in ADR 0007.
- Candidate resources: `TimeSyncPolicy`, `PerformancePolicy`,
  `FirmwarePolicy`, `IdentityProvider`, `WebApplication`.

## Open work (2026-10-11)

Carried out of the v0.9.3 and signed-image cycle; none is a milestone of
its own.

- **Signature identity for consumer enforcement (P0-03), decided 2026-10-25.**
  DeskOS Core follows projectbluefin: keyless OIDC signatures, no key file and
  no enforcement on the installed machine; a person verifies the digest
  manually. An organization that chooses enforcement keeps key-based signing;
  making that signature a policy requirement on update is that organization's
  work. The lab verifies that key-based `sigstoreSigned` policy blocks unsigned,
  wrong-key and wrong-identity images on `bootc switch` and `bootc upgrade`; the
  keyless Fulcio certificate used by Core cannot be pinned by containers/image,
  which matches issuer plus subject email while a GitHub Actions certificate
  carries a URI SAN. No upstream change that pins a GitHub URI-SAN is merged or
  committed. Core's P0-03 is closed as "signed and manually verifiable"; an
  enforcing organization still owns the Anaconda install path and key rotation.
  See [authenticity.md](design/authenticity.md).
- **Documentation restructure (in progress).** The platform matrix now has one
  home, [platforms.md](platforms.md), and the README states the
  `YAML → validate → compose → plan → Containerfile → image → disk` line. Still
  open: splitting `docs/resources.md` into
  `resources/{core,software,desktop,system}.md` and a `troubleshooting.md`
  (symptom → check → remedy). Nothing here documents a migration between
  versions: pre-1.0 there is no supported older version to migrate from. Keep
  `research-notes` a dated archive, not current status.
- **Durable SBOM storage (P0-05).** Decided 2026-10-25: R2, public bucket
  `deskos-evidence-public` served at `evidence.deskos.org`, a private bucket
  owned by the organization, a token scoped to the bucket or prefix, every
  published digest retained, and an unsigned `index.json` initially. Keys are
  `<namespace>/<repository>/sha256/<hex>/`, independent of the registry host.
  The publish attests the SBOM **by reference** (Rekor caps an attestation at
  100 KB; the installed SBOM is ~39 MB). The public bucket, domain, upload
  token and uploader are in place; the separate evidence job verifies the
  uploaded index and objects from `evidence.deskos.org`. The first real upload
  waits for the next published digest; the organization's private bucket token
  is separate. See [evidence-retention.md](design/evidence-retention.md).
- **Renovate.** `renovate.json` is in both repositories; deploying it (app
  or action, PAT, `registry.redhat.io` credentials) is open.
- **Flaky boot check.** The graphical-splash assertion has failed once on an
  otherwise good image (the disk booted; the splash was not captured). Make
  the check robust, or retry, so the gate is not intermittent.
