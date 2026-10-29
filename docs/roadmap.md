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
- **Publishing:** `supply-chain.yml` runs weekly on `main` and publishes by
  itself: it builds, scans and boots each platform's image, then pushes and
  signs it to that platform's repository
  (`quay.io/deskos/deskos-core-centos-stream-10`) as `:latest`, `:candidate` and
  `:<commit>`, with the installer ISO as a signed OCI artifact beside it.
  `promotion.yml` follows the gate's success and moves `:stable` to that digest
  with a recorded approval. A dispatch with `publish=true` does the same on
  demand. CentOS Stream 10 today, with the
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
  maintenance step, not part of compilation. The rule is BuildStream's
  `track:` idea: resolve the pin, propose the move, and let a person review it.
  Renovate covers the rest.
  - **Done (2026-10-27):** `deskosctl track [--fresh] ROOT...` finds every pin in
    the tree, downloads the pinned URL and compares its sha256, and with
    `--fresh` reports a newer GitHub release. It lives in `cmd/deskosctl/track`
    because `internal/` may not reach the network, and it never changes a Plan.
  - **Done (2026-10-27):** `track.yaml` holds the curated tag-to-version rule per
    repository, `--write` moves a pin on its own line (indentation, quoting and
    trailing comment intact), and `track.yml` opens a weekly bundle with one
    commit per resource plus the regenerated goldens. A person reviews and
    merges; nothing is published. See [tracking.md](design/tracking.md).
