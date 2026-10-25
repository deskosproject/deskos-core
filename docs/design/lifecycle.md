# Lifecycle A → B → A on CentOS Stream 10

**Status:** design, evaluator, driver and a dispatch-only workflow
(2026-10-24; boot-failure recovery added 2026-10-25). `lifecycle.yml` runs the
driver on manual dispatch; no pull request and no schedule runs it. It first ran
end to end on 2026-10-24:
install and stage on boot 1, B and rollback on boot 2, A on boot 3,
evaluator `ok`.
Nothing installs itself yet: the spike built a disk by hand and drove it over
SSH, which the harness automates. `docs/roadmap.md` (Milestone 3) lists
lifecycle as remaining; a staged update and a manual rollback were validated
once on a CS10 VM, not by a test.

## What the spike proved (2026-10-23)

The open questions below were measured on the factory host with the published
CS10 image (`sha256:8fc6adb8…`), a `registry:2` container on the host and QEMU
user networking:

- **A guest reaches the registry over plain HTTP.** `curl http://10.0.2.2:5000/v2/`
  from the guest returned `200` with `/etc/containers/registries.conf.d/999-lifecycle.conf`
  holding `location = "10.0.2.2:5000"` and `insecure = true`. No TLS, no proxy,
  no port-forward beyond QEMU's built-in `10.0.2.2`.
- **`bootc` accepts it.** The disk was installed with
  `bootc install to-disk --target-imgref 10.0.2.2:5000/deskos/lifecycle-a:test`;
  `bootc status` reported that exact ref as `spec.image`.
- **A → B → A works on one tag.** Pushing a second image to the same tag and
  running `bootc upgrade` staged it; a reboot ran B (a marker file appeared);
  `bootc rollback` plus a reboot ran A again (the marker was gone).
- **BIB is not the tool for this.** The pinned `bootc-image-builder`
  (`2b52843`) has **no `--target-imgref`**; `bootc install to-disk` does. The
  harness installs with `bootc install`, from an OCI layout
  (`skopeo copy containers-storage:… oci:…`) because the composefs import
  refuses uncompressed layers.
- **Digests do not survive the transport.** The installed system reported
  `sha256:e099174b…` while the registry served the same content as
  `sha256:501987c4…`. The harness compares `image-info.json` hashes, never a
  digest across transports.
- **Driving the guest over SSH is enough.** A test-only image carries a
  throwaway public key for `root` and `sshd` enabled; QEMU forwards
  `127.0.0.1:2222` to the guest. This is simpler than SMBIOS credentials for a
  test that must also run `bootc` as root.
- **A power loss mid-upgrade is recoverable.** Killing QEMU with `SIGKILL`
  three seconds into `bootc upgrade` (B was 3.4 GB of new layers) left the disk
  booting A, with no staged deployment, no rollback entry and no lock: the next
  `bootc upgrade --check` still reported the update, and a retry staged B
  (`sha256:473ba68a…`). The guest-side log of the interrupted run did **not**
  survive (`/root/up.log` was gone after the reboot), so the harness takes its
  evidence from `bootc status` and the markers, never from a log in the
  deployment root.
- **On EL hosts the binary is `/usr/libexec/qemu-kvm`**, not
  `qemu-system-x86_64`, and `bootc install` needs
  `--security-opt label=type:unconfined_t` plus `--privileged --pid=host`.

## What this has to catch

The update path is image content: two timers stage the next deployment, the
staged image applies at reboot, and `bootc rollback` swaps back. Each of these
can fail without any current check noticing:

1. a staged deployment that does not apply on reboot;
2. a rollback that boots the wrong deployment, or that loses the deployment it
   was supposed to keep;
3. machine state under `/var` that does not survive either transition;
4. an upgrade interrupted mid-download that leaves a machine unable to boot or
   unable to retry;
5. `deskos check`/`report` describing the wrong image after a transition.

`bootcheck.py`, `sessioncheck.py` and `firstbootcheck.py` all boot one disk once.
They cannot see any of this.

## Shape

Two images, one installed disk, and reboots.

| Step | Boot | What happens | What is asserted |
|---|---|---|---|
| 1 | A | the installed image records itself | `image-info.json` is A's; `bootc status` has one deployment |
| 2 | A | `bootc upgrade` stages B, then reboot | a staged B exists and is not booted |
| 3 | B | plain reboot | the running deployment is B; the `/var` marker and the local `/etc` edit from step 1 survive; the `/etc` file B ships is present |
| 4 | B | `bootc rollback` (queues the previous deployment), reboot | the running deployment is A again; the `/var` marker and the local `/etc` edit survive; the `/etc` file B ships is gone |
| 5 | A | start an upgrade, interrupt it, reboot | the machine still boots A and a retry can stage B |

