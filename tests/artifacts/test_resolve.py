"""Offline tests for the artifact check. The live resolution is not tested here: it is
CI's job, and a unit test that needs the network is a unit test that fails when GitHub
is slow.
"""

import json
import os
import tempfile
import unittest

import resolve

ARMORED = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nabc\n-----END PGP PUBLIC KEY BLOCK-----\n"


def plan_of(**artifact):
    return {"artifact": artifact}


class PrefixOfTest(unittest.TestCase):
    def test_strips_at_the_first_variable(self):
        # A dnf variable cannot be resolved literally, so the fetchable prefix is used.
        self.assertEqual(resolve.prefix_of("https://r.example/RHEL/$releasever/$basearch/stable/"),
                         "https://r.example/RHEL/")

    def test_no_variable_is_left_alone(self):
        self.assertEqual(resolve.prefix_of("https://r.example/x/"), "https://r.example/x/")


class DeclaredTest(unittest.TestCase):
    def test_binaries_and_rpm_files_carry_a_url_and_a_hash(self):
        got = resolve.declared(plan_of(
            binaries=[{"artifact": "lazygit", "url": "https://e/x.tar.gz", "sha256": "aa"}],
            rpmFiles=[{"url": "https://e/y.rpm", "sha256": "bb"}]))
        self.assertEqual(got, [("binary", "lazygit", "https://e/x.tar.gz", "aa"),
                               ("rpmFile", "https://e/y.rpm", "https://e/y.rpm", "bb")])

    def test_repositories_and_files_are_not_binaries(self):
        self.assertEqual(resolve.declared(plan_of(
            rpmRepositories=[{"id": "code", "baseURL": "https://e/"}], files=[{"path": "/x"}])), [])


class CheckPlanTest(unittest.TestCase):
    def write(self, plan):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "plan.json")
        with open(path, "w") as f:
            json.dump(plan, f)
        return path

    def test_a_declared_key_that_is_not_armored_fails_without_the_network(self):
        # The key check is offline, so it can be asserted here: it is the other half of
        # "the repository is signed" and it needs no request.
        failures, lines = [], []
        path = self.write(plan_of(rpmRepositories=[{"id": "code", "gpgKeys": [{"name": "k", "content": "not a key"}]}]))
        resolve.check_plan(path, 1.0, failures, lines)
        self.assertEqual(len(failures), 1)
        self.assertIn("not an armored public key", failures[0])

    def test_an_armored_key_passes_offline_and_needs_no_repository(self):
        failures, lines = [], []
        path = self.write(plan_of(rpmRepositories=[{"id": "code", "gpgKeys": [{"name": "k", "content": ARMORED}]}]))
        self.assertTrue(resolve.check_plan(path, 1.0, failures, lines))
        self.assertEqual(failures, [])

    def test_a_binary_without_a_url_fails(self):
        failures, lines = [], []
        path = self.write(plan_of(binaries=[{"artifact": "x"}]))
        self.assertFalse(resolve.check_plan(path, 1.0, failures, lines))
        self.assertIn("declares no url", failures[0])

    def test_an_unreadable_plan_is_a_parse_error_not_a_pass(self):
        failures, lines = [], []
        self.assertFalse(resolve.check_plan("/nonexistent/plan.json", 1.0, failures, lines))


if __name__ == "__main__":
    unittest.main()
