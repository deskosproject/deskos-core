# VM boot check

`bootcheck.py` boots an **unmodified** DeskOS disk image in QEMU with UEFI
firmware and records the screen until the machine shows a stable graphical
screen, then shuts it down through ACPI. **The disk is never written**;
each run boots a throwaway qcow2 overlay in its own output directory.
`sessioncheck.py` adds an instrumented boot that checks the GNOME session
against the plan.

**Requirements:** Linux with `/dev/kvm`, QEMU (`qemu-system-x86_64` or
`/usr/libexec/qemu-kvm`), `qemu-img`, OVMF and Python 3. Only the Python
standard library is used. It runs as an unprivileged user with user-mode
networking.

## Building a test disk

Build the image from a rendered context, then a QCOW2 *without*
bootc-image-builder's default kernel arguments
(`rw console=tty0 console=ttyS0`), so the disk boots with the image's own
arguments, as an installed workstation does:

```bash
deskosctl render --workstation deskos-core-centos10 --output ctx
sudo podman build -t localhost/deskos-core-centos10:test ctx
sudo podman run --rm --privileged --pull=missing \
    --security-opt label=type:unconfined_t \
    -v ./output:/output -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
    build --type qcow2 --no-default-kernel-args localhost/deskos-core-centos10:test
```

**Or install to a raw disk**, which is the path for a platform whose image is not
published — Ubuntu 26.04 today. The harness detects the backing format, so a raw
disk and a QCOW2 are both accepted:

```bash
deskosctl render --workstation <workstation> --output ctx
sudo podman build -t localhost/<workstation>:test ctx
# The composefs import rejects uncompressed layers, so recompress into an OCI
# layout, which is also what the install mounts as its source:
sudo skopeo copy containers-storage:localhost/<workstation>:test oci:/tmp/oci:test
truncate -s 32G disk.raw
# --target-imgref is the reference the installed system will update from. It must
# be the reference of the image being installed (this derivative's own published
# reference once it has one); leaving it out records the lab path, and pointing it
# at the base would replace the derivative with the base at the next update.
sudo podman run --rm --privileged --pid=host --security-opt label=type:unconfined_t \
    -v /dev:/dev -v /var/lib/containers:/var/lib/containers \
    -v /tmp/oci:/src-oci:ro -v "$PWD":/output \
    localhost/<workstation>:test bootc install to-disk --via-loopback /output/disk.raw \
    --source-imgref oci:/src-oci:test --target-imgref <published reference> \
    --composefs-backend --bootloader systemd --wipe --filesystem ext4 --allow-missing-verity
```

## Running

```bash
python3 tests/vm/bootcheck.py --disk output/qcow2/disk.qcow2 --out run-1 --expect-splash --hash-disk
```

| Exit status | Meaning |
|---|---|
| `0` | pass |
| `1` | fail |
| `2` | harness error |

The run fails when:

- no stable graphical screen appears within `--boot-timeout`;
- `--expect-splash` is set and no graphical boot splash is seen;
- ACPI shutdown does not power off within `--shutdown-timeout`.

`run-1/` holds `summary.md`, `summary.json`, one PNG per distinct frame,
`serial.log` and `qemu.log`. Frames are classified as:

- `blank`;
- `text` (firmware, GRUB, console);
- `splash` (dark screen with only a lower-middle spinner and watermark);
- `graphical` (a mostly non-dark screen).

> [!NOTE]
> These are **pixel heuristics**: the harness does not identify which
> graphical screen it sees (GNOME Initial Setup, GDM or a desktop) and
> does not test applications. *The frames are the evidence.*

## Session check (instrumented boot)

`sessioncheck.py` boots the same unmodified disk (again through an
overlay) with systemd credentials passed as SMBIOS type 11 strings. They
add:

- a test user `deskos-qa` with GDM autologin;
- a session script that records what GNOME reports;
- a unit that writes the results and the kernel, Plymouth and GDM
  journal to the serial port, then powers the machine off.

