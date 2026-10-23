# Lifecycle tests: A → B → A

**Status: skeleton.** This directory holds the marker format, the boot
sequence and its fail-closed evaluation. There is no QEMU driver yet, so
nothing here boots a machine and nothing here is a CI gate. The design is
[`docs/design/lifecycle.md`](../../docs/design/lifecycle.md).

## What exists

- `lifecycle.py` — the seven-boot sequence (install A, stage B, boot B,
  roll back, boot A, interrupt an upgrade, retry), a parser for the blocks a
  guest prints to the serial console, and an evaluator that fails on a missing
  block, a missing key, an unknown image, a lost `/var` marker or a deployment
  that did not stage.
- `test_lifecycle.py` — unit tests for both.

## Run the unit tests

```bash
python3 -m unittest discover -s tests/lifecycle
```

## Judge a captured serial log

The driver will produce one; until then, any serial log with the marker blocks
can be evaluated:

```bash
python3 tests/lifecycle/lifecycle.py --serial serial.log \
  --image-a "$(sha256sum a/image-info.json | cut -d' ' -f1)" \
  --image-b "$(sha256sum b/image-info.json | cut -d' ' -f1)"
```

Exit `0` means the sequence held, `1` means it did not (each failure is named),
`2` means the harness could not read the report at all. Unknown is never clean.

## Not here yet

- the QEMU driver that builds A and B, serves them from a local registry and
  chains the boots with `keep_overlay`;
- the guest-side unit and its credential;
- a workflow. The first runs must be manual and reviewed, then it can become a
  gate.
