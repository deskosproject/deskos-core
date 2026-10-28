"""Workflow YAML must be what GitHub accepts, not only what PyYAML accepts.

GitHub rejects a mapping with a repeated key at startup, with a message as vague
as "workflow file issue". PyYAML keeps the last value and says nothing, so a
duplicate `env:` key shipped a workflow that every local check passed
(2026-10-28, `mirror-iso.yml`). This test parses every workflow with a loader
that refuses repeated keys, which is the only difference that matters here.
"""
from __future__ import annotations

import pathlib
import unittest

import yaml

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOWS = sorted((ROOT / ".github" / "workflows").glob("*.yml"))


class UniqueKeyLoader(yaml.SafeLoader):
    """A SafeLoader that fails on a repeated key instead of keeping the last."""


def _construct_mapping(loader, node, deep=False):
    seen = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in seen:
            raise ValueError(
                f"duplicate key {key!r} at line {key_node.start_mark.line + 1}"
            )
        seen[key] = loader.construct_object(value_node, deep=deep)
    return seen


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _construct_mapping
)


class Workflows(unittest.TestCase):
    def test_every_workflow_parses_without_duplicate_keys(self):
        self.assertTrue(WORKFLOWS, "no workflows found; the path moved")

        for path in WORKFLOWS:
            with self.subTest(workflow=path.name):
                data = yaml.load(path.read_text(), Loader=UniqueKeyLoader)
                self.assertIsInstance(data, dict)
                self.assertIn("jobs", data)
                for name, job in data["jobs"].items():
                    self.assertIsInstance(job, dict, f"job {name} is not a mapping")

    def test_every_scheduled_workflow_can_also_be_dispatched(self):
        # A schedule is best effort: GitHub runs it hours late and drops ticks
        # (2026-10-28). A workflow that only runs on a schedule has no way back
        # when that happens, so every one of them must also accept a dispatch.
        for path in WORKFLOWS:
            data = yaml.load(path.read_text(), Loader=UniqueKeyLoader)
            triggers = data.get(True) or data.get("on") or {}
            if not isinstance(triggers, dict) or "schedule" not in triggers:
                continue
            with self.subTest(workflow=path.name):
                self.assertIn(
                    "workflow_dispatch",
                    triggers,
                    f"{path.name} is scheduled but cannot be dispatched by hand",
                )

    def test_the_loader_refuses_a_duplicate_key(self):
        # The test above is only worth something if it fails on the real defect.
        with self.assertRaises(ValueError):
            yaml.load("env:\n  A: one\n  A: two\n", Loader=UniqueKeyLoader)


if __name__ == "__main__":
    unittest.main()