Kernel arguments and Plymouth are unchanged, but this is an
**instrumented boot** and is reported as one.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json --out session-1
```

Expected values come from the image's `plan.json`: the enabled
extensions (including Dash to Dock), the favorites order and the
wallpaper. The check also requires `disable-user-extensions` to be false,
Firefox to run headless, no failed user units and no unexpected failed
system units (see [the mcelog exception](#known-amd-guest-exception-mcelog)
below). Results are in `session.md`, `session.json` and `journal.txt`,
with frames as for the boot check.

### Every default the plan declares

Beyond those named checks, the harness asserts **every default the plan
declares** and takes the lists from the plan itself, so a default declared
later is asserted without editing `sessioncheck.py`. There are two declared
lists and two delivery paths:

- **dconf defaults** (`artifact.dconf.defaults`): the guest reads each key
  with `dconf read`, the effective value the session sees, in the same
  GVariant text the plan stores.
- **GSettings vendor defaults** (`artifact.gsettingsVendorDefaults`): the
  guest reads each schema and key with `gsettings get`, in session and in
  the compiled schema default. This is the path a
  `*.gschema.override` takes, and it is the only one that delivers a
  default on a platform whose dconf profile does not read the DeskOS
  database.

Each declared default is **two claims**, not one, and each claim is read
separately:

- **The image carries it.** For a dconf default the guest also reads the key
  through a profile naming only the image's own dconf database, so the
  value the image ships is known apart from the value the session ends up
  with. For a vendor default the guest reads the compiled schema default
  with `GSETTINGS_BACKEND=memory`. A declared default the image does not
  carry **fails**, whether or not the session happens to see the plan's
  value: the session showing the right value does not prove DeskOS put it
  there, and the value could come from anywhere.
- **The session ends up with it.** The value the session sees must match the
  plan. The run is a fresh account on a fresh install, so nothing legitimate
  has set a value yet: a session value that differs **fails**, because a
  declared default that does not reach the user is the defect this check
  exists for. A key the desktop itself rewrites on first login is the only
  exception, and it is declared in `EXPLAINED_SESSION_DIFFERENCES` with the
  reason the report prints — not skipped by default.

A read the guest cannot make is not a contradiction, so it is not failed: if
the image carries the default but the session cannot see it (its dconf
profile does not reach the database that holds it, or the read fails), the
key is **reported as unverified with the reason**. That is the case that
tells whether a platform's profile reads the database DeskOS writes to.
`session.json` carries `plan_defaults` (`declared`, `in_image`,
`in_session`, and every read that was not verified with its axis, state and
reason), and `session.md` prints the counts and each unverified read.

Arrays are compared **in order**, because the plan fixes that order:
`favorite-apps` has its own order-sensitive check for exactly this reason. An
array is compared as a multiset only for the keys in `UNORDERED_DEFAULTS`,
where GNOME itself treats the value as a set and rewrites the whole list
(`enabled-extensions`, `custom-keybindings`); a reorder there is the same
value, and the detail says so. Adding a key to that set is a claim about
GNOME's semantics, so it carries a reason.

The type annotation dconf prints for an empty array is dropped only when it
is the declared type's own: `@as []` is the `[]` an `as` key stores, while
`@ai []` is a different value and does not match.

The two lists travel to the guest as files of their own, so the script that
reads them stays inside the tmpfiles.d argument cap as plans grow
(`tmpfiles_arg` refuses content the guest would drop in silence).

`platform_diagnostics` in `session.json` (and its section in
`session.md`) records the guest CPU vendor, family and model,
`edac_mce_amd` and `/dev/mcelog` state, `mcelog.service` properties and at
most 100 kernel `mce`/`edac` lines; missing data is `unknown`.

### Seeding the skeleton and showing the widgets

Two optional flags reach the per-user state a desktop theme lives in:

- `--seed-skel` copies `/etc/skel` into the test user's `$HOME`, as
  `useradd -m` does (the test user comes from `sysusers`, which does not).
  Without it the image's per-user seeds never apply to the session. With it,
  every generated file the plan places under `/etc/skel` is asserted to reach
  `$HOME` with the generated content.
- `--app <command>` opens a window (for example `nautilus` or `ptyxis`)
  before the report, so a captured frame shows the themed widgets. A check
  confirms the process is running.

```bash
python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json \
    --out session-theme --seed-skel --app nautilus
