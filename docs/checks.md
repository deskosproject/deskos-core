# Checks and what a red run means

What runs on this repository, what each check proves, the evidence it keeps, and
how to tell an infrastructure failure from an image defect. Required status
checks are a GitHub setting outside the repository; this page lists what runs,
not whether a branch protection rule is configured.

## What runs

| Workflow | Trigger | Proves | Evidence |
|---|---|---|---|
| `ci.yml` | pull request, `main`, called by release | resources validate and render deterministically | job log |
| `artifacts.yml` | pull request, `main`, dispatch | declared URLs resolve and bytes match their checksums; repositories look like repositories | job log and summary |
| `vm-bootcheck.yml` | pull request/`main` when resources, examples, code, schemas or the harness change; dispatch | the CentOS workstations build into QCOW2 and pass boot, session and first-boot checks | `bootcheck`, `session`, `firstboot`, failed-boot journal, frames, serial |
| `supply-chain.yml` | Mondays 06:17 UTC; dispatch | a platform builds, scans, compares declared and installed SBOMs, boots and — on the schedule, or a `publish=true` dispatch from `main` — pushes, signs and attests one digest | SBOMs, Grype report, digest, cosign verification, provenance, Plan, evidence directory |
| `iso.yml` | called by the gate; dispatch | the installer for a published digest is built, signed and tagged `sha256-<image digest>`; nothing is built from `main` | artifact digest, `iso.sha256`, cosign verification |
| `promotion.yml` | the gate's success (`workflow_run`); dispatch | `:stable` moves only to a digest whose signature and every evidence object verify, with an approval record | approval record, job log |
| `release-watchdog.yml` | daily 09:17 UTC; dispatch | `:stable` is not older than eight days, and the gate is dispatched when it is | job log |
| `lifecycle.yml` | Mondays 04:17 UTC; dispatch | install, stage B, boot B, roll back to A, interrupt a download and retry, fail a candidate and recover the previous entry | `lifecycle.json`, summary, serial logs, one journal per boot |
| `installer-test.yml` | dispatch | the test-only ISO installs unattended and writes its completion marker | serial log, frames, summary |
| `examples-weekly.yml` | Wednesdays 06:00 UTC; dispatch | AlmaLinux and Fedora examples build and boot; not published | boot/session/first-boot evidence |
| `release.yml` | tag `v*` | CI, release binaries, checksums and the `deskosctl` tool image | GitHub Release, signatures |
| `tool-image.yml` | `Containerfile` or compiler paths; dispatch | the `deskosctl` container builds and runs | job log |
| `mirror-fedora.yml` | dispatch | a Fedora base is mirrored to DeskOS Quay | job log |
| `renovate.yml` | daily; dispatch | opens pin-update pull requests; it never merges | pull requests |
| `identity-freeipa.yml` | daily 04:23 UTC; dispatch | a client image joins a disposable FreeIPA realm and the session proves the login | summary, frames |
| `identity-ad.yml` | daily 04:47 UTC; dispatch | the same, against a disposable Samba AD domain controller | summary, frames |
| `track.yml` | Mondays 05:17 UTC; dispatch | every checksum-pinned resource still resolves, and moves the ones the curated list covers, as a pull request | pull request |
| `evidence-maintenance.yml` | dispatch | repairs or prunes the digest-keyed evidence store; a prefix with an index is refused without `force` | job log |

A scheduled run has no dispatch inputs. `supply-chain.yml` falls back to
`fail-on: critical` explicitly, so the weekly run cannot silently run without a
severity threshold, and since 2026-10-28 the weekly run publishes: the gate,
signature, evidence and promotion are the same as a dispatch's. A dispatch with
`publish=false` is the dry gate.

## Reading a failure

Classify before changing anything:

- **Infrastructure or harness.** The job log shows a registry 503/EOF, a full
  runner disk, a missing firmware path, a permission error, or no runner. The
  image may be untouched. Fix the runner or retry; say which attempt produced
  the result. Do not "fix" the image for this class.
- **Image defect.** The build finished, then the boot, session or first-boot
  check failed. Read the serial log, the failed-boot journal, the frames and the
  structured summary before rerunning; a retry does not make an image boot.
- **Installer defect.** `installer-test` builds but never finds `Install
  finished`, or the screens never reach GNOME Initial Setup. That is the ISO,
  not the image build.
- **Unverified probe.** The artifact resolver reports a size above its bound as
  **not verified**, or a repository body as inconclusive; `deskos check` reports
  items as **not measurable here**. These are not passes. A check that could not
  observe its target stays red.

The VM harness already keeps `bootcheck-attempt1` beside the retried
`bootcheck` when the splash retry runs, so a retry is visible rather than
replacing the first attempt. Workflow artifacts also carry the run attempt in
their name (`…-attempt-2`), so a re-run does not overwrite the first attempt's
evidence.

## Tested and published identity

The publish path builds **one** image, boots that same image, and pushes it by
digest:

- `image.txt` records the local image ID;
- `digest.txt` records the registry digest that was pushed;
- `cosign-verify.txt` records verification against the pushed digest;
- `identity.json` links the base digest, the produced image digest, the Plan and
  the build context;
