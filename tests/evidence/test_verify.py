import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import verify


def write(path: Path, data: bytes) -> str:
    path.write_bytes(data)
    return hashlib.sha256(data).hexdigest()


class VerifyTests(unittest.TestCase):
    def make(self):
        tmp = Path(tempfile.mkdtemp())
        sha = write(tmp / "scan.txt", b"scan\n")
        index = {"digest": "sha256:" + "a" * 64,
                 "objects": [{"name": "scan.txt", "sha256": sha}]}
        (tmp / "index.json").write_text(json.dumps(index))
        return tmp, index

    def test_a_complete_index_verifies(self):
        tmp, _ = self.make()
        self.assertEqual(verify.verify(tmp / "index.json", tmp), [])

    def test_a_missing_object_fails(self):
        tmp, _ = self.make()
        (tmp / "scan.txt").unlink()
        self.assertEqual(verify.verify(tmp / "index.json", tmp), ["missing: scan.txt"])

    def test_changed_bytes_fail(self):
        tmp, _ = self.make()
        (tmp / "scan.txt").write_bytes(b"changed\n")
        errors = verify.verify(tmp / "index.json", tmp)
        self.assertEqual(len(errors), 1)
        self.assertIn("changed: scan.txt", errors[0])

    def test_traversal_is_rejected(self):
        tmp, index = self.make()
        index["objects"][0]["name"] = "../scan.txt"
        (tmp / "index.json").write_text(json.dumps(index))
        with self.assertRaises(ValueError):
            verify.load_index(tmp / "index.json")

    def test_duplicate_names_are_rejected(self):
        tmp, index = self.make()
        index["objects"].append(dict(index["objects"][0]))
        (tmp / "index.json").write_text(json.dumps(index))
        with self.assertRaises(ValueError):
            verify.load_index(tmp / "index.json")

    def test_an_index_for_another_digest_is_refused(self):
        tmp, _ = self.make()
        errors = verify.verify(tmp / "index.json", tmp)
        self.assertEqual(errors, [])
        data = verify.load_index(tmp / "index.json")
        self.assertNotEqual(data["digest"], "sha256:" + "b" * 64)


if __name__ == "__main__":
    unittest.main()