```

### Known AMD guest exception (mcelog)

**Any failed system unit fails the check**, with one exception: the known
AMD guest signature, where all of these hold:

- `mcelog.service` is the only failed unit;
- the CPU is `AuthenticAMD`;
- `/dev/mcelog` is present;
- `edac_mce_amd` is not live;
- the unit is loaded and failed with `exit-code` and status 1;
- its journal says "does not support this processor" and names
  `edac_mce_amd`.

It is then reported as "FAIL (known, non-blocking)" and the raw
`system_failed_units` is kept. **If any condition cannot be verified, the
check fails.** The image still ships and starts mcelog.

### Journal mode

`--mode journal --disk <kept overlay>` adds no user. It boots a disk kept
with `bootcheck.py --keep-overlay` again and writes the journal of the
previous (tested) boot, which is how a failed unmodified boot is
diagnosed.

## Transport limits

The instrumentation reaches the guest as systemd credentials over SMBIOS type 11
(`-smbios type=11,value=io.systemd.credential.binary:<name>=<base64>`), and
`systemd-tmpfiles` turns a `tmpfiles.extra` line into a file. Both have a limit,
and both drop data in silence, which is why the harness checks them instead of
waiting for a report that never comes:

- **A `tmpfiles.d` file argument is capped at 4095 B of resolved content.**
  systemd unescapes the argument and then expands specifiers into a `PATH_MAX-1`
  buffer (`cunescape()` then `specifier_printf(unescaped, PATH_MAX-1, …)`), so
  the cap is on the result, in bytes, not on the bytes in the file: measured
  against `systemd-tmpfiles`, 4095 B is written and 4096 B is refused with `File
  name too long` and **the file is not created**, 4095 B of `%` (8190 B once
  escaped) is written just the same, and 2047 `é` (4094 B) pass while 2048 (4096
  B) do not. `sessioncheck.tmpfiles_arg` counts the unescaped content in bytes
  and refuses anything longer, loudly, instead of letting the guest drop it;
  anything larger travels as a credential of its own.
- **A single SMBIOS type 11 credential is capped near 64 KiB.** The SMBIOS 2.1
  entry point records the length of the *whole* table in a 16-bit field
  (`SMBIOS_21_MAX_TABLES_LEN = 0xffff` in QEMU's `hw/smbios/smbios.c`), so every
  OEM string together — not one structure — has to fit in 64 KiB. Measured with
  QEMU: a string of 65016 B arrives intact and one of 65416 B does not. This is
  far above what the harness passes — the largest payload is the first-boot
  report, about 7 KiB base64. systemd also caps one credential at 1 MiB
  (`CREDENTIAL_SIZE_MAX`).

The first-boot report script is the only payload large enough to matter, so it
travels as a credential of its own and its unit reads it from
`$CREDENTIALS_DIRECTORY`; it never becomes a `tmpfiles.d` argument and needs no
escaping.

## First-boot check (instrumented boot, no account added)

`firstbootcheck.py` answers what a screenshot cannot. It boots the same unmodified
disk through an overlay, adds **no account and no session**, and has the guest
report whether the first-boot wizard is the session that came up, whether any
account could log in with a password, and whether any service unit failed.

It exists because `bootcheck.py` classifies pixels and concludes at the first stable
graphical screen — on Ubuntu 26.04 that screen is GDM, not the wizard — so it reports
the same PASS with the wizard suppressed, and suppressing it is exactly the defect
the base cleanup fixed.

```bash
python3 tests/vm/firstbootcheck.py --disk disk.raw --out firstboot-1
```

The guest waits up to five minutes for `gnome-initial-setup` before reporting, so
`--boot-timeout` has to cover that. Results are in `firstboot.md` and
`firstboot.json`, with the raw guest values and the frames.

The guest signals `DESKOS-QA-READY` as soon as the report runs, before the wait
for the wizard. If it never arrives, the run fails after `--ready-timeout`
(default 420 s) saying the guest never signalled it was instrumented — a
credential may have been dropped — instead of waiting out `--boot-timeout`
(900 s) for an end marker that will not come. The report itself is a credential
of its own, read from `$CREDENTIALS_DIRECTORY` (see
[Transport limits](#transport-limits)).

What it asserts:

- the wizard (`gnome-initial-setup`) is running;
- the wizard **is the session that came up**: the guest reports loginctl sessions, and
  a session by that name must exist on a seat. A process alone is not the session, and
  a stable GDM screen with the wizard suppressed is exactly what this has to tell
  apart from the wizard;
- no account with uid >= 1000, a usable shell, and a password that is neither locked
  (`!` or `*`) nor empty;
- no unexpected failed system services, judged with **`sessioncheck.py`'s own rule for
  the known guest signature** — the same function, not a second copy of it — applied
  here to this boot's observations. The probes it needs (CPU vendor, `edac_mce_amd`,
  `/dev/mcelog`, the unit's properties, its journal) are collected here too.

A guest that does not report the failed services fails: unknown is not clean. Every
check boots its own overlay, so what is shared is the rule, not another check's
observations — a unit can fail in this boot and not in the session check's.

What it reports in full without judging it:

- the complete failed-unit list, including unit types the check above does not judge.
  On the Ubuntu image the sssd responder sockets report `dependency` because
  `sssd.service` is skipped by its own condition (sssd is installed unconfigured);
  those are sockets, not services. Like the session check, this is an instrumented
  boot, and is reported as one.

## CI and unit tests

The GitHub workflow `.github/workflows/vm-bootcheck.yml` runs the same
build, boot and session checks. It runs on pull requests and on `main` for the paths
that can change an image, and on demand (`workflow_dispatch`), per platform: CentOS
Stream 10 from this checkout, and Ubuntu 26.04 with no published image to hand to
`bootc-image-builder`, written by `bootc install to-disk` out of a compressed OCI
layout. The QCOW2 and the raw disk it builds are test input that is not uploaded. It
is a pure check: the publish path is `supply-chain.yml`.

Unit tests: `python3 -m unittest discover -s tests/vm` (run by CI, not by
`make check`).
