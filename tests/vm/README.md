# VM harness

`bootcheck.py` boots an **unmodified** disk in QEMU with UEFI firmware, records
the screen until stably graphical, and shuts down through ACPI. Each run uses a
throwaway qcow2 overlay, so the disk is never written. `sessioncheck.py` checks
the GNOME session against the plan; `firstbootcheck.py` asks whether the
first-boot wizard is the session that came up.

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

`--no-default-kernel-args` drops `console=ttyS0`, so the disk boots with the
image's own arguments. Without a published image, `bootc install to-disk` works
too. `--target-imgref` must be the update reference, or the next update replaces
the derivative with its base.

## Running

```bash
python3 tests/vm/bootcheck.py --disk output/qcow2/disk.qcow2 --out run-1 --expect-splash --hash-disk
```

Exit `0` passes, `1` fails, `2` is a harness error. A run fails without a stable
graphical screen in `--boot-timeout`, without a splash under `--expect-splash`,
or when ACPI shutdown hangs.

`run-1/` holds `summary.md`, `summary.json`, one PNG per frame, `serial.log` and
`qemu.log`. Frames are `blank`, `text` (firmware, GRUB, console), `splash` (dark
spinner and watermark) or `graphical`. These are pixel heuristics: the harness
does not identify *which* screen it sees and does not test applications.

## Session check

The same disk through an overlay, with systemd credentials as SMBIOS type 11: a
`deskos-qa` user with GDM autologin, a script that records what GNOME reports,
and a unit that writes results and journals to the serial and powers off. Kernel
arguments and Plymouth are unchanged; this is an **instrumented boot**.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json --out session-1
```

`--offline` boots with **no NIC**, and the session must still log in with no
default route.

Expectations come from the image's `plan.json`: enabled extensions (including
Dash to Dock), favorites, login logo, wallpaper, `disable-user-extensions`
false, headless Firefox on `10.0.2.2`, `deskos report` naming the workstation,
`deskos check` running (exit 0 or 1), system Flatpaks, and no failed user or
unexpected system units. Results are in `session.md`, `session.json` and
`journal.txt`.

### Every default the plan declares

The lists come from the plan, so a later default needs no harness edit. Two
delivery paths:

- **dconf defaults** (`artifact.dconf.defaults`): each key read with
  `dconf read`.
- **GSettings vendor defaults** (`artifact.gsettingsVendorDefaults`): each
  schema and key read with `gsettings get`, the path a `*.gschema.override`
  takes, and the only one that reaches a default where the platform's dconf
  profile misses the DeskOS database.

Each default is **two claims**, read separately:

- **The image carries it.** A dconf default is also read through a profile
  naming only the image's own database; a vendor default through
  `GSETTINGS_BACKEND=memory`. A declared default the image does not carry fails.
- **The session ends up with it.** On a fresh install a differing value fails. A
  key the desktop rewrites on first login is declared in
  `EXPLAINED_SESSION_DIFFERENCES` with its reason and path.

A read the guest cannot make is **not** a contradiction: `session.json`
`plan_defaults` records it as unverified with its axis, state and reason;
`session.md` prints the counts.

Arrays compare **in order** except the keys in `UNORDERED_DEFAULTS`
(`enabled-extensions`, `custom-keybindings`), where GNOME treats the value as a
set and rewrites the list. `@as []` equals `[]` for an `as` key; `@ai []` does
not.

`platform_diagnostics` records CPU vendor, family and model, `edac_mce_amd` and
`/dev/mcelog` state, `mcelog.service` properties and up to 100 kernel
`mce`/`edac` lines; missing data is `unknown`.

### Every binary the plan installs

The plan carries each binary's destination, mode and download checksum; the
guest reads the file and the host compares.

- **Single-file artifact** (`archive: none`): the plan's checksum *is* the
  file's, so the bytes are compared and a mismatch fails.
- **Archive** (`archive: tar.gz`): the plan pins the archive, gone by session
  time. The file is verified as installed with its declared mode, and the report
  names the claim it could make.

Mode is asserted in both cases; `stat` prints `755` where the plan writes
`0755`. An unread file is reported as unread, not a mismatch. `session.json`
carries `plan_binaries`; `unreadable` names the unread axis.

### Seeding the skeleton, showing widgets

- `--seed-skel` copies `/etc/skel` into the test user's `$HOME`, as `useradd -m`
  does (the user comes from `sysusers`, which does not).
- `--app <command>` opens a window (for example `nautilus`) before the report,
  so a frame shows the themed widgets; a check confirms the process runs.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json \
    --out session-theme --seed-skel --app nautilus
```

### Known AMD guest exception

Any failed system unit fails the check, with one exception. All must hold: only
`mcelog.service` failed, CPU `AuthenticAMD`, `/dev/mcelog` present,
`edac_mce_amd` not live, the unit loaded and failed with `exit-code` 1, and its
journal says "does not support this processor" and names `edac_mce_amd`. It is
then "FAIL (known, non-blocking)"; if any condition is unverifiable, the check
fails.

### Journal mode

`--mode journal --disk <kept overlay>` boots a disk kept with
`bootcheck.py --keep-overlay` again and writes the previous boot's journal.

## Transport limits

Instrumentation arrives as systemd credentials over SMBIOS type 11, written by
`systemd-tmpfiles`. Both drop data in silence, so the harness checks them:

- **A `tmpfiles.d` file argument is capped at 4095 B of resolved content.** 4095
  B is written; 4096 B fails with `File name too long`. The largest is 2,980 B
  of 4,095 (measured 2026-10-18, 21 defaults, two binaries); anything larger
  travels as its own credential.
- **A report that does not run says so on the console.**
  `StandardOutput=journal+console` and `StandardError=journal+console` put a bad
  script or missing file on the serial port, and `serial_hint` quotes it.
- **One SMBIOS type 11 credential is capped near 64 KiB**: SMBIOS 2.1 records
  the table length in 16 bits, so 65016 B arrives and 65416 B does not. systemd
  caps a credential at 1 MiB; the largest payload, the first-boot report, is
  about 7 KiB base64 and its unit reads it from `$CREDENTIALS_DIRECTORY`.

## First-boot check

No account and no session added. The guest reports whether the wizard is the
session that came up, whether any account could log in with a password, and
whether any service failed. `bootcheck.py` stops at the first stable graphical
screen, which can be GDM with the wizard suppressed, so this check exists.

```bash
python3 tests/vm/firstbootcheck.py --disk disk.raw --out firstboot-1
```

The guest waits up to five minutes for `gnome-initial-setup`, so
`--boot-timeout` must cover that. Without `DESKOS-QA-READY` the run fails after
`--ready-timeout` (420 s), before the 900 s marker.

Asserted: the wizard runs; a loginctl session by that name exists on a seat (a
process alone is not the session); no account with uid ≥ 1000, a usable shell
and an unlocked or empty password; and no unexpected failed service, by
`sessioncheck.py`'s rule for the known guest signature.

## CI and unit tests

`.github/workflows/vm-bootcheck.yml` runs the same build, boot, session and
first-boot checks on pull requests, on `main` for image-affecting paths, and on
demand. The QCOW2 is test input, not uploaded; publishing is `supply-chain.yml`.

Unit tests: `python3 -m unittest discover -s tests/vm` (run by CI, not by
`make check`).