That is five steps across three boots: the staged update and the rollback take
effect on the next boot. The interrupted-upgrade run adds a fourth boot.

B is built from the same resources as A with one deliberate difference — the
workstation's `displayName` — so the image digest changes, `image-info.json`
changes, and no package or profile is added for the test. A test-only layer is
still a test fixture, never a resource kind: the harness does not add an escape
hatch to the compiler.

## Mechanics

**Registry.** A `registry:2` container on the runner, reachable from the guest
through QEMU's user networking at `10.0.2.2:5000`. The guest needs to treat it as
insecure, so the test image carries
`/etc/containers/registries.conf.d/999-lifecycle.conf` (a test-only file, listed
in the evidence). The disk is installed with `bootc install to-disk
--target-imgref 10.0.2.2:5000/deskos/lifecycle-a:test`, so the ref the machine
tracks is the one the test can serve. `bootc upgrade` then pulls B from the same
registry, after the host pushes B to that tag.

**Install.** `bootc install to-disk` from a compressed OCI layout, the recipe
`supply-chain.yml` uses for a platform without Anaconda:
`skopeo copy containers-storage:<image> oci:<dir>:test`, then a privileged
container of the image running `bootc install to-disk --via-loopback <disk>
--source-imgref oci:/src-oci:test --target-imgref <registry ref>
--composefs-backend --bootloader grub --filesystem ext4 --wipe
--allow-missing-verity`. BIB is not involved: its pinned version has no
`--target-imgref` and would bake the wrong ref.

**Guest control.** SSH over QEMU's `hostfwd`, with a test-only image that
carries a throwaway public key for `root` and enables `sshd`. The test image is
a fixture, never a resource: the compiler does not gain a user kind for this.
The harness runs each step as root, captures `bootc status --json`, and reads
the marker file. A systemd credential would also work, but SSH is simpler for a
test that has to run `bootc` as root and survives a reboot.

**Chaining boots.** Each boot runs the same raw disk (or a copy), so `/var`
persists for real. The driver also writes `/etc/deskos-lifecycle-local` and B
ships `/etc/deskos-lifecycle-image`. The run of 2026-10-24 measured what happens
on the rollback: the local edit survived both transitions, and the image file was
gone once A ran again. The evaluator now asserts both, so the merge behavior is
a regression, not a note. Each boot keeps its journal (`journalctl -b`) beside
the summary; the evaluator judges the record, a person reads the journal. The disk is rebuilt from scratch
for each run, never reused between runs.

**Interrupting an upgrade.** The upgrade script starts `bootc upgrade` in the
background, waits for the first progress line on the journal, then stops it
(`systemctl kill`/`SIGTERM`) and powers off. The next boot must be A, and a
fresh `bootc upgrade` must be able to stage the update. What counts as
"interrupted" is recorded in the markers, so the report can say how far it got.

The interrupted upgrade uses a **third image C**, not B. After the rollback,
B is the rollback deployment, and `bootc upgrade` to the rollback image is not
a real update: the retry staged nothing on the first CI run. C is a distinct
deployment, so the retry is meaningful.

## What this does not test

- **Automatic recovery.** Step 4 recovers with `bootc rollback`. bootc keeps
  the previous deployment but does not switch to it on its own. The driver
  recovers a non-booting candidate by setting the previous GRUB entry from the
  host; it does not test automatic recovery.
- **Application data.** `/var` and local `/etc` edits survive a rollback
  because neither is snapshotted. An upgrade that migrates or rewrites data
  under `/var` is not undone by rolling back; the operator restores that data
  from their own backup. The image ships no backup tool, and the driver's
  marker is not an application migration.

## Candidate that does not boot: accepted host-side recovery

The run covers a power loss during download; it also writes a deployment that
fails before userspace and recovers the previous entry from the host.

**Failure.** Build D from A with one deliberate, image-owned defect: remove the
init binary the kernel executes (`/usr/lib/systemd/systemd`) so the kernel
starts and then panics. Stage D, reboot, and assert the machine never reaches
SSH, with the failure in the serial log.

**Recovery.** GRUB is not driven through QMP. Both BLS entries have the same
title and the timeout is one second, so blind key injection is neither
repeatable nor safe. The accepted mechanism is **host-side bootloader
recovery**: after the failed boot, the driver attaches the disk with
`qemu-nbd`, mounts its root, sets `default=1` in `/boot/grub2/grubenv`, boots
again, and verifies A. This tests the previous boot entry as the recovery path;
it does not test a person selecting an entry in the menu.

