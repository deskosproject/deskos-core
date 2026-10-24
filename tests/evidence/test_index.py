import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

import index
import verify


class IndexTests(unittest.TestCase):
    def test_round_trip_verifies(self):
        tmp = Path(tempfile.mkdtemp())
        (tmp / "scan.txt").write_text("scan\n")
        (tmp / "nested").mkdir()
        (tmp / "nested" / "plan.json").write_text("{}\n")
        digest = "sha256:" + "a" * 64
        index.build(tmp, digest, tmp / "index.json")
        self.assertEqual(verify.verify(tmp / "index.json", tmp), [])
        self.assertEqual(verify.load_index(tmp / "index.json")["digest"], digest)

    def test_the_output_is_not_indexed(self):
        tmp = Path(tempfile.mkdtemp())
        (tmp / "scan.txt").write_text("scan\n")
        index.build(tmp, "sha256:" + "a" * 64, tmp / "index.json")
        self.assertNotIn("index.json", [o["name"] for o in json.loads((tmp / "index.json").read_text())["objects"]])

    def test_an_empty_directory_fails(self):
        with self.assertRaises(ValueError):
            index.build(Path(tempfile.mkdtemp()), "sha256:" + "a" * 64, Path(tempfile.mkdtemp()) / "index.json")

    def test_a_symlink_is_refused(self):
        tmp = Path(tempfile.mkdtemp())
        (tmp / "real.txt").write_text("x\n")
        (tmp / "link.txt").symlink_to(tmp / "real.txt")
        with self.assertRaises(ValueError):
            index.build(tmp, "sha256:" + "a" * 64, tmp / "index.json")

    def test_a_bad_digest_is_refused(self):
        with self.assertRaises(ValueError):
            index.build(Path(tempfile.mkdtemp()), "sha256:short", Path(tempfile.mkdtemp()) / "index.json")


if __name__ == "__main__":
    unittest.main()
