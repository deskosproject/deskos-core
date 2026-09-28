import base64
import os
import tempfile
import unittest

import sessioncheck as sc

PLAN = {
    "workstation": {"name": "ws"},
    "artifact": {
        "rpmPackages": [{"name": "firefox"}],
        "dconf": {"defaults": [
            {"key": "/org/gnome/shell/enabled-extensions", "value": "['background-logo@fedorahosted.org', 'dash-to-dock@micxgx.gmail.com']"},
            {"key": "/org/gnome/shell/favorite-apps", "value": "['firefox.desktop']"},
            {"key": "/org/gnome/desktop/background/picture-uri", "value": "'file:///usr/share/deskos/backgrounds/a.svg'"},
        ]},
    },
}

GOOD = {
    "session_type": "wayland", "disable_user_extensions": "false",
    "dock_enabled": "Yes", "dock_state": "ACTIVE",
    "favorite_apps": "['firefox.desktop']", "picture_uri": "'file:///usr/share/deskos/backgrounds/a.svg'",
    "firefox_headless_exit": "0", "firefox_screenshot": "yes",
    "system_failed_units": "", "user_failed_units": "",
}


class TmpfilesTest(unittest.TestCase):
    def test_escape(self):
        self.assertEqual(sc.tmpfiles_arg("a\\b%c\nd"), "a\\\\b%%c\\nd")

    def test_session_credentials(self):
        c = sc.credentials("session")
        self.assertIn("u deskos-qa 1500", c["sysusers.extra"])
        lines = c["tmpfiles.extra"].splitlines()
        self.assertTrue(all("\n" not in l for l in lines))
        self.assertTrue(any(l.startswith("f+ /etc/gdm/custom.conf ") and "AutomaticLogin=deskos-qa" in l for l in lines))
        self.assertIn("ConditionPathExists=!/etc/initrd-release", c["systemd.extra-unit.deskos-qa-report.service"])
        self.assertIn("Wants=deskos-qa-report.service", c["systemd.unit-dropin.graphical.target"])

    def test_journal_credentials_add_no_user(self):
        c = sc.credentials("journal")
        self.assertNotIn("sysusers.extra", c)
        self.assertIn("systemd.unit-dropin.multi-user.target", c)
        self.assertNotIn("custom.conf", c["tmpfiles.extra"])

    def test_smbios_round_trip(self):
        args = sc.smbios_args({"tmpfiles.extra": "x y\n"})
        self.assertEqual(args[0], "-smbios")
        name, b64 = args[1].split("io.systemd.credential.binary:", 1)[1].split("=", 1)
        self.assertEqual((name, base64.b64decode(b64).decode()), ("tmpfiles.extra", "x y\n"))
        self.assertNotIn(",", b64)


class ParseEvaluateTest(unittest.TestCase):
    def test_parse_serial(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "serial.log")
            with open(p, "w") as f:
                f.write("noise\nDESKOS-QA-BEGIN\na_b=1\nfavorite_apps=['x']\nDESKOS-QA-JOURNAL-BEGIN\nkernel line\nDESKOS-QA-END\n")
            values, journal = sc.parse_serial(p)
        self.assertEqual(values, {"a_b": "1", "favorite_apps": "['x']"})
        self.assertEqual(journal, "kernel line")

    def test_all_pass(self):
        self.assertTrue(all(c["pass"] for c in sc.evaluate(GOOD, PLAN)))

    def test_dock_disabled_fails(self):
        bad = dict(GOOD, disable_user_extensions="true", dock_enabled="No", dock_state="INITIALIZED")
        failed = {c["check"] for c in sc.evaluate(bad, PLAN) if not c["pass"]}
        self.assertEqual(failed, {"user extensions allowed", "Dash to Dock active"})

    def test_missing_results_fail(self):
        self.assertFalse(all(c["pass"] for c in sc.evaluate({"session_results": "missing"}, PLAN)))


if __name__ == "__main__":
    unittest.main()