**Evidence.** `serial-failed.log` (the failed boot), the staged and recovered
blocks in `lifecycle.json`, the recovered `bootc status`, and the same `/var`
marker as the A→B→A run. A failure to reach a booted candidate, a recovery that
boots the wrong digest, and a lost marker all fail the run.

**Not covered.** UEFI firmware menu behavior, BIOS, a rescue environment that
queues a rollback without a person, and a candidate that boots far enough to
show a login screen but fails afterwards.

## Lab result: a candidate that does not boot (2026-10-25)

On the disposable lab disk (24 GiB), A was installed and booted; D was built
from A with `/usr/lib/systemd/systemd` removed, pushed to the tracked tag and
staged with `bootc upgrade`. After the reboot:

- D did not reach SSH; the screen stayed blank and the serial log stopped after
  GRUB's "Booting CentOS Stream 10".
- The disk had two BLS entries, **both titled** "CentOS Stream 10 (Coughlan)",
  with a one-second GRUB timeout. QMP key events sent after reset did not
  reliably select the other entry.
- Setting the on-disk GRUB default to the previous entry (`grub2-editenv …
  set default=1`, with the disk mounted) and booting recovered A: SSH returned
  and `bootc status` reported A's digest, with systemd present again.

So the previous boot entry does recover the machine. The lab automated it by
editing the bootloader environment from the host, not by a person choosing an
entry. The driver now uses the same host-side mechanism: identical titles and a
one-second timeout make blind QMP key injection unsafe. Menu behavior itself is
not tested.

## Driver

`tests/lifecycle/driver.py` implements the steps above and is the artifact the
spikes were folded into: it builds A and B, pushes them to the local registry,
installs with `bootc install to-disk`, boots, drives the guest over SSH and
evaluates the record. `--interrupt` adds the power-loss run; `--boot-failure`
adds D and the host-side GRUB recovery. It needs rootful podman, qemu,
`qemu-nbd`, `skopeo` and `/dev/kvm`, takes about thirty minutes, and is
**dispatch-only**: `ci.yml` runs its unit tests, and `lifecycle.yml` runs the
driver by hand until it has been green on real builds for a while.

## Markers and evaluation

The guest prints one block per boot:

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

The harness parses blocks, compares the image hash with the expected A or B
hash, and fails closed: a missing block, a missing key, an image hash that
matches neither, or a transition that did not happen are all failures. There is
no "unknown is clean".

## Evidence

Per run: one directory per boot (serial log, markers, frames, `summary.json`,
`journal-boot-N-<step>.log`), the A and B `plan.json` and `image-info.json`, the
registry log, the installer log, and a top-level `lifecycle.json` naming the A
digest, the B digest, each boot's observed image, the `/var` marker, and the
local and image `/etc` files across boots. That file is what a
person reads to see the timeline, and what a future gate would compare.

## Cost and placement

Two image builds, one disk, five boots: roughly 45–60 minutes on a runner that
already builds one image in 12–15 minutes. It must not join the pull-request
workflow. The first step is a manual `workflow_dispatch` job (like
`vm-bootcheck.yml` but separate), with the results reviewed by a person; it
becomes a gate only after it has been green on real builds for a while.

## Open questions

1. **Insecure registry over NAT — answered (2026-10-23).** It works: `200` from
   the guest and `bootc upgrade` pulled over plain HTTP with
   `insecure = true` in `registries.conf.d`. No ostree repo needed.
2. **`--target-imgref` — answered (2026-10-23).** BIB's pinned version has no
   such flag; `bootc install to-disk` does and records the ref the test serves.
   The install source is a compressed OCI layout, and its digest differs from
   the registry's for the same content, so compare content hashes.
3. **Where to stop the interrupt — answered for the first window
   (2026-10-23).** A kill during the layer download is recovered: nothing
   staged, clean boot, retry works. The ostree-write window and a kill after the
   deployment is written are still untested; they need the timing to be made
   observable, not guessed.
4. **Time budget for a retry — no lock observed (2026-10-23).** The retry
   immediately after the interrupted boot staged B without a wait. If a future
   Anaconda of bootc introduces one, the assertion must read the reason, not
   sleep.
5. **SELinux and the bootc install.** Answered for the host side:
   `--security-opt label=type:unconfined_t`, `--privileged --pid=host`, and
   `/dev` mounted. The guest side is plain bootc, so no custom unit labels.
