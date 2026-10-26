import io
import os
import unittest
from contextlib import redirect_stdout, redirect_stderr
from unittest import mock

import prune

DIGEST = "sha256:" + "a" * 64
PREFIX = "deskos/repo/sha256/" + "a" * 64
ENV = {
    "EVIDENCE_S3_ENDPOINT": "https://example.invalid",
    "EVIDENCE_S3_BUCKET": "bucket",
    "EVIDENCE_S3_ACCESS_KEY_ID": "key",
    "EVIDENCE_S3_SECRET_ACCESS_KEY": "secret",
}
ARGS = ["--namespace", "deskos", "--repository", "repo",
        "--digest", DIGEST, "--confirm", PREFIX]


def _xml(keys, truncated="false", token=None):
    contents = "".join(f"<Contents><Key>{key}</Key></Contents>" for key in keys)
    tail = f"<NextContinuationToken>{token}</NextContinuationToken>" if token else ""
    return (f'<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">'
            f"<IsTruncated>{truncated}</IsTruncated>{tail}{contents}"
            f"</ListBucketResult>")


def _run(argv):
    out, err = io.StringIO(), io.StringIO()
    with mock.patch.dict(os.environ, ENV), redirect_stdout(out), redirect_stderr(err):
        code = prune.main(argv)
    return code, out.getvalue(), err.getvalue()


class PrefixTests(unittest.TestCase):
    def test_the_prefix_ends_at_the_hex_digest(self):
        self.assertEqual(prune.prefix("deskos", "repo", DIGEST), PREFIX)

    def test_a_bad_digest_is_refused(self):
        with self.assertRaises(ValueError):
            prune.prefix("deskos", "repo", "sha256:abc")


class ListingTests(unittest.TestCase):
    def test_continuation_tokens_are_followed(self):
        bodies = [_xml(["a"], "true", "next"), _xml(["b"])]
        with mock.patch.object(prune, "_curl", side_effect=bodies) as curl:
            keys = prune.list_keys("https://example.invalid", "bucket", PREFIX,
                                   "key", "secret", "auto")
        self.assertEqual(keys, ["a", "b"])
        self.assertEqual(curl.call_count, 2)
        self.assertIn("continuation-token=next", " ".join(curl.call_args.args[0]))


class CommandTests(unittest.TestCase):
    def test_a_wrong_confirmation_is_refused_before_any_request(self):
        argv = ["--namespace", "deskos", "--repository", "repo",
                "--digest", DIGEST, "--confirm", "deskos/repo/sha256/" + "b" * 64]
        with mock.patch.object(prune, "list_keys") as listed:
            code, _, err = _run(argv)
        self.assertEqual(code, 2)
        self.assertIn("--confirm must be exactly", err)
        listed.assert_not_called()

    def test_a_dry_run_lists_and_deletes_nothing(self):
        with mock.patch.object(prune, "list_keys", return_value=["a", "b"]), \
                mock.patch.object(prune, "delete_key") as deleted:
            code, out, _ = _run(ARGS)
        self.assertEqual(code, 0)
        self.assertIn("2 object(s)", out)
        self.assertIn("dry run", out)
        deleted.assert_not_called()

    def test_an_indexed_prefix_needs_force(self):
        keys = [PREFIX + "/index.json", PREFIX + "/a"]
        with mock.patch.object(prune, "list_keys", return_value=keys), \
                mock.patch.object(prune, "delete_key") as deleted:
            code, _, err = _run(ARGS + ["--apply"])
        self.assertEqual(code, 1)
        self.assertIn("pass --force", err)
        deleted.assert_not_called()

    def test_apply_deletes_and_checks_the_prefix_is_empty(self):
        with mock.patch.object(prune, "list_keys", side_effect=[["a", "b"], []]), \
                mock.patch.object(prune, "delete_key") as deleted:
            code, out, _ = _run(ARGS + ["--apply"])
        self.assertEqual(code, 0)
        self.assertEqual(deleted.call_count, 2)
        self.assertIn("is empty", out)

    def test_an_object_left_behind_fails(self):
        with mock.patch.object(prune, "list_keys", side_effect=[["a"], ["a"]]), \
                mock.patch.object(prune, "delete_key"):
            code, _, err = _run(ARGS + ["--apply"])
        self.assertEqual(code, 1)
        self.assertIn("1 object(s) remain", err)


if __name__ == "__main__":
    unittest.main()
