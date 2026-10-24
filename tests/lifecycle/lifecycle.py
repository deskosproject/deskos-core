#!/usr/bin/env python3
"""Lifecycle A -> B -> A on an installed DeskOS disk: markers and evaluation.

This is the harness skeleton described in docs/design/lifecycle.md. It does
not drive QEMU yet: it defines the boot sequence, parses the blocks a guest
prints to the serial console, and judges them fail-closed. The driver that
boots each step is the next change, so nothing here is a gate.

Markers, one block per boot:

    DESKOS-LIFECYCLE-BEGIN
    boot=3
    step=boot-b
    image=sha256:...
    staged=no
    marker=A1
    DESKOS-LIFECYCLE-END
"""
from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass

BEGIN = "DESKOS-LIFECYCLE-BEGIN"
END = "DESKOS-LIFECYCLE-END"
REQUIRED = ("boot", "step", "image", "staged", "marker")


@dataclass(frozen=True)
class Boot:
    """One boot of the sequence: what the guest does and which image it runs."""

    number: int
    step: str
    image: str  # "a" or "b"
    # The /var marker this boot must observe, or None before one is written.
    expect_marker: str | None
    # The marker this boot writes, if it writes one.
    writes: str | None = None
    # The staged deployment bootc status must report, or None when unchecked.
    staged: str | None = None
    # The queued rollback bootc status must report, or None when unchecked.
    rollback: str | None = None
    # The locally edited /etc file this boot must see, or None when unchecked.
    etc: str | None = None
    # Whether the file B ships under /etc must be present ("yes"/"no"), or None.
    image_etc: str | None = None


# Five steps, seven boots: staging and rollback each need a reboot to show.
# A boot number repeats when one boot runs two steps: staging and the
# rollback command both run on the deployment they are about to leave, and
# take effect on the next boot. Three boots for A -> B -> A, four with the
# interrupted upgrade.
SEQUENCE = (
    Boot(1, "install", "a", None, writes="A1", staged="no", rollback="no",
         image_etc="no"),
    Boot(1, "stage", "a", "A1", staged="yes", rollback="no", etc="local",
         image_etc="no"),
    Boot(2, "boot-b", "b", "A1", staged="no", rollback="no", etc="local",
         image_etc="yes"),
    # `bootc rollback` queues the previous deployment; it does not stage a
    # download, so the assertion is on rollbackQueued, not on staged.
    Boot(2, "rollback", "b", "A1", staged="no", rollback="yes", etc="local",
         image_etc="yes"),
    # On the rollback deployment, a file B added under /etc is recorded but not
    # asserted: bootc merges /etc rather than versioning it per deployment, and
    # the run reports what actually happened.
    Boot(3, "boot-a", "a", "A1", writes="A2", staged="no", rollback="no",
         etc="local"),
    Boot(4, "interrupt", "a", "A2", staged="no", rollback="no", etc="local"),
    Boot(4, "retry", "a", "A2", staged="yes", rollback="no", etc="local"),
)


def parse_blocks(text):
    """Return the marker blocks in one serial log, in order.

    A block is only a block when it is complete: a BEGIN with no END is an
    error, not an empty result, because a guest that died mid-report is exactly
    the failure this has to report.
    """
    blocks = []
    current = None
    for raw in text.splitlines():
        line = raw.strip()
        if line == BEGIN:
            if current is not None:
                raise ValueError("nested BEGIN before END")
            current = {}
            continue
        if line == END:
            if current is None:
                raise ValueError("END without BEGIN")
            blocks.append(current)
            current = None
            continue
        if current is None or not line:
            continue
        key, sep, value = line.partition("=")
        if not sep or not key:
            raise ValueError(f"marker line without key=value: {line!r}")
        if key in current:
            raise ValueError(f"duplicate marker key {key!r}")
        current[key] = value
    if current is not None:
        raise ValueError("BEGIN without END: the guest did not finish its report")
    return blocks


def evaluate(blocks, seq=SEQUENCE, images=None):
    """Return the failures of one run; empty means the sequence held.

    ``images`` maps "a"/"b" to the sha256 of that image's image-info.json. A
    hash that matches neither is a failure: the machine is running something
    this run did not build.
    """
    images = images or {}
    failures = []
    if len(blocks) != len(seq):
        failures.append(f"{len(blocks)} boots reported, {len(seq)} expected")

    for index, boot in enumerate(seq):
        where = f"boot {boot.number} ({boot.step})"
        if index >= len(blocks):
            failures.append(f"{where}: no report")
            continue
        block = blocks[index]
        missing = [k for k in REQUIRED if k not in block]
        if missing:
            failures.append(f"{where}: missing {', '.join(missing)}")
            continue
        if block["boot"] != str(boot.number):
            failures.append(f"{where}: guest reported boot {block['boot']}")
        if block["step"] != boot.step:
            failures.append(f"{where}: guest reported step {block['step']}")
        want = images.get(boot.image)
        if want is None:
            failures.append(f"{where}: no image hash was given for {boot.image}")
        elif block["image"] != want:
            failures.append(
                f"{where}: image {block['image']} is not {boot.image} ({want})")
        if boot.staged is not None and block["staged"] != boot.staged:
            failures.append(
                f"{where}: staged={block['staged']}, expected {boot.staged}")
        if boot.rollback is not None and block.get("rollback", "no") != boot.rollback:
            failures.append(
                f"{where}: rollback={block.get('rollback', 'no')}, expected {boot.rollback}")
        if boot.etc is not None and block.get("etc") != boot.etc:
            failures.append(
                f"{where}: /etc edit {block.get('etc')!r}, expected {boot.etc!r}")
        if boot.image_etc is not None and block.get("image_etc") != boot.image_etc:
            failures.append(
                f"{where}: image /etc file {block.get('image_etc')!r}, "
                f"expected {boot.image_etc}")
        if boot.expect_marker is not None and block["marker"] != boot.expect_marker:
            failures.append(
                f"{where}: /var marker {block['marker']!r}, expected {boot.expect_marker!r}")
    return failures


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--serial", help="captured serial log to evaluate")
    parser.add_argument("--image-a", help="sha256 of A's image-info.json")
    parser.add_argument("--image-b", help="sha256 of B's image-info.json")
    parser.add_argument("--plan", action="store_true",
                        help="print the boot sequence and exit")
    args = parser.parse_args(argv)

    if args.plan:
        for boot in SEQUENCE:
            staged = f", staged={boot.staged}" if boot.staged else ""
            print(f"boot {boot.number}: {boot.step} on {boot.image}"
                  f"{staged}, marker={boot.expect_marker!r}")
        return 0

    if not args.serial:
        parser.error("--serial is required (there is no driver yet; this reads a log)")
    with open(args.serial, encoding="utf-8") as fh:
        text = fh.read()
    try:
        blocks = parse_blocks(text)
    except ValueError as exc:
        print(f"lifecycle: {exc}", file=sys.stderr)
        return 2
    failures = evaluate(blocks, images={"a": args.image_a, "b": args.image_b})
    if failures:
        for failure in failures:
            print(f"FAIL: {failure}", file=sys.stderr)
        return 1
    print(f"ok: {len(blocks)} boots, A -> B -> A held")
    return 0


if __name__ == "__main__":
    sys.exit(main())
