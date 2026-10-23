import hashlib
import json
import os
import tempfile
import unittest

import identity


class IdentityTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = os.path.join(self.tmp.name, "artifact.bin")
        with open(self.path, "wb") as fh:
            fh.write(b"deskos\n")
        self.sha = hashlib.sha256(b"deskos\n").hexdigest()

    def test_builds_a_canonical_complete_record(self):
        record = identity.build_identity(
            ["commit=abc123", "run_id=42"],
            [f"disk={self.path}"],
            ["commit", "disk"],
            allow_missing=False,
        )
        self.assertTrue(record["complete"])
        self.assertEqual(record["missing"], [])
        self.assertEqual(record["files"]["disk"]["sha256"], self.sha)
        self.assertEqual(list(record["fields"]), ["commit", "run_id"])
        self.assertEqual(list(record["files"]), ["disk"])

    def test_missing_file_is_named_and_can_be_allowed(self):
        record = identity.build_identity(
            ["commit=abc123"],
            [f"disk={self.path}", "image=/does/not/exist"],
            ["commit", "disk", "image"],
            allow_missing=True,
        )
        self.assertFalse(record["complete"])
        self.assertEqual(record["missing"], ["image"])
        self.assertIsNone(record["files"]["image"]["sha256"])
        self.assertEqual(record["files"]["disk"]["sha256"], self.sha)

    def test_missing_required_file_fails_without_allow(self):
        with self.assertRaises(ValueError):
            identity.build_identity([], ["disk=/does/not/exist"], ["disk"], allow_missing=False)

    def test_duplicate_keys_are_rejected(self):
        with self.assertRaises(ValueError):
            identity.build_identity(["a=1", "a=2"], [], [], allow_missing=False)
        with self.assertRaises(ValueError):
            identity.build_identity([], [f"a={self.path}", f"a={self.path}"], [], allow_missing=False)

    def test_check_rejects_incomplete_or_wrong_schema(self):
        record = identity.build_identity([], [f"disk={self.path}"], ["disk"], allow_missing=False)
        out = os.path.join(self.tmp.name, "identity.json")
        identity.write_record(record, out)
        self.assertEqual(identity.check_record(out)["schema"], identity.SCHEMA)

        record["schema"] = "other"
        with open(out, "w", encoding="utf-8") as fh:
            json.dump(record, fh)
        with self.assertRaises(ValueError):
            identity.check_record(out)

        record["schema"] = identity.SCHEMA
        record["complete"] = False
        with open(out, "w", encoding="utf-8") as fh:
            json.dump(record, fh)
        with self.assertRaises(ValueError):
            identity.check_record(out)


if __name__ == "__main__":
    unittest.main()
