# Roadmap

What each milestone has done and what it left. **No dates are implied.** Fleet
configuration management is outside every milestone.

## Milestone 0: resource model (done)

Resources, schemas, layered composition, typed Plan, Containerfile backend, Core
on CentOS Stream 10 and RHEL 10, the example organization.

## Milestone 1: entitled RHEL build and boot (done, retired)

- The entitled build of `example-devops-rhel-10` (`0e65bf4b`) passed lint, the
  scans found no build-host identity, and both VM checks passed on its QCOW2 in
  the factory VM. That host was retired; RHEL now builds downstream, this
  repository builds CentOS Stream 10 only, and Ubuntu has its own repository
  ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).
- RHEL's Flatpak 1.16 reads `/usr/share/flatpak/preinstall.d`, checked with a
  private workstation that declares Flathub apps.

## Milestone 1b: visual identity and boot

Design: [visual-identity-and-boot.md](design/visual-identity-and-boot.md).

1. First-boot wallpaper from the effective wallpaper. Seen in a CS10 VM.
2. `appearance.loginLogo` for GDM, with the DeskOS lockup. Dock semantics done;
   the reference workstations select `Profile/deskos-dock`. A session
   `desktopLogo` is not started.
3. Boot intent lives in `BootProfile` ([ADR 0006](adr/0006-bootprofile-tenth-kind.md)).
4. Quiet graphical boot with the stock Plymouth theme, seen on CS10 UEFI VMs and
   on RHEL 10.2 through the retired host.
5. DeskOS splash watermark (`BootProfile.watermark`), seen at boot and shutdown.

## Milestone 2: independent CentOS CI build

- **CI build:** `vm-bootcheck.yml` builds both CentOS Stream 10 workstations into
  QCOW2 disks and runs the boot, session and first-boot checks, on pull requests,
  on `main` for image-changing paths, and on dispatch.
