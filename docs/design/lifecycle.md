# Lifecycle A → B → A on CentOS Stream 10

**Status:** design, evaluator, driver and a weekly workflow on `main`
(2026-10-24; boot-failure recovery 2026-10-25; full workflow green 2026-10-26).
Run `40305080925` (2026-10-26, first **scheduled**) took the full path in 7
steps; no pull request runs it. A staged update and a manual rollback were
validated once on a CS10 VM; the weekly workflow repeats them. Run
`42710320711` (2026-11-09) added the post-deployment power-loss window and
passed A → B → A, both interrupt windows and the failed-candidate recovery.

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
5. the booted deployment identity after a transition, read from `bootc status`.

## Shape

| Step | Boot | What happens | What is asserted |
|---|---|---|---|
| 1 | A | the installed image records itself | `image-info.json` is A's; `bootc status` has one deployment |
| 2 | A | `bootc upgrade` stages B, then reboot | a staged B exists and is not booted |
| 3 | B | plain reboot | the running deployment is B; the `/var` marker and the local `/etc` edit from step 1 survive; the `/etc` file B ships is present |
| 4 | B | `bootc rollback` (queues the previous deployment), reboot | the running deployment is A again; the `/var` marker and the local `/etc` edit survive; the `/etc` file B ships is gone |
| 5 | A | start an upgrade, interrupt it, reboot | the machine still boots A and a retry can stage C |
| 6 | A | start a real reboot, wait for the finalize service marker, kill QEMU, boot | the next boot runs C; the `/var` marker and local `/etc` edit survive |

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

**Interrupting an upgrade.** The boot that runs `boot-a` starts with the disk
capped at 100 IOPS, which widens the deployment write from milliseconds to tens
of seconds. A guest watcher prints `LIFECYCLE-WRITE` on the serial port when new
deployment state appears (`/sysroot/state/deploy` on the composefs backend, or
`/ostree/deploy/*/deploy`), and the host kills QEMU as soon as it reads the
marker. The report records `interrupt_write_window`; a run that never sees the
marker fails instead of silently testing the download. The next boot must be A
and a fresh upgrade must stage. The retry uses a **third image C**, because
upgrading to B after the rollback stages nothing.

## What this does not test

- **Automatic recovery.** Step 4 recovers with `bootc rollback`; bootc keeps the
  previous deployment but does not switch to it on its own. The driver recovers
  a non-booting candidate by setting the previous GRUB entry from the host.
- **Torn writes.** The post-deployment marker runs `sync` before it signals, so
  the test models a guest-RAM loss after the finalize has reached the disk. It
  does not prove that a write torn before the flush is recoverable.
- **Application data.** `/var` and local `/etc` edits survive a rollback because
  neither is snapshotted. An upgrade that rewrites `/var` is not undone by
  rolling back; the operator restores from their own backup. Flatpak
  applications live under `/var/lib/flatpak`, so an image rollback neither
  removes nor downgrades them. This run records `/var`, not a Flatpak install;
  the Flatpak path has its own checks.

## Candidate that does not boot: accepted host-side recovery

**Failure.** Remove `/usr/lib/systemd/systemd` in D so the kernel panics. Stage
D and reboot; the machine must never reach SSH.

**Recovery.** The BLS entries share one title and the timeout is one second, so
blind key injection through QMP is unsafe. The driver attaches the disk with
`qemu-nbd`, finds the installed entry by its composefs verity in the BLS options,
sets that index in `/boot/grub2/grubenv`, boots again, and verifies the
deployment that was running before the candidate.

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
`checks.yml` runs its unit tests, and `lifecycle-test.yml` runs it weekly and by hand.

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
logs, the finalize serial log, and a top-level `lifecycle.json` naming both
digests, each boot's observed image, the `/var` and `/etc` state, and whether
each interrupt window was reached.

## Cost and placement

Three test images and five boots for the update path; the optional failed
candidate adds a fourth image and two more boots. Roughly 45-60 minutes.

## Open questions

1. **Insecure registry over NAT: answered (2026-10-23).** Plain HTTP works; no
   ostree repo is needed.
2. **`--target-imgref`: answered (2026-10-23).** Only `bootc install to-disk`
   has it; BIB's pinned build does not.
3. **Where to stop the interrupt: two windows tested.** A kill during the
   layer download is recovered (2026-10-23). The deployment-write window is
   tested since 2026-11-09 (lifecycle-test run `42588368979`): the guest
   recovered A, kept the `/var` marker and staged C on the retry. The
   post-deployment window starts a real reboot, waits for the finalize service
   marker, kills QEMU after the staged entry is active and asserts that the next
   boot runs C with the `/var` marker and local `/etc` edit.
4. **Time budget for a retry: no lock observed (2026-10-23).** If a future bootc
   introduces one, the assertion must read the reason, not sleep.
5. **SELinux and the bootc install: answered for the host side (2026-10-23).**
   The guest side is plain bootc.
