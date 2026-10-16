"""Tests for the artifact check, all offline.

The probes run against a local HTTP server answering canned responses, so the paths that
matter — a 404 for a binary against a 404 for a repository, a 500, a wrong hash, a body
larger than the bound, a body that does not finish before the deadline, a captive portal
answering 200 with HTML — are exercised for real, through urlopen, without touching the
network. A unit test that needs GitHub is a unit test that fails when GitHub is slow.
"""

import hashlib
import http.server
import json
import os
import subprocess
import sys
import tempfile
import threading
import time
import unittest

import resolve

BODY = b"the artifact bytes"
REPOMD = b'<?xml version="1.0"?><repomd/>'
# A key-shaped body: the armor lines and base64 lines of the width PGP emits.
KEY = ("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n"
       + "mQINBGXk9pQBEADQ7fZp3hKGYZ0k9pKQ0t3Z9hVfQwXkLzBv1nHc/" * 2
       + "\n-----END PGP PUBLIC KEY BLOCK-----\n")


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def serve(self, body, status=200):
        self.send_response(status)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/ok":
            self.serve(BODY)
        elif self.path == "/rpm":
            self.serve(BODY)
        elif self.path == "/repo/repodata/repomd.xml":
            self.serve(REPOMD)
        elif self.path == "/portal/repodata/repomd.xml":
            self.serve(b"<html><body>sign in to continue</body></html>")   # a captive portal
        elif self.path == "/big":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"x" * 8192)
        elif self.path == "/slow":
            self.send_response(200)
            self.end_headers()
            for _ in range(200):
                try:
                    self.wfile.write(b"y")
                    self.wfile.flush()
                except OSError:
                    break
                time.sleep(0.05)
        elif self.path == "/500":
            self.send_error(500)
        else:
            self.send_error(404)


