# Lifecycle A → B → A on CentOS Stream 10

**Status:** design, harness skeleton and a completed registry spike (2026-10-23).
Nothing installs itself yet: the spike built a disk by hand and drove it over
SSH, which is what the harness will automate. `docs/roadmap.md` (Milestone 3)
lists lifecycle as remaining; a staged update and a manual rollback were
validated once on a CS10 VM, not by a test.

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
| 2 | A | `bootc upgrade` stages B, then power off | a staged B exists and is not booted |
| 3 | B | plain reboot | the running deployment is B; the `/var` marker written in step 1 survives |
| 4 | B | `bootc rollback`, reboot | the running deployment is A again; the `/var` marker survives |
| 5 | A | start an upgrade, interrupt it, reboot | the machine still boots A and a retry can stage B |

That is five steps across seven boots: staging and rollback each need a reboot
to become visible.

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
persists for real. Every boot gets its own output directory, serial log and
summary; the disk is rebuilt from scratch for each run, never reused between
runs.

**Interrupting an upgrade.** The upgrade script starts `bootc upgrade` in the
background, waits for the first progress line on the journal, then stops it
(`systemctl kill`/`SIGTERM`) and powers off. The next boot must be A, and a
fresh `bootc upgrade` must be able to stage B. What counts as "interrupted" is
recorded in the markers, so the report can say how far it got.

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
DESKOS-LIFECYCLE-END
```

The harness parses blocks, compares the image hash with the expected A or B
hash, and fails closed: a missing block, a missing key, an image hash that
matches neither, or a transition that did not happen are all failures. There is
no "unknown is clean".

## Evidence

Per run: one directory per boot (serial log, markers, frames, `summary.json`),
the A and B `plan.json` and `image-info.json`, the registry log, the installer
log, and a top-level `lifecycle.json` naming the A digest, the B digest, each
boot's observed image, and the `/var` marker across boots. That file is what a
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
3. **Where to stop the interrupt.** Unanswered. A kill during the layer
   download, a poweroff during the ostree write and a kill after the deployment
   is written are three different failures; the first is the one users hit.
   Start there.
4. **Time budget for a retry.** Unanswered. If `bootc upgrade` leaves a lock
   after the interrupt, the retry assertion needs a documented wait, not a
   sleep.
5. **SELinux and the bootc install.** Answered for the host side:
   `--security-opt label=type:unconfined_t`, `--privileged --pid=host`, and
   `/dev` mounted. The guest side is plain bootc, so no custom unit labels.
