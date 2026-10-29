# VM harness

`bootcheck.py` boots an **unmodified** disk in QEMU with UEFI firmware, records
the screen until it is stably graphical, then shuts down through ACPI. The disk
is never written: each run boots a throwaway qcow2 overlay in its own output
directory. `sessioncheck.py` adds an instrumented boot that checks the GNOME
session against the plan; `firstbootcheck.py` asks whether the first-boot wizard
is the session that came up.

**Requirements:** Linux with `/dev/kvm`, QEMU, `qemu-img`, OVMF and Python 3 (no
third-party modules). It runs unprivileged, with user-mode networking.

## Building a test disk

```bash
deskosctl render --workstation deskos-core-centos-stream-10 --output ctx
sudo podman build -t localhost/deskos-core-centos-stream-10:test ctx
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args localhost/deskos-core-centos-stream-10:test
```

`--no-default-kernel-args` drops bootc-image-builder's `console=ttyS0`, so the
disk boots with the image's own arguments. A platform that publishes no image can
be written with `bootc install to-disk` instead; the harness detects the backing
format, so a raw disk and a QCOW2 are both accepted. `--target-imgref` must be the
reference the installed system will update from, or the lab path is recorded and
the next update would replace the derivative with its base.

## Running

```bash
python3 tests/vm/bootcheck.py --disk output/qcow2/disk.qcow2 --out run-1 --expect-splash --hash-disk
```

Exit `0` passes, `1` fails, `2` is a harness error. A run fails when no stable
graphical screen appears within `--boot-timeout`, when `--expect-splash` sees no
splash, or when ACPI shutdown does not power off in time.

`run-1/` holds `summary.md`, `summary.json`, one PNG per distinct frame,
`serial.log` and `qemu.log`. Frames are classed `blank`, `text` (firmware, GRUB,
console), `splash` (dark, lower-middle spinner and watermark) or `graphical`.

> [!NOTE]
> These are pixel heuristics: the harness does not identify *which* graphical
> screen it sees, and does not test applications. The frames are the evidence.

## Session check

The same disk, again through an overlay, with systemd credentials passed as SMBIOS
type 11: a `deskos-qa` user with GDM autologin, a script that records what GNOME
reports, and a unit that writes results and journals to the serial port and powers
off. Kernel arguments and Plymouth are unchanged, but this is an **instrumented
boot** and is reported as one.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json --out session-1
```

`--offline` boots with **no NIC** and asserts the session still logs in, checking
that the guest really has no default route. Browser and Flatpak checks are skipped
there because they need the network.

Expectations come from the image's `plan.json`: enabled extensions (including Dash
to Dock), favorites order, each favorite's desktop entry and the command it
launches, the login logo's file, the wallpaper, and `disable-user-extensions`
false. It also requires headless Firefox to fetch a page served by the harness on
`10.0.2.2`, `deskos report` to name the workstation, `deskos check` to run (exit 0
or 1), every declared system Flatpak installed, no failed user units and no
unexpected failed system units. Results are in `session.md`, `session.json` and
`journal.txt`, with frames as above.

### Every default the plan declares

The lists come from the plan, so a default declared later is asserted without
editing the harness. Two delivery paths:

- **dconf defaults** (`artifact.dconf.defaults`): each key read with `dconf read`.
- **GSettings vendor defaults** (`artifact.gsettingsVendorDefaults`): each schema
  and key read with `gsettings get`; the path a `*.gschema.override` takes, and
  the only one that delivers a default where the platform's dconf profile does not
  read the DeskOS database.

Each default is **two claims**, read separately:

- **The image carries it.** A dconf default is also read through a profile naming
  only the image's own database; a vendor default through
  `GSETTINGS_BACKEND=memory`. A declared default the image does not carry fails,
  whether or not the session happens to show the right value.
- **The session ends up with it.** On a fresh install a differing session value
  fails: a default that does not reach the user is the defect. A key the desktop
  itself rewrites on first login is declared in `EXPLAINED_SESSION_DIFFERENCES`
  with its reason, naming the delivery path (`dconf:<key>` or
  `gsettings:<schema> <key>`), because one key can be declared on both.

A read the guest cannot make is **not** a contradiction: it is reported as
unverified with the reason (the image carries the default but the profile does not
reach it). `session.json` carries `plan_defaults` with every unverified read, its
axis, state and reason; `session.md` prints the counts.

Arrays compare **in order** except the keys in `UNORDERED_DEFAULTS`
(`enabled-extensions`, `custom-keybindings`), where GNOME treats the value as a set
and rewrites the list; adding a key there is a claim about GNOME and carries a
reason. `@as []` equals `[]` for an `as` key; `@ai []` does not.

Both lists travel as files of their own so the script stays inside the tmpfiles
cap.

`platform_diagnostics` records CPU vendor, family and model, `edac_mce_amd` and
`/dev/mcelog` state, `mcelog.service` properties and up to 100 kernel `mce`/`edac`
lines; missing data is `unknown`.

### Every binary the plan installs

The plan carries each binary's destination, mode and the checksum its download was
verified against; the guest reads the file and the host compares.

- **Single-file artifact** (`archive: none`): the plan's checksum *is* the file's,
  so the bytes are compared and a mismatch fails.
- **Archive** (`archive: tar.gz`): the plan pins the archive, which is gone by
  session time. The file is verified as installed with its declared mode, and the
  report says which claim it could make. Comparing the two anyway once reported a
  difference that was not one, on the example workstation's own `openshift-client`;
  a real boot found that.

Mode is asserted in both cases, because the plan declares it; `stat` prints `755`
where the plan writes `0755`. A read that did not happen is reported as unread, not
as a mismatch. Both reads follow a link, so a symlink describes the file that runs.

`session.json` carries `plan_binaries` and `session.md` the counts; `unreadable`
names which axis could not be read.

### Seeding the skeleton, showing widgets

- `--seed-skel` copies `/etc/skel` into the test user's `$HOME`, as `useradd -m`
  does (the user comes from `sysusers`, which does not). Without it the image's
  per-user seeds never apply.
- `--app <command>` opens a window (for example `nautilus`) before the report, so a
  frame shows the themed widgets; a check confirms the process runs.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json \
    --out session-theme --seed-skel --app nautilus
```

