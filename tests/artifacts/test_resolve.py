"""Tests for the artifact check, all offline.

The probes run against a local HTTP server that answers canned responses, so the paths
that matter — a 404 for a binary against a 404 for a repository, a 500, a wrong hash, a
body larger than the bound, a body that does not finish in time — are exercised for
real, through urlopen, without touching the network. A unit test that needs GitHub is a
unit test that fails when GitHub is slow.
"""

import hashlib
import http.server
import json
import os
import tempfile
import threading
import time
import unittest

import resolve

BODY = b"the artifact bytes"


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_GET(self):
        if self.path == "/ok":
            self.send_response(200)
            self.send_header("Content-Length", str(len(BODY)))
            self.end_headers()
            self.wfile.write(BODY)
        elif self.path == "/repo/repodata/repomd.xml":
            self.send_response(200)
            self.send_header("Content-Length", "2")
            self.end_headers()
            self.wfile.write(b"ok")
        elif self.path == "/big":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"x" * 8192)
        elif self.path == "/slow":
            self.send_response(200)
            self.end_headers()
            for _ in range(100):
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
        # The distinction the check turns on: urlopen raises HTTPError, but an HTTP status
        # is an answer, not a dead host.
        status, digest, err = resolve.fetch(self.base + "/404", 5)
        self.assertEqual((status, digest, err), (404, None, None))

    def test_a_500_is_also_a_response(self):
        self.assertEqual(resolve.fetch(self.base + "/500", 5)[0], 500)

    def test_a_dead_host_is_an_error(self):
        # Nothing listens on this port; that is the only kind of failure fetch reports.
        status, _, err = resolve.fetch("http://127.0.0.1:1/nothing", 2)
        self.assertIsNone(status)
        self.assertTrue(err)

    def test_the_digest_is_the_body_digest(self):
        _, digest, err = resolve.fetch(self.base + "/ok", 5, digest=True)
        self.assertIsNone(err)
        self.assertEqual(digest, hashlib.sha256(BODY).hexdigest())

    def test_the_body_is_not_read_without_a_digest_to_compute(self):
        status, digest, err = resolve.fetch(self.base + "/ok", 5)
        self.assertEqual((status, digest, err), (200, None, None))

    def test_a_body_over_the_bound_is_refused(self):
        _, _, err = resolve.fetch(self.base + "/big", 5, digest=True, limit=100)
        self.assertIn("larger than", err)

    def test_a_body_that_does_not_finish_in_time_is_refused(self):
        _, _, err = resolve.fetch(self.base + "/slow", 5, digest=True,
                                  deadline=time.monotonic() + 0.3)
        self.assertIn("ran out of time", err)


class RepodataTest(unittest.TestCase):
    def test_a_variable_url_has_no_repodata_to_fetch(self):
        self.assertIsNone(resolve.repodata_url("https://r.example/RHEL/$releasever/$basearch/"))

    def test_a_plain_url_gets_the_repository_file(self):
        self.assertEqual(resolve.repodata_url("https://r.example/x"),
                         "https://r.example/x/repodata/repomd.xml")


class CheckPlanTest(ServerCase):
    def check(self, plan, **kwargs):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "plan.json")
        with open(path, "w") as f:
            json.dump(plan, f)
        r = resolve.Result()
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
        self.assertEqual(len(r.failures), 1)
        self.assertIn("!= declared", r.failures[0])

    def test_a_binary_with_the_right_hash_verifies(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "x", "url": self.base + "/ok",
             "sha256": hashlib.sha256(BODY).hexdigest()}]}})
        self.assertEqual(r.failures, [])
        self.assertTrue(any(line.startswith("- verified binary x") for line in r.lines))

    def test_a_binary_without_a_sha256_fails(self):
        # The model requires one, so a plan without it is a bug in what produced it.
        r = self.check({"artifact": {"binaries": [{"artifact": "x", "url": self.base + "/ok"}]}})
        self.assertEqual(len(r.failures), 1)
        self.assertIn("declares no sha256", r.failures[0])

    def test_a_binary_without_a_url_fails(self):
        r = self.check({"artifact": {"binaries": [{"artifact": "x"}]}})
        self.assertIn("declares no url", r.failures[0])

    def test_a_body_over_the_bound_fails(self):
        r = self.check({"artifact": {"binaries": [
            {"artifact": "x", "url": self.base + "/big", "sha256": "00"}]}}, limit=100)
        self.assertIn("larger than", r.failures[0])

    def test_a_repository_serving_repodata_is_verified(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo"}]}})
        self.assertEqual(r.failures, [])
        self.assertTrue(any(line.startswith("- verified repo r") for line in r.lines))

    def test_a_repository_that_404s_is_inconclusive_not_a_pass_and_not_a_failure(self):
        # A repository base URL is not a browsable index: a non-2xx here means nothing
        # either way, so the run says inconclusive instead of claiming success.
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/nope"}]}})
        self.assertEqual(r.failures, [])
        self.assertEqual(len(r.gaps), 1)
        self.assertTrue(any(line.startswith("- inconclusive repo r") for line in r.lines))

    def test_a_repository_with_a_variable_url_is_inconclusive(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": "https://r.example/$releasever/"}]}})
        self.assertEqual(r.failures, [])
        self.assertEqual(len(r.gaps), 1)

    def test_a_repository_with_a_malformed_key_fails_even_offline(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo",
             "gpgKeys": [{"name": "k", "content": "not a key"}]}]}})
        self.assertEqual(len(r.failures), 1)
        self.assertIn("not an armored public key", r.failures[0])

    def test_an_armored_key_does_not_fail(self):
        r = self.check({"artifact": {"rpmRepositories": [
            {"id": "r", "baseURL": self.base + "/repo",
             "gpgKeys": [{"name": "k", "content": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nx\n"}]}]}})
        self.assertEqual(r.failures, [])

    def test_an_unreadable_plan_is_a_failure_not_zero_failures(self):
        r = resolve.Result()
        resolve.check_plan("/nonexistent/plan.json", 5, r)
        self.assertEqual(len(r.failures), 1)
        self.assertIn("cannot be read", r.failures[0])


if __name__ == "__main__":
    unittest.main()
