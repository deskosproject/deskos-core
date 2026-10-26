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
- **Decided (2026-10-24):** accept the three vendor-scriptlet warnings and keep
  `bootc container lint` in report mode
  ([architecture.md](architecture.md#lint-warnings)). A warning that originates
  in a DeskOS `RUN` step is a defect and blocks; the vendors' are tracked
  upstream.

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
  or qecore. Checks the dock, favorites and the command each launches, the
  wallpaper and login logo, headless Firefox against a page the harness serves,
  offline login, the shipped `deskos report` and `deskos check`, and failed
  units against the Plan; required checks *fail closed*.
- **First boot, instrumented, no account:** `firstbootcheck.py` adds no
  account and no session, and asks the guest whether the first-boot wizard is
  the session that came up, whether an account could log in, and whether a
  service failed.

**Remaining:**

- Every declared favorite resolves to a desktop entry and to a command the
  image ships (`sessioncheck.py`). Launching the application itself is still a
  field item: a GUI app that starts and exits is not a deterministic check.
- The browser fetches a page the harness serves on the host side of QEMU's
  network and returns its marker, the login logo's file exists in the image,
  and Core boots once with no NIC to prove the session logs in offline
  (`sessioncheck.py`).
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
  requests: it is too slow for every change, and the first scheduled run
  (2026-10-26, run `40305080925`) did the full path. The ostree-write
  interruption window is still untested.
- **Installer ISO:** the test-only ISO from `tests/installer/` installs
  unattended in QEMU (`installer-test.yml`, dispatch only; last green
  2026-10-26, run `40266578605`, on the published `sha256:96fe80d3…`) and
  requires the builder's "Install finished" marker. Nothing
  installs the **published** ISO: its storage is interactive, so only a person
  completes it. An organization's RHEL ISO is installed by hand; on 2026-10-23
  one shipped a `graphical --non-interactive` kickstart that RHEL 10.2's
  Anaconda aborted at the progress screen (`User interaction required on
  standalone spoke ProgressSpoke`), and the machine rebooted, which read as a
  hardware failure.
- **Installer tree pinning: decided (2026-10-26).** The installer's own payload
  (Anaconda, lorax) floats; the guard is the unattended install of the test-only
  ISO in `installer-test.yml`. The installed system is the pinned image digest;
  the ISO is the delivery vehicle. See
  [installer-payload.md](design/installer-payload.md).
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
  [resources/software.md](resources/software.md#rpmrepository)).
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
- ~~Mission statement, non-goals and values~~ done: [mission.md](mission.md)
  states the mission, the non-goals that already exist as invariants (ADR 0001,
  0002, 0005, 0011), and the values. The README links it.
- **Website and documentation for two audiences.** The site explains the model
  to a reader who already knows bootc and Flatpak. Restructure it around a
  normal user and a developer: a task-oriented manual with a screenshot in
  every chapter, an install page that shows the download, the checksum and how
  to try the image in a VM, and a first-run path for someone who has never used
  an image-based system. For the user, a plain page on installing software —
  Flatpak through Bazaar and `flatpak`, mise for command-line tools — and
  updates and rollback explained without jargon. For the developer, bootc from
  zero: image, deployment, update, rollback, and how a DeskOS plan becomes that
  image. The point is more screenshots and a beginner path that does not assume
  the vocabulary.
- **Evidence on the site (proposed 2026-10-26).** The digest-keyed evidence
  exists at `evidence.deskos.org`, but the site never uses it. Add an
  `/evidence/` page generated from the index — digest, platform, what the boot
  and session checks proved, the signature, and what was not measured — and a
  strip on the home page that names the verified digest and links to it. Read
  the public bucket only: an organization's private evidence never appears.
  Generated, never hand-written, so a claim cannot drift from the record.

### Secure Boot (proposed 2026-10-26)

Today every installation runs with Secure Boot **off**: the field checklist says
so, the recovery page says so, and no check covers it. Supporting it means the
firmware trusts every link from the ESP to the kernel:

- **The chain.** shim, signed by the Microsoft UEFI CA (or by the owner's db) →
  GRUB, signed by the distro or owner key and verified by shim → kernel, signed
  and verified by GRUB. The initramfs is not signed by this chain; a **unified
  kernel image** (kernel, initramfs and command line under one signature) is the
  alternative that closes it.
- **Who signs.** Core can rely on the platform vendor: RHEL 10 ships a
  Microsoft-signed shim and signed GRUB and kernel. **CentOS Stream 10 does not
  boot with Secure Boot on today** — its shim is blocked on Microsoft review
  (CentOS Stream 10 release notes, Known issues) — so the CS10 platform cannot
  claim support until that lands. An organization that wants its own key needs a
  named owner, enrollment in firmware db or MOK, signed GRUB/kernel/UKI, and
  Microsoft signing if it uses the default trust instead of a custom db.
- **The installer.** The ISO's shim and GRUB must be signed and the test-only
  install must boot with Secure Boot on; the published ISO's instructions stop
  telling people to turn it off only after that test exists.
- **Out-of-tree modules.** NVIDIA, `v4l2loopback` and similar are not signed by
  the vendor. With Secure Boot on they need MOK enrollment and per-kernel
  signing, or they will not load; an image that ships them must say which.
- **Tests.** QEMU/OVMF with Secure Boot on and the vendor keys: a positive test
  (the published image installs and boots) and a negative one (an unsigned
  kernel is refused). Extend `bootcheck.py`, `installer-test.yml` and the
  lifecycle disk. `mokutil --sb-state` is already collected by the field report.
- **Decision.** Vendor chain only, or a DeskOS key with an owner and a rotation
  plan (the signing side of P0-03, separate from the image `cosign` signature).
  Until the chain is signed and tested, the support statement stays: **Secure
  Boot off**, as `platforms.md` and `recovery.md` say.

### Test organization (proposed 2026-10-26)

QA stays next to what it tests for now: compiler determinism in Core, the built
image in DayTwo, Ubuntu in its own repository. A separate `deskos-qa`
repository starts when a nightly suite has to consume published digests from
more than two repositories. It then consumes digests and never builds, is
triggered by `workflow_run` or `repository_dispatch`, holds read-only pull
credentials and no signing key, and writes evidence to the same digest-keyed
bucket. Black-box tests move; unit and determinism tests stay with the code
they cover.

## Milestone 4: workstation E2E

- QEMU/KVM GNOME sessions tested through accessibility APIs, following
  Bluefin's factory practice.
- Promotion gates: only digests that passed E2E move forward.

## Milestone 5: identity and provisioning

- Standalone (installer-created local user) and managed (FreeIPA or AD)
  models, per-machine emergency account.
- Installer and kickstart integration; enrollment kept out of the image.
- **Domain join against a disposable FreeIPA (proposed 2026-10-26).** Prove the
  managed model end to end without a corporate directory: run FreeIPA in a
  container on the test host, join a throwaway installed VM with
  `ipa-client-install`, and log in as a directory user at GDM. It is the only
  way to exercise DNS, Kerberos, SSSD, PAM and the home-directory policy
  together, which unit tests cannot. It comes after the model exists — an
  organization profile that installs `ipa-client`, enrollment kept out of the
  image — and runs nightly or on demand, not on every pull request: it needs a
  privileged container or a second VM and several minutes. The evidence records
  the server version, the realm and the image digest; the container is a test
  fixture, never a support claim.
- **Corporate dconf lockdown (proposed 2026-10-26).** A managed workstation
  must be able to stop users from changing chosen settings, following
  [GNOME's dconf lockdown](https://help.gnome.org/system-admin-guide/dconf-lockdown.html).
  `GnomeProfile.spec.locks` already renders locks for the settings the model
  declares — Core never locks (invariant 8) — but a corporate fleet needs more:
  - the lock enum stops at the settings DeskOS models. The guide also locks
    printing, file saving, online accounts, logout and user switching, and
    `org.gnome.desktop.lockdown.*`. Decide between extending the enum and a
    controlled list of dconf keys/prefixes; either keeps the "no escape hatch"
    rule by validating what is declared;
  - **Done (2026-10-27):** the session check proves a locked key cannot be
    changed, not only that the lock file was rendered. The example organization
    locks `shell.favorites`; the session writes the key, expects the write to
    fail and the value to be unchanged;
  - `deskos report` and `deskos check` should list the active locks and say
    which are not enforced;
  - an organization-facing guide: which locks fit a managed laptop, what they
    break (printing, saving files, online accounts), and how they interact with
    user and site settings.

  This is an organization's policy layered like any other `GnomeProfile`; Core
  provides no lock of its own.

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
- **Done and exercised (2026-10-26):** `candidate → stable` promotion of one
  digest without a rebuild, with an approval record
  ([design](design/promotion.md)). The first promotion moved
  `sha256:96fe80d3…` to `:stable` (run `40317904401`); a second, after the
  Stream 10 base bump, moved `sha256:649aff75…` (run `40412048361`). No canary
  or pilot channel.
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

- ~~Channels promoting one digest~~ done for `candidate → stable`; canary and
  pilot are not planned at this scale.
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
- ~~Documentation restructure~~ done: [platforms.md](platforms.md) is the one
  platform matrix, the README states the
  `YAML → validate → compose → plan → Containerfile → image → disk` line,
  [troubleshooting.md](troubleshooting.md) covers symptom → check → remedy, and
  `docs/resources.md` is now an index over
  `resources/{core,software,desktop,system}.md`. Nothing here documents a
  migration between versions: pre-1.0 there is no supported older version to
  migrate from. Keep `research-notes` a dated archive, not current status.
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
