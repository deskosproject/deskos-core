import unittest

import upload


class KeyTests(unittest.TestCase):
    def test_the_key_uses_the_repository_and_hex_digest(self):
        key = upload.object_key(
            "deskos", "deskos-core-centos-stream-10",
            "sha256:" + "a" * 64, "installed.sbom.cdx.json")
        self.assertEqual(
            key,
            "deskos/deskos-core-centos-stream-10/sha256/" + "a" * 64
            + "/installed.sbom.cdx.json")

    def test_a_traversal_name_is_refused(self):
        with self.assertRaises(ValueError):
            upload.object_key("deskos", "repo", "sha256:" + "a" * 64, "../x")

    def test_a_bad_digest_is_refused(self):
        with self.assertRaises(ValueError):
            upload.object_key("deskos", "repo", "sha256:abc", "index.json")

    def test_a_public_name_gets_a_public_content_type(self):
        self.assertEqual(upload.content_type("index.json"), "application/json")
        self.assertEqual(upload.content_type("scan.grype.txt"), "text/plain; charset=utf-8")


if __name__ == "__main__":
    unittest.main()
