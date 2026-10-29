# Lifecycle A → B → A on CentOS Stream 10

**Status:** design, evaluator, driver and a weekly workflow on `main`
(2026-10-24; boot-failure recovery 2026-10-25; full workflow green 2026-10-26).
Run `40305080925` (2026-10-26, first **scheduled**) took the full path in 7
steps; no pull request runs it. A staged update and a manual rollback were
validated once on a CS10 VM; the weekly workflow repeats them.

## What the spike proved (2026-10-23)

Factory host, published CS10 image (`sha256:8fc6adb8…`), `registry:2` on the
host, QEMU user networking:

- `curl http://10.0.2.2:5000/v2/` returned `200`; the test image's
  `/etc/containers/registries.conf.d/999-lifecycle.conf` sets
  `location = "10.0.2.2:5000"`, `insecure = true`.
- `bootc install to-disk --target-imgref 10.0.2.2:5000/deskos/lifecycle-a:test`
  tracked that ref, and `bootc upgrade`/`bootc rollback` moved A → B → A on one
  tag. The pinned `bootc-image-builder` (`2b52843`) has no `--target-imgref`.
- Digests differ across transports (`sha256:e099174b…` installed,
  `sha256:501987c4…` in the registry); compare `image-info.json` hashes.
- `SIGKILL` three seconds into `bootc upgrade` recovered with nothing staged; a
  retry staged B (`sha256:473ba68a…`).

## What this has to catch

The update path is image content: timers stage the next deployment, it applies at
reboot, and `bootc rollback` swaps back.

1. a staged deployment that does not apply on reboot;
2. a rollback that boots the wrong deployment, or loses the one to keep;
3. machine state under `/var` that does not survive either transition;
4. an interrupted upgrade that leaves a machine unable to boot or retry;
5. `deskos check`/`report` describing the wrong image after a transition.

## Shape

| Step | Boot | What happens | What is asserted |
|---|---|---|---|
| 1 | A | the installed image records itself | `image-info.json` is A's; `bootc status` has one deployment |
| 2 | A | `bootc upgrade` stages B, then reboot | a staged B exists and is not booted |
| 3 | B | plain reboot | the running deployment is B; the `/var` marker and the local `/etc` edit from step 1 survive; the `/etc` file B ships is present |
| 4 | B | `bootc rollback` (queues the previous deployment), reboot | the running deployment is A again; the `/var` marker and the local `/etc` edit survive; the `/etc` file B ships is gone |
| 5 | A | start an upgrade, interrupt it, reboot | the machine still boots A and a retry can stage B |

B differs from A in one resource value, the workstation's `displayName`, so the
digest changes; a test-only layer is a fixture, never a resource kind.

## Mechanics

**Install.** `skopeo copy containers-storage:<image> oci:<dir>:test`, then a
privileged container running `bootc install to-disk --via-loopback <disk>
--source-imgref oci:/src-oci:test --target-imgref <registry ref>
--composefs-backend --bootloader grub --filesystem ext4 --wipe
--allow-missing-verity`; `bootc upgrade` pulls B after the host pushes the tag.

**Guest control and boots.** SSH over `hostfwd` records `bootc status --json`
and markers. Each boot runs the same raw disk, so `/var` persists. The driver
writes `/etc/deskos-lifecycle-local` and B ships
`/etc/deskos-lifecycle-image`; the evaluator asserts both.

**Interrupting an upgrade.** `bootc upgrade` is stopped
(`systemctl kill`/`SIGTERM`) after the first journal progress line and the machine
powers off; the next boot must be A and a fresh upgrade must stage. The retry uses
a **third image C**, because upgrading to B after the rollback stages nothing.

## What this does not test

- **Automatic recovery.** Step 4 recovers with `bootc rollback`; bootc keeps the
  previous deployment but does not switch to it on its own. The driver recovers
  a non-booting candidate by setting the previous GRUB entry from the host.
- **Application data.** `/var` and local `/etc` edits survive a rollback because
  neither is snapshotted. An upgrade that rewrites `/var` is not undone by
  rolling back; the operator restores from their own backup.

## Candidate that does not boot: accepted host-side recovery

**Failure.** Remove `/usr/lib/systemd/systemd` in D so the kernel panics. Stage
D and reboot; the machine must never reach SSH.

**Recovery.** The BLS entries share one title and the timeout is one second, so
blind key injection through QMP is unsafe. The driver attaches the disk with
`qemu-nbd`, finds the installed entry by its composefs verity in the BLS options,
sets that index in `/boot/grub2/grubenv`, boots again, and verifies A.

**Evidence.** `serial-failed.log`, the staged and recovered blocks in
`lifecycle.json`, the recovered `bootc status`, and the `/var` marker; a
candidate that never boots, a wrong-digest recovery and a lost marker fail.

**Not covered.** UEFI firmware menu behaviour, BIOS, an unattended rescue
rollback, and a login-screen failure after boot.

## Lab result: a candidate that does not boot (2026-10-25)

On the 24 GiB lab disk, setting the on-disk GRUB default to the previous entry
(`grub2-editenv … set default=1`) recovered A with the `/var` marker intact.

## Driver

`tests/lifecycle/driver.py` builds A and B, pushes them, installs with `bootc
install to-disk` and boots. `--interrupt` adds the power-loss run;
`--boot-failure` adds D and the host-side GRUB recovery. It needs rootful podman,
qemu, `qemu-nbd`, `skopeo` and `/dev/kvm`, and is **not a pull-request check**:
`ci.yml` runs its unit tests, and `lifecycle.yml` runs it weekly and by hand.

## Markers and evaluation

```
DESKOS-LIFECYCLE-BEGIN
boot=2
step=stage
image=<sha256 of /usr/share/deskos/image-info.json>
deployments=<bootc status --json, one line>
var_marker=<contents of /var/lib/deskos-lifecycle/marker>
staged=<yes|no>
etc=<contents of /etc/deskos-lifecycle-local>
image_etc=<yes|no>
DESKOS-LIFECYCLE-END
```

The harness fails closed on a missing block, key or transition.

## Evidence

Per run: one directory per boot (serial log, markers, frames, `summary.json`,
`journal-boot-N-<step>.log`), the A and B `plan.json` and `image-info.json`, the
logs, and a top-level `lifecycle.json` naming both digests, each boot's observed
image, and the `/var` and `/etc` state.

## Cost and placement

Two image builds, one disk, five boots: roughly 45-60 minutes.

## Open questions

1. **Insecure registry over NAT: answered (2026-10-23).** Plain HTTP works; no
   ostree repo is needed.
2. **`--target-imgref`: answered (2026-10-23).** Only `bootc install to-disk`
   has it; BIB's pinned build does not.
3. **Where to stop the interrupt: answered for the first window (2026-10-23).**
   A kill during the layer download is recovered; the ostree-write and
   post-deployment windows are untested.
4. **Time budget for a retry: no lock observed (2026-10-23).** If a future bootc
   introduces one, the assertion must read the reason, not sleep.
5. **SELinux and the bootc install: answered for the host side (2026-10-23).**
   The guest side is plain bootc.
