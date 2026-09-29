# VM boot check

`bootcheck.py` boots an unmodified DeskOS disk image in QEMU with UEFI
firmware and records the screen until the machine shows a stable graphical
screen, then shuts it down through ACPI. The disk is never written; each run
boots a throwaway qcow2 overlay in its own output directory.

Requirements: Linux with `/dev/kvm`, QEMU (`qemu-system-x86_64` or
`/usr/libexec/qemu-kvm`), `qemu-img`, OVMF and Python 3. Only the Python
standard library is used. It runs as an unprivileged user with user-mode
networking.

## Building a test disk

Build the image from a rendered context, then a QCOW2 without
bootc-image-builder's default kernel arguments (`rw console=tty0
console=ttyS0`), so the disk boots with the image's own arguments, as an
installed workstation does:

    deskosctl render ./resources --workstation deskos-core-centos10 --output ctx
    sudo podman build -t localhost/deskos-core-centos10:test ctx
    sudo podman run --rm --privileged --pull=missing \
        --security-opt label=type:unconfined_t \
        -v ./output:/output -v /var/lib/containers/storage:/var/lib/containers/storage \
        quay.io/centos-bootc/bootc-image-builder@sha256:2b52843ea2bfda73b0a08d97e76b734393b1d3a804681b9fabb26723bd3a2f0b \
        build --type qcow2 --no-default-kernel-args localhost/deskos-core-centos10:test

## Running

    python3 tests/vm/bootcheck.py --disk output/qcow2/disk.qcow2 --out run-1 --expect-splash --hash-disk

Exit status: 0 pass, 1 fail, 2 harness error. The run fails when no stable
graphical screen appears within `--boot-timeout`, when `--expect-splash` is
set and no graphical boot splash is seen, or when ACPI shutdown does not
power off within `--shutdown-timeout`.

`run-1/` holds `summary.md`, `summary.json`, one PNG per distinct frame,
`serial.log` and `qemu.log`. Frames are classified as `blank`, `text`
(firmware, GRUB, console), `splash` (dark screen with only a lower-middle
spinner and watermark) or `graphical` (a mostly non-dark screen). These
are pixel heuristics: the harness does not identify which graphical
screen it sees (GNOME Initial Setup, GDM or a desktop) and does not test
applications. The frames are the evidence.

## Session check (instrumented boot)

`sessioncheck.py` boots the same unmodified disk (again through an
overlay) with systemd credentials passed as SMBIOS type 11 strings. They
add:

- a test user `deskos-qa` with GDM autologin;
- a session script that records what GNOME reports;
- a unit that writes the results and the kernel, Plymouth and GDM
  journal to the serial port, then powers the machine off.

Kernel arguments and Plymouth are unchanged, but this is an instrumented
boot and is reported as one.

    python3 tests/vm/sessioncheck.py --disk output/qcow2/disk.qcow2 --plan ctx/plan.json --out session-1

Expected values come from the image's `plan.json`: the enabled
extensions (including Dash to Dock), the favorites order and the
wallpaper. The check also requires `disable-user-extensions` to be false,
Firefox to run headless, no failed user units and no unexpected failed
system units (see the mcelog exception below). Results are
in `session.md`, `session.json` and `journal.txt`, with frames as for the
boot check.

`platform_diagnostics` in `session.json` (and its section in
`session.md`) records the guest CPU vendor, family and model,
`edac_mce_amd` and `/dev/mcelog` state, `mcelog.service` properties and at
most 100 kernel `mce`/`edac` lines; missing data is `unknown`.

Any failed system unit fails the check, with one exception: the known AMD
guest signature, where `mcelog.service` is the only failed unit, the CPU
is `AuthenticAMD`, `/dev/mcelog` is present, `edac_mce_amd` is not live,
the unit is loaded and failed with `exit-code` and status 1, and its
journal says "does not support this processor" and names
`edac_mce_amd`. It is then reported as "FAIL (known, non-blocking)" and
the raw `system_failed_units` is kept. If any condition cannot be
verified, the check fails. The image still ships and starts mcelog.

`--mode journal --disk <kept overlay>` adds no user. It boots a disk kept
with `bootcheck.py --keep-overlay` again and writes the journal of the
previous (tested) boot, which is how a failed unmodified boot is
diagnosed.

The GitHub workflow `.github/workflows/vm-bootcheck.yml` runs the same
build, boot and session checks. It is manual only (`workflow_dispatch`),
and the QCOW2 it builds is test input that is not uploaded.

Unit tests: `python3 -m unittest discover -s tests/vm` (run by CI, not by
`make check`).
