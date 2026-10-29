# Lifecycle tests: A to B to A

The design is [docs/design/lifecycle.md](../../docs/design/lifecycle.md).
`.github/workflows/lifecycle.yml` runs the driver weekly and on dispatch; no pull
request runs it.

## What is here

- `driver.py` installs a disk from a source image, boots it under QEMU, stages B
  from a local registry, reboots into it, rolls back to A, and can interrupt an
  upgrade with a power loss. `--boot-failure` stages a candidate with no init,
  verifies it cannot boot, recovers the previous GRUB entry from the host and boots
  A. It writes a `/var` marker and edits a file under `/etc`, and B ships a
  different `/etc` file, so the run reports what survives each transition. Output:
  `lifecycle.json` and `summary.md` in the work directory.
- `lifecycle.py` holds the boot sequence, a parser for serial marker blocks, and a
  fail-closed evaluator: a missing block, a missing key, an unknown image, a lost
  `/var` marker, a deployment that did not stage, or a rollback that was not queued
  all fail.
- `test_lifecycle.py` and `test_driver.py` are the unit tests; `ci.yml` runs them
  with the other harness tests.

## Unit tests

```bash
python3 -m unittest discover -s tests/lifecycle
```

## Run the driver

On a Linux host with rootful podman, `skopeo`, `qemu-img`, qemu (on EL,
`/usr/libexec/qemu-kvm`), `qemu-nbd`, OVMF (`/usr/share/edk2/ovmf/`) and
`/dev/kvm`, with about 30 GB free:

```bash
python3 tests/lifecycle/driver.py \
  --image quay.io/deskos/deskos-core-centos-stream-10@sha256:<digest> \
  --workdir /var/tmp/lifecycle --keep --boot-failure
```
