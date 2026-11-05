import json
import tempfile
import unittest
from pathlib import Path

import release

DIGEST = "sha256:" + "a" * 64
INDEX_SHA = "b" * 64


def index():
    return {
        "digest": DIGEST,
        "objects": [
            {"name": "installed.sbom.cdx.json", "sha256": "c" * 64},
            {"name": "session/session.json", "sha256": "d" * 64},
        ],
    }


def approval():
    return {
        "schema": "https://deskos.org/supply-chain/promotion/v1",
        "repository": "deskos-core-centos-stream-10",
        "image": "quay.io/deskos/deskos-core-centos-stream-10",
        "digest": DIGEST,
        "from": "candidate",
        "to": "stable",
        "previous": None,
        "actor": "rarguello",
        "approver": "automatic (gate)",
        "reason": "the gate passed",
        "evidence_index_sha256": INDEX_SHA,
        "run": "https://example/run/1",
        "time": "2026-11-05T01:02:03Z",
    }


class ReleaseTest(unittest.TestCase):
    def test_record_names_the_evidence_and_the_channels(self):
        data = release.build(index(), approval(), iso="sha256:" + "e" * 64,
                             signature_scheme="keyless",
                             signature_identity="workflow@main")
        self.assertEqual(data["schema"], release.SCHEMA)
        self.assertEqual(data["digest"], DIGEST)
        self.assertEqual(data["channel"], {"from": "candidate", "to": "stable",
                                           "previous": None})
        self.assertEqual(data["iso"]["digest"], "sha256:" + "e" * 64)
        self.assertEqual(data["signature"]["scheme"], "keyless")
        self.assertEqual(data["evidence"]["index_sha256"], INDEX_SHA)
        self.assertEqual(len(data["evidence"]["objects"]), 2)

    def test_no_installer_is_a_null_field(self):
        data = release.build(index(), approval())
        self.assertIsNone(data["iso"])

    def test_another_digest_is_refused(self):
        bad = approval()
        bad["digest"] = "sha256:" + "f" * 64
        with self.assertRaises(ValueError):
            release.build(index(), bad)

    def test_a_channel_other_than_stable_is_refused(self):
        bad = approval()
        bad["to"] = "candidate"
        with self.assertRaises(ValueError):
            release.build(index(), bad)

    def test_a_missing_index_hash_is_refused(self):
        bad = approval()
        bad.pop("evidence_index_sha256")
        with self.assertRaises(ValueError):
            release.build(index(), bad)

    def test_record_key_is_digest_keyed(self):
        key = release.record_key("deskos-core-centos-stream-10", "stable",
                                 "20261105T010203Z", DIGEST)
        self.assertEqual(
            key,
            "releases/deskos-core-centos-stream-10/stable/20261105T010203Z-"
            + "a" * 64 + ".json",
        )

    def test_main_writes_the_record_dry_run(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp)
            (d / "index.json").write_text(json.dumps(index()))
            (d / "approval.json").write_text(json.dumps(approval()))
            out = d / "release.json"
            code = release.main([
                "--dir", str(d), "--approval", str(d / "approval.json"),
                "--stamp", "20261105T010203Z", "--output", str(out),
                "--dry-run",
            ])
            self.assertEqual(code, 0)
            data = json.loads(out.read_text())
            self.assertEqual(data["digest"], DIGEST)


if __name__ == "__main__":
    unittest.main()