- a machine reports what it boots with `bootc status` and `deskos report`.

`:latest` is moved after the checks; `:<commit>` is a naming convention, not
registry-enforced immutability. A digest is the exact identity, which is why
anything beyond a lab pins one.

## Workstation behavior matrix

Ties a promoted image's claims to the probe behind them. **Instrumented** means
the check adds a test user, autologin or a session script; **unmodified** means
it boots the disk as shipped.

| Claim | Probe | Positive | Negative | Kind |
|---|---|---|---|---|
| Quiet graphical boot and splash | `bootcheck.py --expect-splash` | stable graphical frames and splash detected | text-only, blank or no stable frame | unmodified |
| GNOME Initial Setup on first boot | `firstbootcheck.py` | the wizard and its session are present | no wizard, or a different session | unmodified |
| Declared dconf defaults and vendor overrides | `sessioncheck.py` | each plan key reads back as declared | a value differs or cannot be read | instrumented |
| Declared extensions load | `sessioncheck.py`, `gnome-extensions info` | state `ACTIVE`/`ENABLED` | missing or `ERROR` | instrumented |
| Tray extension serves the session bus | `sessioncheck.py` | `StatusNotifierWatcher` present and accepts a registration | service absent or registration rejected | instrumented |
| Favorites list matches the plan | `sessioncheck.py` | `favorite-apps` equals the declared list, in order | differs | instrumented |
| Each favorite resolves to a desktop entry | `sessioncheck.py` | every id exists in a system or Flatpak application directory | an id has no `.desktop` entry | instrumented |
| Each favorite resolves to a command | `sessioncheck.py` | the entry's `Exec` names a binary the image ships | the command is missing, or a Flatpak app, which the Flatpak check owns | instrumented |
| Browser fetches harness content | `sessioncheck.py` | the guest fetches the harness page, the marker is in it, the fixture saw a Firefox request and a screenshot exists | fetch fails, marker missing, no browser request or no screenshot | instrumented |
| Login logo file exists | `sessioncheck.py` | the dconf logo names a file the image carries | the key is set and the file is missing | instrumented |
| Offline login | `sessioncheck.py --offline` (Core) | autologin reaches a session with no NIC and no default route | no session, or the boot kept a route | instrumented |
| Runtime tool reads the plan | `sessioncheck.py` | `deskos report` names the tested workstation | no report, or another workstation | instrumented |
| Runtime checker runs | `sessioncheck.py` | `deskos check` exits 0 (clean) or 1 (it found a difference) | it exits 2 or cannot run | instrumented |
| Screen locks on request | `sessioncheck.py` | `org.gnome.ScreenSaver.GetActive` is true after `Lock` | false or no answer | instrumented |
| Firefox runs | `sessioncheck.py` headless probe | the guest reports Firefox installed, headless exit 0 and a non-empty screenshot | non-zero or no screenshot; an image without Firefox says so | instrumented |
| System Flatpaks materialize | `sessioncheck.py` and `flatpak list --system --app` | every declared application id is installed | a declared id is missing | instrumented |
| Lifecycle A → B → A | `lifecycle.yml` | stage, boot B, roll back, local state, failed candidate and host-side recovery | wrong deployment, lost `/var`, failed retry, recovered wrong digest | unmodified, weekly |
| Test ISO installs unattended | `installer-test.yml` | serial marker and GNOME Initial Setup | timeout or marker missing | unmodified, dispatch |
| Published ISO carries the signed digest | `iso.yml` (called by the gate, dispatchable) | the digest's keyless signature verifies, the ISO is built from it, pushed signed, and tagged `sha256-<image digest>` | signature does not verify, build fails, or the annotation does not name the digest | from the published digest, not `main` |

A missing result, an unreadable value or a `not measurable` count is a failure
or an explicit unverified item, not a pass. The session row is an **instrumented
autologin session**, not proof of an interactive GDM login.

**Not proved by these probes:** an interactive GDM login (the probes autologin),
so the Wayland session type through GDM is unproved too; launching every
declared favorite; Flatpak install after reconnection; suspend, battery, display
and other hardware behavior (P1-07). Those are P1-06/P1-07 work, not implied
claims.

## What is not covered

- The shutdown splash is captured in the session run's frame list but not
  asserted: the guest powered off in 0.4 s and only one frame was captured, so
  the assertion would be a race. A reboot splash needs a boot that reboots.

- Bare metal, BIOS boot, Secure Boot and non-x86_64 are not tested.
- No check installs the **published** ISO; its storage is interactive, so only a
  person completes it. The test-only ISO has the `installer-test` job.
- The lifecycle check recovers a failed candidate through the previous GRUB
  entry from the host, not through a person selecting it in the menu. Rollback
  after a candidate reaches userspace is still manual.
- Bit-for-bit reproducibility of images is not claimed; compilation is
  deterministic, the image build is not yet bit-for-bit reproducible.
- RHEL composition and rendering are tested here; the entitled build and boot
  happen in the downstream organization.
