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
| `supply-chain.yml` | Mondays 06:17 UTC; dispatch | a platform builds, scans, compares declared and installed SBOMs, boots and (only on a `publish=true` dispatch from `main`) pushes and signs one digest | SBOMs, Grype report, digest, cosign verification, Plan, evidence directory |
| `lifecycle.yml` | dispatch | install, stage B, boot B, roll back to A, interrupt a download and retry | `lifecycle.json`, summary, serial log, one journal per boot |
| `installer-test.yml` | dispatch | the test-only ISO installs unattended and writes its completion marker | serial log, frames, summary |
| `examples-weekly.yml` | Wednesdays 06:00 UTC; dispatch | AlmaLinux and Fedora examples build and boot; not published | boot/session/first-boot evidence |
| `release.yml` | tag `v*` | CI, release binaries, checksums and the `deskosctl` tool image | GitHub Release, signatures |
| `tool-image.yml` | `Containerfile` or compiler paths; dispatch | the `deskosctl` container builds and runs | job log |
| `mirror-fedora.yml` | dispatch | a Fedora base is mirrored to DeskOS Quay | job log |
| `renovate.yml` | daily; dispatch | opens pin-update pull requests; it never merges | pull requests |

A scheduled run has no dispatch inputs. `supply-chain.yml` falls back to
`fail-on: critical` explicitly, so the weekly watch cannot silently run without
a severity threshold. Publishing is impossible from the schedule: it requires
`publish=true` on a dispatch from `main`.

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
| Screen locks on request | `sessioncheck.py` | `org.gnome.ScreenSaver.GetActive` is true after `Lock` | false or no answer | instrumented |
| Firefox runs | `sessioncheck.py --app`/headless probe | headless exit 0 and a non-empty screenshot | non-zero or no screenshot | instrumented |
| System Flatpaks materialize | `deskos-flatpak-preinstall` state and `flatpak list --system` | service succeeded and declared apps are installed | failed unit or missing app | instrumented |
| Lifecycle A → B → A | `lifecycle.yml` | stage, boot B, roll back, local state | wrong deployment, lost `/var`, failed retry | unmodified, dispatch |
| Test ISO installs unattended | `installer-test.yml` | serial marker and GNOME Initial Setup | timeout or marker missing | unmodified, dispatch |

A missing result, an unreadable value or a `not measurable` count is a failure
or an explicit unverified item, not a pass. The session row is an **instrumented
autologin session**, not proof of an interactive GDM login.

**Not proved by these probes:** an interactive GDM login and the Wayland session
type through GDM; launching every declared favorite; offline login; Flatpak
install after reconnection; suspend, battery, display and other hardware
behavior (P1-07). Those are P1-06/P1-07 work, not implied claims.

## What is not covered

- Bare metal, BIOS boot, Secure Boot and non-x86_64 are not tested.
- No check installs the **published** ISO; its storage is interactive, so only a
  person completes it. The test-only ISO has the `installer-test` job.
- The lifecycle check does not test a candidate that never boots, and rollback
  is manual.
- Bit-for-bit reproducibility of images is not claimed; compilation is
  deterministic, the image build is not yet bit-for-bit reproducible.
- RHEL composition and rendering are tested here; the entitled build and boot
  happen in the downstream organization.