- **Publishing:** `supply-chain.yml` runs weekly and publishes by itself
  ([Release model](architecture.md#release-model)).
- **Pinned base:** both shipped Platforms pin a digest.
- **Example AlmaLinux and Fedora:** neither is shipped; their roots compose and
  render in CI, and `examples-weekly.yml` builds and boots them weekly
  ([example-platforms.md](example-platforms.md)).
- **Decided (2026-10-24):** the three vendor-scriptlet lint warnings stand, and
  `bootc container lint` stays in report mode
  ([architecture.md](architecture.md#lint-warnings)).

## Milestone 3: VM boot and desktop tests

Harness: [`tests/vm/`](../tests/vm/README.md). Design:
[visual-identity-and-boot.md](design/visual-identity-and-boot.md) ("VM QA").

- **Tier A, unmodified artifact:** `bootcheck.py` boots the exact test disk
  through an overlay, classifies screendumps (splash, graphical) and checks ACPI
  shutdown, without matching against references.
- **Tier B, instrumented session:** `sessioncheck.py` adds a test user and GDM
  autologin through systemd credentials (SMBIOS type 11), writes the report to the
  serial port, and runs the probes in
  [checks.md](checks.md#workstation-behavior-matrix), including AT-SPI and opening
  an application since 2026-10-29.
- **First boot:** `firstbootcheck.py` adds no account and no session, and asks
  whether the wizard came up and whether an account could log in.
- **Tier C lifecycle:** `tests/lifecycle/driver.py` ran A → B → A on CS10,
  including power loss mid-upgrade and a candidate that cannot boot, recovered by
  its composefs verity. `lifecycle.yml` ran the full path on GitHub-hosted runners
  (2026-10-26, run `40305080925`), weekly rather than per pull request; the
  ostree-write window is untested.
- **Test ISO:** `tests/installer/` installs unattended in QEMU
  (`installer-test.yml`, dispatch only; last green 2026-10-26, run `40266578605`).
  Nothing installs the **published** ISO: its storage is interactive (#294).
- **Installer payload pinning:** decided 2026-10-26
  ([installer-payload.md](design/installer-payload.md)); the installed system is
  the pinned digest.

**Remaining:** reboot and shutdown splash (recorded, not asserted: the guest
powers off in 0.4 s with one frame), reference matching (not planned), an
installed run of the published ISO (#294), and RHEL's checks downstream.

## Milestone 4: workstation E2E

Design: [session-e2e.md](design/session-e2e.md); prior art and what we take:
[session-e2e-prior-art.md](design/session-e2e-prior-art.md).

| Step | State |
|---|---|
| Session exposes its applications through AT-SPI | **done** 2026-10-29, run `40795106343` |
| Launch an application and see its window | **done** 2026-10-29, same run |
| Reach a Settings page | designed |
| Lock and unlock | designed |
| Open and close a window through the tree | designed |
| Promotion waits for E2E | not before the steps above exist |

Keyboard injection is deferred: Wayland needs the remote-desktop portal, not a
root helper, delivered without adding a package to the artifact.

## Milestone 5: identity and provisioning

- **Domain join against a disposable FreeIPA.** `tests/identity/` holds the
  client profile, container fixture and join script; `identity-freeipa.yml` runs
  daily and on demand. The session asserts the join, SSSD and the directory user,
  then logs in at GDM with a home from `oddjob-mkhomedir` (2026-10-27).
- **AD membership.** `tests/identity/samba/` and `identity-ad.yml` join through
  the realm's own DNS and log in as `qa@ad.lab.test`; an AD trust is a later step.
- **Next:** an interactive GDM login with a typed password, and a realm with its
  own DNS ([identity-validation.md](design/identity-validation.md)).
- **Corporate dconf lockdown.** `GnomeProfile.spec.locks` renders locks and the
  `lockdown` block models `org.gnome.desktop.lockdown.*`. The session check proves
  a locked key cannot change, `deskos report` lists the locks, and
  [locks.md](locks.md) says what they break; Core ships none.

## Milestone 6: compliance and supply chain

Plan: [ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md); how-to:
[supply-chain.md](supply-chain.md).

- Declared SBOM (`deskosctl sbom`), installed SBOM with Syft, Grype scan, the
  comparison of the two and keyless signing with the installed SBOM attached:
  passed 2026-10-06 with 0 Critical.
- `TrustAnchor.blocklist` and `deskos-trust` keep a distrusted CA out of every
  extracted bundle; Core ships the October 2026 eMudhra roots (RHSB-2026-012).
- Promotion of one digest without a rebuild runs after the weekly build with an
  approval record ([promotion.md](design/promotion.md)).
- [ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md) blocks on
  Critical, tracks High, and wants every exception as OpenVEX or in
  `tests/supply-chain/exceptions.json`.
- Later: OpenSCAP records; `TrustPolicy`, `ComplianceProfile`.

## Milestone 7: release promotion and update UX

- `candidate → stable` promotes one digest; canary and pilot are not planned at
  this scale.
- `UpdatePolicy` ([ADR 0007](adr/0007-updatepolicy.md)): staged image updates on
  a DeskOS timer, daily and on AC in Core, the rebooting bootc timer masked, and a
  battery skip caught up when AC returns.
- `GnomeProfile` shell, clock and keyboard settings
  ([ADR 0008](adr/0008-gnome-shell-clock-keyboard.md)), `TrustAnchor`
  ([ADR 0009](adr/0009-trustanchor.md)), `/usr/share/deskos/image-info.json`,
  `Theme` ([ADR 0013](adr/0013-theme.md)), local signing keys.
- `deskos status`, `update`, `rollback`, `etc`: one machine's deployment, bootc's
  staging and rollback, and the visible, reversible `/etc` three-way merge. No
  controller, no reconciliation
  ([ADR 0001](adr/0001-artifact-factory-not-configuration-management.md)).
- Candidate kinds: `TimeSyncPolicy`, `PerformancePolicy`, `FirmwarePolicy`,
  `IdentityProvider`, `WebApplication`.

## Closeout of the P0/P1 items (2026-10-28)

Every item of the [2026-10-20 review](future-backlog.md), with what closes it.

| Item | Status | Records |
|---|---|---|
| P0-01 actionable check failures | closed | attempt-suffixed artifacts (14 days) beside the retry; `identity.json`; failed-boot journal kept |
| P0-02 upgrade, rollback, state | closed, one residual | `lifecycle.yml` green 2026-10-26 (run `40305080925`); ostree-write interruption untested |
| P0-03 consumer-side enforcement | closed by decision | keyless signature, manual verification; an organization enforcing keeps key-based signing |
| P0-04 bound build authority, rehearse incidents | closed | incident exercise ran 2026-10-28 ([threat-model.md](threat-model.md)) and produced `PUBLISH_FREEZE`; one-person residual recorded |
| P0-05 digest-indexed records | closed | `evidence.deskos.org`; `promotion.yml` refuses unverified records; SLSA v1 provenance |
| P0-06 promotion of one digest | closed | first automatic move 2026-10-28, `sha256:871e59cc…`, ISO `:stable` `sha256:9803337b…` |
| P0-07 released-image risk over time | closed | rescan procedure in [supply-chain.md](supply-chain.md) |
| P0-08 upstream inputs deliberate | closed | input inventory in [supply-chain.md](supply-chain.md#inputs-and-who-moves-them) |
| P1-01 compatibility contract | closed | [compatibility.md](compatibility.md) |
| P1-02 platform support and retirement | closed | [platforms.md](platforms.md), [example-platforms.md](example-platforms.md) |
| P1-03 one adoption journey | closed, one residual | `examples/organization-and-role/WALKTHROUGH.md`; no outside reader has followed it |
| P1-04 current guidance, recovery | closed | [recovery.md](recovery.md), [troubleshooting.md](troubleshooting.md) |
| P1-05 diagnostics for outside authors | closed | [diagnostics.md](diagnostics.md) |
| P1-06 behaviour a vendor update can break | closed | the organization's field validation, 2026-10-27, digest `sha256:5e80b551…` |
| P1-07 physical hardware matrix | not a Core item | the organization owns the machine and the record |
| P1-08 predictable update behaviour | closed | the situation table in [operations.md](operations.md) |

## Proposals

### Secure Boot (proposed 2026-10-26)

Every installation runs with Secure Boot **off** today. Enabling it means shim →
GRUB → kernel all signed, or a unified kernel image. RHEL 10 ships the chain;
**CentOS Stream 10 does not boot with Secure Boot on** (its shim is blocked on
Microsoft review). An organization with its own key needs an owner, db or MOK
enrolment, signed GRUB/kernel and Microsoft signing for the default trust. The
installer, out-of-tree modules and two QEMU tests (a signed image boots, an unsigned
kernel is refused) belong to the item. **Decision:** vendor chain only, or a DeskOS
key with an owner and a rotation plan.

### Test organization (proposed 2026-10-26)

QA stays next to what it tests: determinism in Core, the built image where it is
built, Ubuntu in its own repository. A `deskos-qa` repository starts when a suite
consumes published digests from more than two repositories: it consumes digests,
never builds, holds read-only credentials and no signing key, and writes to the
same digest-keyed bucket.

### Repository templates (proposed 2026-10-29)

Two small repositories show the model without the publication machinery: an
internal CentOS Stream 10 image and an office RHEL 10 workstation with no developer
role. No new kinds, no escape hatches, and they call the released `deskosctl`.
Acceptance: a newcomer follows one, boots a machine, and writes down where they
got stuck.

### GNOME system administration coverage (audited 2026-10-29)

[gnome-coverage.md](gnome-coverage.md) gives every page of the [GNOME System
Administration Guide](https://help.gnome.org/system-admin-guide/) a verdict:
modelled, partial, artifact, a missing field, a candidate kind, or refused with the
invariant that refuses it. The vocabulary is **closed** (no raw dconf field), so
the audit found fourteen modelled exactly and four partially; the greeter's two
fields, the power pair, Looking Glass and the shield have shipped since, so the
table now reads nineteen and one. Three fields need no kind; a NetworkManager connection needs an ADR (secrets at runtime); a polkit
action file is a security surface; autostart, custom sessions and
single-application mode are refused under invariant 5. Next: one issue per field,
one PR per group, each with its check.

## Open work

- **P2-03, adoption beyond one organization.** The item asks for two external
  environments; one organization (private, the same maintainer) is not it.
- **An outside reader on the walkthrough.** Unproven until one does it.
- **Renovate outside Core.** `deskos-infrastructure` has no pins that move yet.
- **Installed run of the published ISO** (#294), by a person, recorded.

## Later

- **Retire the v1 repositories from the organization (proposed 2026-10-29).**
  `deskosproject` lists about 120 repositories. Eight are current: core,
  infrastructure, site, ubuntu, ubuntu-bootc, brand and the two templates. The rest
  is the 2016-2018 RPM packaging of the first attempt. They are noise in the
  organization list and in search, and none should be deleted: that history is the
  record of how the project got here.

  Plan: create `deskosproject-archive`, transfer every repository whose last push is
  before 2019 (the `*-rpm` set, the packager, livemedia and the old website),
  archive each one there so it is read-only, and set its description to point at
  the current project. GitHub redirects the old URLs, so links in old issues and
  posts keep working. Check before transferring anything that publishes a container
  image or a Pages site: package names follow the repository, and a Pages URL does
  not redirect.

  Acceptance: `deskosproject` lists only current repositories; the archived ones
  stay readable and clonable with their tags and releases; a sample of old links
  resolves.

- Lockfiles for binary artifacts and base digests, once release experience shows
  which inputs need them.
- `deskosctl track` resolves pins, verifies the SHA-256 and with `--fresh` reports
  a newer release; `track.yml` opens a weekly bundle, one commit per resource, for
  a person to merge, and nothing is published ([tracking.md](design/tracking.md)).
- **Enterprise use cases.** EDR, antivirus, VPN and proxy settings are organization
  software, never Core (invariant 7). A check may claim that software installs and
  that the machine, session and updates keep working; never vendor support, a
  licence or the deployment method. `docs/use-cases/` will split "what DeskOS
  does", "what it does not" and "how it is validated", each claim pointing at a run.
- **Model a network proxy, or leave it to configuration management** (pending).
  Only CA trust and desktop settings exist today; a proxy a machine needs to reach
  its own repositories is baseline, so it needs a resource and an ADR.
- **Example AlmaLinux and Fedora:** weekly builds, no publication, no support
  promise.
- The **splash race**: one disk booted to text for a whole boot and drew the
  splash on the next; the retry absorbs it but not always. Cause unexplained.
- RHEL's Red Hat logo in the Activities button: replacing `fedora-logo-icon`
  through an icon theme is untried.
- Applying the rest of a [Theme](adr/0013-theme.md) palette from a git URL:
  reading only `colors.toml` and `backgrounds/`, never running its code.
- **Skip the GNOME Initial Setup software page** and assert in the first-boot check
  that it is gone.
- **Website and documentation for two audiences:** a task-oriented manual with a
  screenshot per chapter, and bootc from zero for the developer ([style.md](style.md)).
- **Records on the site:** `deskos.org/evidence/` comes from the public index,
  follows `:stable` and describes today's download digest; a private
  organization's records never appear.