### Known AMD guest exception

Any failed system unit fails the check, with one exception — all of: only
`mcelog.service` failed, CPU `AuthenticAMD`, `/dev/mcelog` present,
`edac_mce_amd` not live, the unit loaded and failed with `exit-code` status 1, and
its journal saying "does not support this processor" and naming `edac_mce_amd`. It
is then reported as "FAIL (known, non-blocking)" with the raw value kept. If any
condition cannot be verified, the check fails.

### Journal mode

`--mode journal --disk <kept overlay>` adds no user, boots a disk kept with
`bootcheck.py --keep-overlay` again, and writes the journal of the previous boot.

## Transport limits

Instrumentation arrives as systemd credentials over SMBIOS type 11 and is written
by `systemd-tmpfiles`. Both drop data in silence, so the harness checks them:

- **A `tmpfiles.d` file argument is capped at 4095 B of resolved content.** The cap
  is on the unescaped result in bytes: 4095 B is written, 4096 B is refused with
  `File name too long` and the file is not created. `tmpfiles_arg` counts the
  content and refuses anything longer, loudly. The largest content today is 2,980 B
  of 4,095 (measured 2026-10-18, 21 defaults and two binaries); anything larger
  travels as a credential of its own.
- **A report that does not run says so on the console.** The report unit carries
  `StandardOutput=journal+console` and `StandardError=journal+console`, so a script
  that does not parse, or a missing file, appears on the serial port and the
  failure quotes it (`serial_hint`).
- **One SMBIOS type 11 credential is capped near 64 KiB**, because SMBIOS 2.1
  records the whole table length in 16 bits: 65016 B arrives, 65416 B does not.
  systemd also caps one credential at 1 MiB. The largest payload here is about
  7 KiB base64.

The first-boot report is the only payload large enough to matter, so it travels as
a credential and its unit reads it from `$CREDENTIALS_DIRECTORY`.

## First-boot check

No account and no session added. The guest reports whether the wizard is the
session that came up, whether any account could log in with a password, and whether
any service failed. It exists because `bootcheck.py` stops at the first stable
graphical screen, which can be GDM with the wizard suppressed — the exact defect
one base cleanup fixed.

```bash
python3 tests/vm/firstbootcheck.py --disk disk.raw --out firstboot-1
```

The guest waits up to five minutes for `gnome-initial-setup`, so `--boot-timeout`
must cover that. It signals `DESKOS-QA-READY` as soon as the report runs; without
it the run fails after `--ready-timeout` (420 s) saying the guest never signalled,
rather than waiting out the 900 s end marker.

Asserted: the wizard runs; a loginctl session by that name exists on a seat (a
process alone is not the session); no account with uid ≥ 1000, a usable shell and
an unlocked or empty password; and no unexpected failed service, judged by
`sessioncheck.py`'s own rule for the known guest signature, applied to this boot's
observations. Unknown is not clean. Every check boots its own overlay, so only the
rule is shared. The full failed-unit list is reported without judgement.

## CI and unit tests

`.github/workflows/vm-bootcheck.yml` runs the same build, boot, session and
first-boot checks on pull requests and on `main` for paths that can change an
image, and on demand. The QCOW2 is test input and is not uploaded; the publish
path is `supply-chain.yml`.

Unit tests: `python3 -m unittest discover -s tests/vm` (run by CI, not by
`make check`).