class ServerCase(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        cls.base = f"http://127.0.0.1:{cls.srv.server_address[1]}"
        threading.Thread(target=cls.srv.serve_forever, daemon=True).start()

    @classmethod
    def tearDownClass(cls):
        cls.srv.shutdown()
        cls.srv.server_close()


class FetchTest(ServerCase):
    def test_a_404_is_a_response_the_caller_judges(self):
        # The distinction the check turns on: urlopen raises HTTPError, but a status is an
        # answer, not a dead host.
        got = resolve.fetch(self.base + "/404", 5)
        self.assertEqual((got.status, got.error), (404, None))
        self.assertFalse(got.ok)

    def test_a_500_is_also_a_response(self):
        got = resolve.fetch(self.base + "/500", 5)
        self.assertEqual((got.status, got.error, got.ok), (500, None, False))

    def test_a_dead_host_is_an_error(self):
        got = resolve.fetch("http://127.0.0.1:1/nothing", 2)
        self.assertIsNone(got.status)
        self.assertTrue(got.error)
        self.assertFalse(got.ok)

    def test_a_200_is_ok(self):
        self.assertTrue(resolve.fetch(self.base + "/ok", 5).ok)

    def test_the_digest_is_the_body_digest(self):
        got = resolve.fetch(self.base + "/ok", 5, digest=True)
        self.assertEqual(got.digest, hashlib.sha256(BODY).hexdigest())

    def test_the_body_is_not_read_without_a_reason(self):
        got = resolve.fetch(self.base + "/ok", 5)
        self.assertIsNone(got.digest)
        self.assertIsNone(got.prefix)

    def test_a_prefix_is_returned_when_asked(self):
        got = resolve.fetch(self.base + "/repo/repodata/repomd.xml", 5, prefix=True)
        self.assertTrue(got.prefix.startswith(b"<?xml"))

    def test_a_body_over_the_bound_is_refused(self):
        got = resolve.fetch(self.base + "/big", 5, digest=True, limit=100)
        self.assertIn("larger than", got.error)

    def test_the_deadline_bounds_the_elapsed_time_not_only_the_message(self):
        # The server dribbles for ~10 s; with a 0.3 s deadline and a 2 s socket timeout the
        # call has to come back well before the body is done. Asserting the message alone
        # would not have caught a deadline that only checks between reads without handing
        # the remaining time to the socket.
        start = time.monotonic()
        got = resolve.fetch(self.base + "/slow", 2, digest=True,
                            deadline=time.monotonic() + 0.3)
        elapsed = time.monotonic() - start
        self.assertTrue(got.error, "expected the deadline to stop the read")
        self.assertLess(elapsed, 3.0, f"took {elapsed:.1f}s, the deadline did not bound it")


class ShapesTest(unittest.TestCase):
    def test_a_variable_url_has_no_repodata_to_fetch(self):
        self.assertIsNone(resolve.repodata_url("https://r.example/RHEL/$releasever/$basearch/"))

    def test_a_plain_url_gets_the_repository_file(self):
        self.assertEqual(resolve.repodata_url("https://r.example/x"),
                         "https://r.example/x/repodata/repomd.xml")

    def test_repomd_is_xml_not_any_markup(self):
        self.assertTrue(resolve.looks_like_repomd(REPOMD))
        self.assertFalse(resolve.looks_like_repomd(b"<html><body>portal</body></html>"))
        self.assertFalse(resolve.looks_like_repomd(b""))

    def test_an_armored_key_is_a_shape_check(self):
        self.assertTrue(resolve.looks_armored(KEY))
        # Missing the end line, and a body too short to be a key: both refused, which is
        # what "looks like an armored public key" is allowed to claim.
        self.assertFalse(resolve.looks_armored(KEY.replace("-----END PGP PUBLIC KEY BLOCK-----\n", "")))
        self.assertFalse(resolve.looks_armored("-----BEGIN PGP PUBLIC KEY BLOCK-----\nx\n"
                                              "-----END PGP PUBLIC KEY BLOCK-----\n"))
        self.assertFalse(resolve.looks_armored("not a key"))


class CheckPlanTest(ServerCase):
    def check(self, plan, **kwargs):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "plan.json")
        with open(path, "w") as f:
            json.dump(plan, f)
        r = resolve.Report()
        resolve.check_plan(path, 5, r, **kwargs)
        return r

    def test_a_binary_that_404s_fails(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "lazygit", "url": self.base + "/404", "sha256": "00"}]}})
        self.assertEqual(len(r.failures), 1)
        self.assertIn("HTTP 404", r.failures[0])

    def test_a_binary_with_a_wrong_hash_fails(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "x", "url": self.base + "/ok", "sha256": "00"}]}})
        self.assertIn("!= declared", r.failures[0])

    def test_a_binary_with_the_right_hash_verifies(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "x", "url": self.base + "/ok", "sha256": hashlib.sha256(BODY).hexdigest()}]}})
        self.assertEqual(r.failures, [])
        self.assertTrue(any(line.startswith("- verified binary x") for line in r.lines))

    def test_an_rpm_file_takes_the_same_path(self):
        r = self.check({"artifact": {"rpmFiles": [
            {"url": self.base + "/rpm", "sha256": hashlib.sha256(BODY).hexdigest()}]}})
        self.assertEqual(r.failures, [])
        self.assertTrue(any(line.startswith("- verified rpmFile") for line in r.lines))

    def test_a_binary_without_a_sha256_fails(self):
        # The model requires one, so a plan without it is a bug in what produced it.
        r = self.check({"artifact": {"binaries": [{"artifact": "x", "url": self.base + "/ok"}]}})
        self.assertIn("declares no sha256", r.failures[0])

    def test_a_binary_without_a_url_fails(self):
        r = self.check({"artifact": {"binaries": [{"artifact": "x"}]}})
        self.assertIn("declares no url", r.failures[0])

    def test_a_body_over_the_bound_fails(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "x", "url": self.base + "/big", "sha256": "00"}]}}, limit=100)
        self.assertIn("larger than", r.failures[0])

    def test_a_repository_serving_repomd_is_verified(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo"}]}})
        self.assertEqual(r.failures, [])
        self.assertTrue(any(line.startswith("- verified repo r") for line in r.lines))

    def test_a_captive_portal_answering_200_with_html_is_inconclusive(self):
        # A 2xx is not evidence of a repository on its own, which is what "every response
        # is success" got wrong.
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/portal"}]}})
        self.assertEqual(r.failures, [])
        self.assertEqual(len(r.gaps), 1)
        self.assertIn("does not look like repomd", r.gaps[0])

    def test_a_repository_that_404s_is_inconclusive(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/nope"}]}})
        self.assertEqual(r.failures, [])
        self.assertEqual(len(r.gaps), 1)

    def test_a_repository_with_a_variable_url_is_inconclusive(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": "https://r.example/$releasever/"}]}})
        self.assertEqual(r.failures, [])
        self.assertEqual(len(r.gaps), 1)

    def test_a_repository_with_a_malformed_key_fails_even_offline(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo",
             "gpgKeys": [{"name": "k", "content": "not a key"}]}]}})
        self.assertIn("not an armored public key", r.failures[0])

    def test_an_armored_key_is_verified(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo",
             "gpgKeys": [{"name": "k", "content": KEY}]}]}})
        self.assertEqual(r.failures, [])

    def test_an_unreadable_plan_is_a_failure_and_its_own_class(self):
        r = resolve.Report()
        resolve.check_plan("/nonexistent/plan.json", 5, r)
        self.assertEqual(len(r.failures), 1)
        self.assertEqual(len(r.unreadable), 1)
        self.assertIn("cannot be read", r.failures[0])


class ExitContractTest(ServerCase):
    """The documented exit codes, exercised through the command line."""

    def run_check(self, *plans):
        script = os.path.join(os.path.dirname(os.path.abspath(__file__)), "resolve.py")
        return subprocess.run([sys.executable, script, *plans],
                              capture_output=True, text=True)

    def plan(self, artifact):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "plan.json")
        with open(path, "w") as f:
            json.dump({"artifact": artifact}, f)
        return path

    def test_0_when_there_is_nothing_to_report(self):
        got = self.run_check(self.plan({}))
        self.assertEqual(got.returncode, 0, got.stderr)

    def test_1_when_a_failure_was_found(self):
        got = self.run_check(self.plan({"binaries": [
            {"artifact": "x", "url": self.base + "/404", "sha256": "00"}]}))
        self.assertEqual(got.returncode, 1, got.stderr)

    def test_2_when_a_plan_could_not_be_read(self):
        got = self.run_check("/nonexistent/plan.json")
        self.assertEqual(got.returncode, 2, got.stderr)
        self.assertIn("error", got.stdout)


if __name__ == "__main__":
    unittest.main()
