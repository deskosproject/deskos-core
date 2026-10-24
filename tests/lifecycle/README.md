# Lifecycle tests: A → B → A

The design is [`docs/design/lifecycle.md`](../../docs/design/lifecycle.md). Two
registry spikes ran on 2026-10-23 and are recorded there; this directory holds
the evaluator and the driver, and the driver is **not** wired into CI yet.

## What exists

- `driver.py` — installs a disk from a source image, boots it under QEMU, stages
  B from a local registry, reboots into it, rolls back to A, and optionally
  interrupts an upgrade with a power loss. It writes a `/var` marker, edits a
  file under `/etc`, and B ships a different `/etc` file, so the run reports
  what survives each transition; it keeps each boot's journal. Writes
  `lifecycle.json` and `summary.md` in the work directory.
- `lifecycle.py` — the boot sequence, a parser for serial marker blocks and a
  fail-closed evaluator: a missing block, a missing key, an unknown image, a lost
  `/var` marker, a deployment that did not stage or a rollback that was not
  queued all fail.
- `test_lifecycle.py` and `test_driver.py` — unit tests for both; `ci.yml` runs
  them with the other harness unit tests.

## Run the unit tests

```bash
python3 -m unittest discover -s tests/lifecycle
```

## Run the driver

On a Linux host with rootful podman, `skopeo`, `qemu-img`, qemu (on EL,
`/usr/libexec/qemu-kvm`), OVMF (`/usr/share/edk2/ovmf/`) and `/dev/kvm`, with
about 30 GB free:

```bash
python3 tests/lifecycle/driver.py \
  --image quay.io/deskos/deskos-core-centos-stream-10@sha256:<digest> \
  --workdir /var/tmp/lifecycle --keep
```

It builds two test images from `--image` (A and B; B adds one marker file), pushes
them to a `registry:2` on `127.0.0.1:5000`, installs with `bootc install to-disk`
against `10.0.2.2:5000`, and drives the guest over SSH. `--interrupt` adds the
power-loss run. Nothing is published and the source image is not modified.

## What is deliberately absent

- A workflow. The driver takes about 20 minutes and needs privileged podman;
  the first runs are manual, and it becomes a gate only after it has been green
  on real builds for a while.
- Reference screenshots and the session check: this tier is about deployments,
  not about the desktop.
