import json
import os
import tempfile
import unittest

import compare


def comp(name, source=None):
    c = {"type": "library", "name": name}
    if source:
        c["properties"] = [{"name": "deskos:source", "value": source}]
    return c


class CompareTest(unittest.TestCase):
    def test_declared_nevra_matches_the_installed_name(self):
        missing, _ = compare.compare(
            [comp("ca-certificates-2026.2.92_v9.0.319-100.3.el10_2", "rpm")],
            [comp("ca-certificates")],
        )
        self.assertEqual(missing, [])

    def test_a_different_package_does_not_match(self):
        missing, _ = compare.compare(
            [comp("ca-certificates-extra", "rpm")],
            [comp("ca-certificates")],
        )
        self.assertEqual(missing, ["ca-certificates-extra"])

    def test_declared_rpm_must_appear(self):
        missing, others = compare.compare(
            [comp("git", "rpm"), comp("oc", "binary"), comp("root", "trust-anchor")],
            [comp("git"), comp("oc")],
        )
        self.assertEqual(missing, [])
        self.assertEqual(others, 2)

    def test_missing_rpm_is_reported(self):
        missing, _ = compare.compare(
            [comp("git", "rpm"), comp("podman", "rpm")],
            [comp("git")],
        )
        self.assertEqual(missing, ["podman"])

    def test_match_is_case_insensitive(self):
        missing, _ = compare.compare([comp("NetworkManager", "rpm")], [comp("networkmanager")])
        self.assertEqual(missing, [])

    def test_main_exit_codes(self):
        with tempfile.TemporaryDirectory() as d:
            declared = os.path.join(d, "declared.json")
            installed = os.path.join(d, "installed.json")
            with open(declared, "w", encoding="utf-8") as fh:
                json.dump({"bomFormat": "CycloneDX", "components": [comp("git", "rpm")]}, fh)
            with open(installed, "w", encoding="utf-8") as fh:
                json.dump({"bomFormat": "CycloneDX", "components": [comp("git")]}, fh)
            self.assertEqual(compare.main(["compare.py", declared, installed]), 0)
            with open(installed, "w", encoding="utf-8") as fh:
                json.dump({"bomFormat": "CycloneDX", "components": []}, fh)
            self.assertEqual(compare.main(["compare.py", declared, installed]), 1)
        self.assertEqual(compare.main(["compare.py"]), 2)


if __name__ == "__main__":
    unittest.main()
