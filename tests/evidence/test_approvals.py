import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stdout, redirect_stderr
from pathlib import Path
from unittest import mock

import approvals

DIGEST = "sha256:" + "a" * 64
ENV = {
    "EVIDENCE_S3_ENDPOINT": "https://example.invalid",
    "EVIDENCE_S3_BUCKET": "bucket",
    "EVIDENCE_S3_ACCESS_KEY_ID": "key",
    "EVIDENCE_S3_SECRET_ACCESS_KEY": "secret",
}


class KeyTests(unittest.TestCase):
    def test_the_key_names_the_channel_and_the_digest(self):
        key = approvals.record_key("repo", "stable", "20261026T010203Z", DIGEST)
        self.assertEqual(key, "approvals/repo/stable/20261026T010203Z-" + "a" * 64 + ".json")

    def test_a_bad_digest_is_refused(self):
        with self.assertRaises(ValueError):
            approvals.record_key("repo", "stable", "20261026T010203Z", "sha256:abc")

    def test_a_bad_channel_is_refused(self):
        with self.assertRaises(ValueError):
            approvals.record_key("repo", "Stable", "20261026T010203Z", DIGEST)


class RecordTests(unittest.TestCase):
    def build(self, **overrides):
        args = dict(repository="repo", image="quay.io/deskos/repo", digest=DIGEST,
                    source="candidate", target="stable", previous="",
                    actor="someone", approver="", reason="the week's candidate",
                    evidence_index_sha256="f" * 64, run_url="https://run")
        args.update(overrides)
        return approvals.build(**args)

    def test_the_approver_defaults_to_the_actor(self):
        self.assertEqual(self.build()["approver"], "someone")

    def test_no_previous_is_null_not_empty(self):
        self.assertIsNone(self.build()["previous"])

    def test_a_reason_is_required(self):
        with self.assertRaises(ValueError):
            self.build(reason="  ")


class CommandTests(unittest.TestCase):
    def run_main(self, tmp, *extra):
        (Path(tmp) / "index.json").write_text(json.dumps({"digest": DIGEST}))
        argv = ["--dir", tmp, "--repository", "repo", "--image", "quay.io/deskos/repo",
                "--digest", DIGEST, "--from", "candidate", "--to", "stable",
                "--actor", "someone", "--reason", "the week's candidate",
                "--run-url", "https://run", "--stamp", "20261026T010203Z", *extra]
        out, err = io.StringIO(), io.StringIO()
        with mock.patch.dict(os.environ, ENV), redirect_stdout(out), redirect_stderr(err):
            code = approvals.main(argv)
        return code, out.getvalue(), err.getvalue()

    def test_a_dry_run_prints_the_record_and_uploads_nothing(self):
        with tempfile.TemporaryDirectory() as tmp, \
                mock.patch.object(approvals.upload, "upload") as uploaded:
            code, out, _ = self.run_main(tmp, "--dry-run")
        self.assertEqual(code, 0)
        self.assertIn('"to": "stable"', out)
        self.assertIn("would upload to approvals/repo/stable/", out)
        uploaded.assert_not_called()

    def test_the_record_is_uploaded_under_the_derived_key(self):
        with tempfile.TemporaryDirectory() as tmp, \
                mock.patch.object(approvals.upload, "upload") as uploaded:
            code, _, _ = self.run_main(tmp)
        self.assertEqual(code, 0)
        key = uploaded.call_args.args[2]
        self.assertEqual(key, "approvals/repo/stable/20261026T010203Z-" + "a" * 64 + ".json")

    def test_an_index_for_another_digest_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp, \
                mock.patch.object(approvals.upload, "upload") as uploaded:
            (Path(tmp) / "index.json").write_text(json.dumps({"digest": "sha256:" + "b" * 64}))
            argv = ["--dir", tmp, "--repository", "repo", "--image", "quay.io/deskos/repo",
                    "--digest", DIGEST, "--from", "candidate", "--to", "stable",
                    "--actor", "someone", "--reason", "x", "--run-url", "https://run",
                    "--stamp", "20261026T010203Z"]
            with mock.patch.dict(os.environ, ENV), redirect_stdout(io.StringIO()), \
                    redirect_stderr(io.StringIO()):
                code = approvals.main(argv)
        self.assertEqual(code, 1)
        uploaded.assert_not_called()


if __name__ == "__main__":
    unittest.main()
