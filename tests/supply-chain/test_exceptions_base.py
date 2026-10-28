"""The accepted-risk record must name the base image the plans pin.

A base bump moves the artifact the gate scans, and every accepted path in the
record belongs to that base's layers. #240 moved the pin and left the record
behind, so the publish failed at its own guard two days later, after merge, in a
job nobody was watching. This test reads the same golden the gate compiles and
turns that into a failure at review time, where the change that caused it is.
"""
from __future__ import annotations

import json
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
RECORD = ROOT / "tests/supply-chain/exceptions.json"
# The record the supply-chain matrix pairs with this workstation.
PLAN = (
    ROOT
    / "internal/compiler/testdata/golden/deskos-core-centos-stream-10.plan.json"
)


class ExceptionBase(unittest.TestCase):
    def test_the_record_names_the_base_the_plan_pins(self):
        record = json.loads(RECORD.read_text())
        base = json.loads(PLAN.read_text())["artifact"]["baseImage"]
        where = RECORD.relative_to(ROOT)

        self.assertEqual(
            record["base"]["image"],
            base["image"],
            f"{where} was reviewed against {record['base']['image']} but the plan "
            f"pins {base['image']}: re-review the record against the new base",
        )
        self.assertEqual(
            record["base"]["digest"],
            base["digest"],
            f"{where} was reviewed against {record['base']['digest']} but the plan "
            f"pins {base['digest']}: re-review the layer paths the record accepts, "
            "then update base.digest in the same change that moves the pin",
        )


if __name__ == "__main__":
    unittest.main()
