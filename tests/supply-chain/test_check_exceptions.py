import datetime
import unittest

import check_exceptions


def match(vid="GO-1", pkg="stdlib", version="go1.2", path="/usr/bin/a", sev="Critical", kind="go-module"):
    return {
        "vulnerability": {"id": vid, "severity": sev},
        "artifact": {"name": pkg, "type": kind, "version": version, "locations": [{"path": path}]},
    }


def record(review="2999-01-01", digest="sha256:base"):
    return {
        "owner": "tester",
        "review": review,
        "base": {"image": "quay.io/x", "digest": digest},
        "exceptions": [
            {
                "id": "GO-1",
                "kind": "accepted-risk",
                "reason": "why",
                "matches": [
                    {"package": "stdlib", "type": "go-module", "version": "go1.2", "path": "/usr/bin/a"},
                ],
            }
        ],
    }


class CheckExceptionsTest(unittest.TestCase):
    def test_approved_match_holds(self):
        self.assertEqual(check_exceptions.check(record(), {"matches": [match()]}), [])

    def test_new_location_fails(self):
        problems = check_exceptions.check(record(), {"matches": [match(path="/usr/bin/b")]})
        self.assertEqual(len(problems), 1)
        self.assertIn("unapproved match", problems[0])

    def test_new_version_fails(self):
        problems = check_exceptions.check(record(), {"matches": [match(version="go1.3")]})
        self.assertEqual(len(problems), 1)
        self.assertIn("unapproved match", problems[0])

    def test_uncovered_critical_fails(self):
        problems = check_exceptions.check(record(), {"matches": [match(vid="GO-2")]})
        self.assertEqual(len(problems), 1)
        self.assertIn("not covered by an exception", problems[0])

    def test_non_critical_is_not_gated(self):
        self.assertEqual(check_exceptions.check(record(), {"matches": [match(sev="High")]}), [])

    def test_expired_review_fails(self):
        problems = check_exceptions.check(
            record(review="2000-01-01"), {"matches": [match()]}, today=datetime.date(2026, 10, 10)
        )
        self.assertTrue(any("review expired" in p for p in problems))

    def test_expired_review_passes_on_the_day(self):
        problems = check_exceptions.check(
            record(review="2026-11-10"), {"matches": []}, today=datetime.date(2026, 11, 10)
        )
        self.assertEqual(problems, [])

    def test_base_change_fails(self):
        plan = {"artifact": {"baseImage": {"digest": "sha256:other"}}}
        problems = check_exceptions.check(record(), {"matches": []}, plan=plan)
        self.assertTrue(any("base image changed" in p for p in problems))

    def test_missing_identification_fails(self):
        broken = {"vulnerability": {"id": "GO-1", "severity": "Critical"}, "artifact": {"type": "go-module"}}
        problems = check_exceptions.check(record(), {"matches": [broken]})
        self.assertTrue(any("without component identification" in p for p in problems))

    def test_emit_gate_is_a_grype_config(self):
        text = check_exceptions.emit_gate(record())
        self.assertIn("ignore:", text)
        self.assertIn("vulnerability: GO-1", text)
        self.assertIn("name: stdlib", text)
        self.assertIn("version: go1.2", text)
        self.assertIn("review: 2999-01-01", text)

    def test_emit_gate_deduplicates_versions(self):
        rec = record()
        rec["exceptions"][0]["matches"].append(
            {"package": "stdlib", "type": "go-module", "version": "go1.2", "path": "/usr/bin/c"}
        )
        text = check_exceptions.emit_gate(rec)
        self.assertEqual(text.count("version: go1.2"), 1)


if __name__ == "__main__":
    unittest.main()