- ~~Immutable identity for RPM repository GPG keys~~ done: `RpmRepository`
  keys are local assets read into the plan (see
  [resources/software.md](resources/software.md#rpmrepository)).
- ~~APT sources as their own kind~~ done: `AptRepository` renders a signed
  deb822 source and its keyring, and a `PackageSet` names it through
  `aptRepositories` (ADR 0015).
- **Enterprise use cases: what we claim and how we say it (proposed
  2026-10-27, separate spike).** The documentation is where adoption is won or
  lost, and the risk is a sentence that reads "DeskOS comes with an EDR". It does
  not. The pipeline proves that an organization's software fits a DeskOS baseline
  and that the machine stays healthy; it does not ship that software and it is
  not how a fleet is deployed.
  - **Never "DeskOS comes with X".** DeskOS composes a declarative baseline; the
    organization declares its software in its own layer. EDR, antivirus, VPN and
    proxy settings are organization software, never Core (invariant 7).
  - **What a check may claim:** the software installs from the declared source,
    the machine boots, the session works, and an update leaves it working. Not
    vendor support, not a licence, not the production deployment method.
  - **How it is deployed in real life:** configuration management — typically
    Ansible — and the software's own console, outside these repositories. The
    same is true of enrolment: a real machine receives a per-host credential at
    provisioning time. Nothing about that belongs in an ISO.
  - **No secrets in the ISO, and no per-laptop ISO.** A shared credential baked
    into an image compromises every machine that ever uses it; an ISO prepared
    per laptop is not a fleet artifact. Enrolment and configuration are runtime
    acts, and the artifact carries neither.
  - **Deliverables:** `docs/use-cases/` pages for identity, EDR, antivirus, VPN
    and the corporate proxy, each split into "what DeskOS does", "what it does
    not" and "how it is validated"; an FAQ for "does DeskOS ship an EDR?"; and a
    pointer from every claim to a run or a page. A claim with neither is not
    made.
  - **Positioning:** a fleet artifact proves what a machine can do after it boots,
    and proving it in CI is the part few projects do. Compare on method and
    evidence, never on the reputation of another project.
- **Model a network proxy, or leave it to configuration management (decision
  pending, 2026-10-27).** Nothing in the compiler describes a proxy today: dnf,
  Flatpak and `deskos update` behind a corporate proxy cannot be expressed, only
  the CA trust (`TrustAnchor`) and the desktop settings (`GnomeProfile`). If a
  machine needs the proxy to reach its own repositories, the proxy is baseline
  and deserves a resource of its own — a new kind, which needs an ADR and an
  explicit decision (invariant 4). If it is environment, Ansible owns it. The
  proxy check starts with what can be expressed today and leaves this open.
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
  `/etc/gnome-initial-setup/vendor.conf` with `skip=software`, and assert in the
  first-boot check that the page is gone.
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
- ~~Evidence on the site~~ **done, 2026-10-28.** `deskos.org/evidence/` is
  generated from the public index — digest, platform, what the boot and session
  checks proved, the signature, and what was not measured — and it follows
  `:stable`, so it describes the digest people download today. The home page
  names that digest too, stamped by the same weekly workflow
  (`tools/stamp-digest.py`), never typed. The site reads the public bucket only:
  an organization's private evidence never appears.

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
image in the repository that builds it, Ubuntu in its own repository. A separate `deskos-qa`
repository starts when a nightly suite has to consume published digests from
more than two repositories. It then consumes digests and never builds, is
triggered by `workflow_run` or `repository_dispatch`, holds read-only pull
credentials and no signing key, and writes evidence to the same digest-keyed
bucket. Black-box tests move; unit and determinism tests stay with the code
they cover.

### Repository templates (proposed 2026-10-29)

Two small repositories that show the model **without** the publication machinery,
because most people who use DeskOS are not publishing an image to the world.

- **An internal CentOS Stream 10 image.** One platform, one baseline profile, one
  workstation, one workflow: `validate`, `render`, `podman build`, push to the
  organization's registry. No evidence index, no promotion, no ISO, no signing —
  and the template says so in its first line, because an image produced this way
  carries no evidence and must not be published as the reference. It keeps the
  part that cannot be given up: deterministic composition and the pin checks.
- **An office RHEL 10 workstation.** One platform (the organization's entitled
  RHEL), a baseline with what an office actually uses — fonts, browser, office
  suite, printing, the organization's own CA, an update policy — and no developer
  role: no virtualization stack, no container tooling, no IDE. The shape is the
  one the organization's private RHEL repository already builds and installs,
  reduced to an office. It stays private, signs with the organization's own key if
  it signs at all, and is never published.

Both are templates, not a second product: no new kinds, no escape hatches, and
they call the released `deskosctl`. The acceptance is the same for each — someone
who has never built a DeskOS image follows it, ends with a booted machine, and
writes down where they got stuck.

### GNOME system administration coverage (proposed 2026-10-29)

Read the [GNOME System Administration
Guide](https://help.gnome.org/system-admin-guide/) end to end — Setup, User
settings, Appearance, Login, Network & hardware, Software, Other — with every
linked page, and decide for each item whether it is **expressible today**, a
**missing field**, a **candidate kind**, or **deliberately out of scope**.

The guide is the closest thing GNOME has to a list of what an administrator
changes on a machine. The model's credibility depends on being able to say which
of those a DeskOS image can carry and which it refuses, with a reason.

What the first pass shows, before writing anything:

- **Expressible today** as `GnomeProfile` dconf defaults and locks, because a
  default is a typed key at a path and the model does not restrict which one:
  idle dim and brightness, screen blank and lock, the compose key, the greeter's
  banner text and user-list switch, the lock-screen shield, and any typed
  wallpaper key.
- **Two resources, not one.** Extra fonts and extra backgrounds are a
  `BinaryArtifact` placing the files plus a `GnomeProfile` key naming them; the
  guide's `/usr/share/...` step is the artifact's destination, and the model
  already says so.
- **Missing fields worth a decision.** The GDM dconf profile carries a `file-db`
  line written by the platform's package; the check must stay on the greeter
  actually reading the database we write, not on the file existing.
- **Candidate kinds, each needing an ADR:** a file association (`mimeapps.list`),
  a NetworkManager connection (VPN and 802.1X, secrets supplied at runtime and
  never in the image), a custom session file, and an NFS home profile (the
  `service-db:keyfile/user` line). **Autostart entries and kiosk mode are out**:
  both are an arbitrary `Exec` in a directory, which invariant 5 refuses, and a
  kiosk is a session definition, not a workstation default.
- **Out of scope, said plainly:** per-user files under `$HOME`, fingerprint
  readers the image does not drive, and anything that only a running desktop can
  change.

Deliverable: `docs/gnome-coverage.md`, one table row per guide page with the
verdict and the reason, linked from the roadmap and from
`resources/desktop.md`; then one issue per missing field or candidate kind, each
naming the ADR it needs. The guide is a menu, not a backlog: declining an item
with a reason is a result, and the count of new kinds stays at fourteen until an
ADR says otherwise.

## Milestone 4: workstation E2E

**Designed 2026-10-29, not implemented.** See
[session-e2e.md](design/session-e2e.md): the transport the session check already
uses, an accessibility driver inside the guest speaking AT-SPI through the binding
the image already carries (`python3-pyatspi`, confirmed in the installed SBOM),
and four bounded scenarios —
launch an app, reach a Settings page, lock and unlock, open and close a window —
asserted on the accessibility tree and not on pixels.

- QEMU/KVM GNOME sessions driven through accessibility APIs, the way a person
  would use them, so a check fails where a user would be stuck.
- Promotion gates: only digests that passed E2E move forward. That needs the first
  part implemented before it is worth promising.
- **Prior art, surveyed 2026-10-29.** Three shapes exist, and none of them is a
  drop-in dependency:
  - **Distribution-level OS testers** install an image in a VM, send synthetic
    input and compare the screen against stored reference images. Cover the
    installer and the desktop, and cost most in re-recording references when the
    UI moves.
  - **Desktop automation through the accessibility tree**, driven by a Python
    binding with a helper that bridges input over Wayland's remote-desktop
    portal. This is what the vendors' own desktop QA uses, and at least one
    image-based distribution ships it as a reusable pull-request gate.
  - **Post-build gates** that boot a published digest and run upgrade, rollback
    and persistence scenarios. `lifecycle.yml` already is this shape.

  We are the second shape, with the third already in place. Two things are worth
  taking from the survey: the **Wayland input bridge** (through the desktop
  portal, which is supported, instead of a root helper writing to `/dev/uinput`,
  which is not), and the **10–15 minute budget for a full pass**, which is what
  makes a suite survive as a gate. Nothing surveyed removes the need for our own
  transport: the artifact stays unmodified, and a suite that needs an agent
  baked into the image is not a check on the artifact.

## Milestone 5: identity and provisioning

- Standalone (installer-created local user) and managed (FreeIPA or AD)
  models, per-machine emergency account.
- Installer and kickstart integration; enrollment kept out of the image.
- **Domain join against a disposable FreeIPA (proposed 2026-10-26).** Prove the
  managed model end to end without a corporate directory: run FreeIPA in a
  container on the test host, join a throwaway installed VM with
  `ipa-client-install`, and log in as a directory user at GDM. It is the only
  way to exercise DNS, Kerberos, SSSD, PAM and the home-directory policy
  together, which unit tests cannot.
  - **Done (2026-10-27):** `tests/identity/` carries the organization client
    profile (`ipa-client`, `oddjob-mkhomedir`, no realm or credentials), a
    disposable FreeIPA container fixture and the join script; `make check`
    validates the workstation.
  - **Done (2026-10-27):** `.github/workflows/identity-freeipa.yml` runs daily
    and on demand, never on every pull request: it starts the pinned FreeIPA
    container (on the bridge network — the image supports neither privileged
    mode nor host networking), builds the identity image and disk, boots the VM
    and joins it from a unit before GDM with runtime credentials. The session
    asserts the join completed, SSSD is active and the directory user resolves;
    the run of 2026-10-27 reports `join_ok=yes`, `join_sssd=active` and
    `join_user=qa:…:/home/qa:/bin/sh`.
  - **Done (2026-10-27):** the same check logs in at GDM as the directory user
    and asserts the session is that user, that `oddjobd` is active and that the
    home did not exist before the login and belongs to the uid SSSD resolved
    afterwards — `oddjob-mkhomedir` created it, rather than a directory prepared
    in advance. Autologin exercises PAM's session phase, the greeter and the
    home policy together.
  - **Next:** an interactive GDM login, with a person typing the directory
    password at the greeter; autologin does not prove the password prompt. AD,
    and a realm with its own DNS, remain separate paths. The evidence records
    the server version, the realm and the image digest; the container is a test
    fixture, never a support claim.
- **AD membership (proposed 2026-10-27).** The managed model also means AD, and
  the identity fixture can prove the client path without Windows: a Samba AD DC
  in a container, `realm`/`adcli`/`sssd-ad` in the VM, DNS pointed at the realm,
  and a login at GDM as an AD user. Samba AD is a real AD implementation but not
  Windows: Group Policy and Windows-specific behaviour stay unclaimed.
  - **How it is validated:** [identity-validation.md](design/identity-validation.md)
    explains the shape — disposable realm, credentials at runtime, join before
    GDM, session and home asserted, watchdog for a boot that never reports — and
    why that, not the join itself, is the fleet property.
  - **Done (2026-10-27):** `tests/identity/samba/` builds the DC and
    `identity-ad.yml` runs daily and on demand. The check joins the VM through
    the realm's own DNS, logs in at GDM as `qa@ad.lab.test`, and asserts the home
    was created at that login and `oddjobd` is active. An AD trust (FreeIPA ↔
    AD) is a later step and needs both realms reachable at once.
- **Corporate dconf lockdown (proposed 2026-10-26).** A managed workstation
  must be able to stop users from changing chosen settings, following
  [GNOME's dconf lockdown](https://help.gnome.org/system-admin-guide/dconf-lockdown.html).
  `GnomeProfile.spec.locks` already renders locks for the settings the model
  declares — Core never locks (invariant 8) — but a corporate fleet needs more:
  - **Done (2026-10-27):** the `org.gnome.desktop.lockdown.*` capabilities are
    modeled as a `lockdown` block in `GnomeProfile.defaults` — printing, print
    setup, save to disk, logout, user switching, application handlers, command
    line, password reveal, lock screen, removable storage read-only and user
    administration — and their semantic keys are lockable like any other.
    Online accounts and repartitioning are not modeled: they are separate
    mechanisms, not one boolean key;
  - **Done (2026-10-27):** the session check proves a locked key cannot be
    changed, not only that the lock file was rendered. The example organization
    locks `shell.favorites`; the session writes the key, expects the write to
    fail and the value to be unchanged;
  - **Done (2026-10-27):** `deskos report` lists the locks the plan declares and
    says that enforcement is not its measurement; `deskos check` names the locks
    in its scope and points at the instrumented session check;
  - **Done (2026-10-27):** [locks.md](locks.md) covers which locks fit a managed
    laptop, what they break, and how they interact with user and site settings;
    `resources/desktop.md` links it from the `GnomeProfile` section.

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
- **Done (2026-10-27):** a staged update skipped on battery catches up when AC
  comes online: the image carries a udev rule that starts the same staging
  service, which still checks `ConditionACPower`.
- **Done (2026-10-26):** `deskos status`, `deskos update` and `deskos rollback`
  let a user or administrator inspect one machine's deployment and invoke
  bootc's own staged update and rollback. **No controller, reconciliation loop
  or automatic enforcement**
  ([ADR 0001](adr/0001-artifact-factory-not-configuration-management.md)); the
  commands read the files listed in ADR 0007. **Done (2026-10-27):**
  `deskos update --flatpak` updates system Flatpak applications before the
  image; `--firmware` lists firmware updates and never applies them; `--all`
  does both plus the image. The next release carries the companion that knows
  them.
- **Done (2026-10-27):** `deskos etc` makes the `/etc` three-way merge visible
  and reversible. It compares the running `/etc` with the booted image's
  baseline (`/usr/etc` on the ostree/bootc backend) and lists modified, added
  and removed paths. `deskos etc reset PATH…`, `--all`
  and `--dry-run` copy the baseline back, keep a timestamped copy under
  `/var/lib/deskos/etc-backup/`, and refuse a path the image does not own unless
  asked; `--all` names the local additions it would delete before doing it. It
  only works on a backend that keeps a baseline and merges `/etc`; elsewhere it
  refuses and says why. It is also the way back after rebasing a machine from
  another ostree distribution, where resetting all of `/etc` to the DeskOS
  baseline is the point — a narrow case, but the same tool.
  Runtime files the image cannot carry — NetworkManager connections,
  machine-id, SSH host keys, printer and Bluetooth state, sockets, caches — are
  ignored by default, and an organization can extend the list. The policy file
  is **`/etc/deskos/deskos.json`**: an image-owned default, not a hand-edited
  escape hatch. JSON was chosen because the runtime parses it with the Go
  standard library and it needs no comments; a line-oriented `deskos.conf`
  would win only if the list were meant to be edited on the machine, which is
  not this design. How an organization ships the file (a `Profile` field or a
  first-class resource) is still open; the runtime reads it when it is there.
  The tool ignores
  `/etc/deskos/` in both the diff and the reset: the policy is not a managed
  file. If it differs from the image, it says so separately and suggests
  rebuilding the image; `reset --all` never touches it unless asked with
  `--include-policy`. Ignored paths are counted and named in a summary, never
  silently dropped; content is shown only on request because `/etc` holds
  secrets. No daemon, no reconciliation, no enforcement: a person asks, and the
  tool reports or acts (ADR 0001, 0018).
- Candidate resources: `TimeSyncPolicy`, `PerformancePolicy`,
  `FirmwarePolicy`, `IdentityProvider`, `WebApplication`.

## Closeout of the P0/P1 items (2026-10-28)

Every item of the [2026-10-20 review](future-backlog.md), with the evidence that
closes it. "Closed by decision" means the item asked for a decision and the
decision is recorded; "open" means the gap is named, not that it is forgotten.

| Item | What it was | Status | Evidence |
|---|---|---|---|
| P0-01 | Make gate failures actionable and preserve the tested identity | **closed** | attempt-suffixed artifacts (14 days) beside the retry; `identity.json` with commit, run, plan and manifest hashes, image ID and digest; the failed-boot journal is kept and uploaded |
| P0-02 | Prove upgrade, rollback and persistent-state recovery | **closed, one residual** | `lifecycle.yml` green 2026-10-26 (run `40305080925`): stage, boot B, roll back, power loss and a candidate that cannot boot, recovered by composefs verity. The ostree-write interruption window is still untested |
| P0-03 | Establish consumer-side authenticity enforcement | **closed by decision**, 2026-10-25 | keyless OIDC signature and a documented manual verification; Core does not enforce a policy on update. An organization that wants enforcement keeps key-based signing and owns the install path and key rotation |
| P0-04 | Bound build authority and rehearse supply-chain incidents | **closed**, 2026-10-28 | the tabletop ran the same day (scenarios A, C and E, in [threat-model.md](threat-model.md)): it produced `PUBLISH_FREEZE`, which stops the gate, the installer, the promotion and the mirror from the repository settings, and recorded the one-person residual. Build authority is bounded in [promotion.md](design/promotion.md): the automatic path moves only its run's digest, after the signature and every evidence object verify |
| P0-05 | Preserve digest-indexed evidence and verifiable provenance | **closed** | `evidence.deskos.org` serves the index and objects of a published digest; `promotion.yml` refuses evidence that does not verify; the image carries a SLSA v1 provenance predicate |
| P0-06 | Design promotion of one tested digest | **closed** | the first automatic move, 2026-10-28: `sha256:871e59cc…` from `sha256:649aff75…`, record `approvals/…/20261028T161205Z-871e59cc….json` (`automatic (gate)`), ISO `:stable` `sha256:9803337b…` |
| P0-07 | Review released-image risk throughout its useful life | **closed** | the rescan procedure in [supply-chain.md](supply-chain.md): select a published digest, record scanner and database versions, and route a new Critical to revalidation, a dated exception or replacement/withdrawal, recorded on an issue with the digest and the matching package |
| P0-08 | Make upstream-input changes deliberate and verifiable | **closed** | the input inventory in [supply-chain.md](supply-chain.md#inputs-and-who-moves-them) with owner, trigger and check per input; resolver and checksum tests; ADR 0019 and ADR 0021 for download capability and pinned extensions; the base re-pin and the exception re-review of 2026-10-28 are worked examples |
| P1-01 | Define the public compatibility contract | **closed** | [compatibility.md](compatibility.md): surfaces, versioning and migration, a fixture matrix, beta/1.0 criteria and explicit non-promises |
| P1-02 | Bound platform support, adoption and retirement | **closed** | [platforms.md](platforms.md) is the single matrix with security maintenance, upgrade paths and retirement; [example-platforms.md](example-platforms.md) draws the example boundary; [SECURITY.md](../SECURITY.md) and [support.md](support.md) are the entry points |
| P1-03 | Complete one public organizational adoption journey | **closed, one residual** | `examples/organization-and-role/WALKTHROUGH.md`: one pinned exercise from compiler to a booted UEFI VM, with the pins, prerequisites, Plan and SBOM, trust versus lab images, update source, and what it exposed. The residual is named in Open work: no reader from outside the project has followed it |
| P1-04 | Correct current guidance and make recovery discoverable | **closed** | [recovery.md](recovery.md) and [troubleshooting.md](troubleshooting.md) are symptom → check → remedy; the stale AppIndicator and Ubuntu-era claims were corrected, not restated |
| P1-05 | Make existing diagnostics useful to outside authors | **closed** | [diagnostics.md](diagnostics.md): resource and composition failures, download and build inputs, runtime evidence, and what is deliberately not a diagnostic |
| P1-06 | Test the workstation behaviour a vendor update can break | **closed** | the organization's `docs/field-validation.md`, recorded 2026-10-27, digest `sha256:5e80b551…`: interactive login, lock, logout with Wi-Fi off, offline Flatpak and reconnect, the favorite |
| P1-07 | Record a small physical-hardware validation matrix | **not a Core item**: the evidence belongs to the organization that owns the machine | the organization's `docs/hardware-validation.md` writes `hardware-report-<timestamp>/` and keeps the model, the serial and the screenshots private, in a private repository. Core states the scope — one model, best effort, never a compatibility claim — and never the identifiers |
| P1-08 | Make update behaviour predictable to users and operators | **closed** | the situation table in [operations.md](operations.md): automatic on and off, battery skip, staged with and without a window, machine off, registry unreachable, interrupted download, metered connection, Flatpak independent |

That is the review closed as a tracking document: twelve items done or decided,
one outside Core, and **two gaps named** — the incident rehearsal (P0-04) and the
adoption journey (P1-03). A number with an owner and a gap is a status; a number
nobody can look up is not.

## Open work

- **P2-03, adoption beyond one organization.** One organization records its own
  resource root and its field evidence (private, the same maintainer); the item
  asks for **two external environments**, and a second pair of eyes is the whole
  point. Named here so the first environment is not mistaken for the validation.
- **An outside reader on the walkthrough.** P1-03 is closed on the document and
  the run behind it; the part that cannot be manufactured from inside — a
  stranger following it without asking — is unproven until one does.
- **Renovate outside Core.** Core runs it daily (`renovate.yml`, `RENOVATE_TOKEN`)
  and it opens the pin pull requests; `deskos-infrastructure` has no
  `renovate.json` and no pins that move, so there is nothing to deploy there until
  it does.
- **Flaky boot check.** The graphical-splash assertion failed once on a good
  image because the harness sampled the screen once a second and the splash can
  last under one. It now samples every 0.25 s until the splash or the graphical
  screen is seen, and the boot checks of #282 passed with the assertion on.
