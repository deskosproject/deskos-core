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

What it asserts, and what it only reports:

- the wizard (`gnome-initial-setup`) is running;
- no account with uid >= 1000, a usable shell, and a password that is neither locked
  (`!` or `*`) nor empty;
- no failed **service** units.

The full failed-unit list is reported even where it is not asserted, so a run cannot
hide what it did not judge. On the Ubuntu image the sssd responder sockets report
`dependency` because `sssd.service` is skipped by its own condition (sssd is
installed unconfigured); those are sockets, not services. Like the session check,
this is an instrumented boot, and is reported as one.

## CI and unit tests

The GitHub workflow `.github/workflows/vm-bootcheck.yml` runs the same
build, boot and session checks. It is manual only (`workflow_dispatch`),
and the QCOW2 it builds is test input that is not uploaded. It is a pure
check: the publish path is `supply-chain.yml`.

Unit tests: `python3 -m unittest discover -s tests/vm` (run by CI, not by
`make check`).
