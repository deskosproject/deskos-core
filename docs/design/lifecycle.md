# Lifecycle A → B → A on CentOS Stream 10

**Status:** design and harness skeleton. Nothing here has run yet, so it proves
nothing yet. `docs/roadmap.md` (Milestone 3) lists lifecycle as remaining; a
staged update and a manual rollback were validated once on a CS10 VM, not by a
test.

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
in the evidence). The disk is installed with `--target-imgref
10.0.2.2:5000/deskos/lifecycle-a:latest`, so the ref the machine tracks is the
one the test can serve. `bootc upgrade` then pulls B from the same registry.

**Guest control.** No SSH. A one-shot systemd unit is injected with a systemd
credential over SMBIOS type 11, the way `sessioncheck.py` adds its report unit:
the unit runs a script from the credential, writes JSON under
`/var/lib/deskos-lifecycle/`, prints markers to the serial console, and powers
off (`systemctl poweroff`). The harness reads the serial log, exactly as
`bootcheck.py` reads `until_serial`.

**Chaining boots.** Each boot runs with `keep_overlay=True`; the next boot uses
the kept overlay as its disk. `/var` lives in that overlay, so state persistence
is real, and each boot still starts from a copy of the previous one, never from
the original disk. Every boot gets its own output directory and summary.

**Interrupting an upgrade.** The upgrade script starts `bootc upgrade` in the
background, waits for the first progress line on the journal, then stops it
(`systemctl kill`/`SIGTERM`) and powers off. The next boot must be A, and a
fresh `bootc upgrade` must be able to stage B. What counts as "interrupted" is
recorded in the serial markers, so the report can say how far it got.

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

1. **Insecure registry over NAT.** `bootc` may refuse a plain-HTTP registry even
   with `registries.conf`; if it does, the fallback is a local ostree repo served
   over HTTP, which has no precedent here either. This is the first spike.
2. **`--target-imgref` and BIB.** The disk must track the ref the test serves,
   not the ref the image was built from. Verify on a real disk before writing the
   sequence.
3. **Where to stop the interrupt.** A kill during the layer download, a poweroff
   during the ostree write and a kill after the deployment is written are three
   different failures; the first is the one users hit. Start there.
4. **Time budget for a retry.** If `bootc upgrade` leaves a lock after the
   interrupt, the retry assertion needs a documented wait, not a sleep.
5. **SELinux and the injected unit.** `sessioncheck.py` already writes units
   under `/etc/systemd/system` from a credential; reuse its labels rather than
   inventing another path.
