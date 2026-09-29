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


MCELOG_FAILED = dict(
    GOOD, system_failed_units="mcelog.service",
    diag_cpu_vendor="AuthenticAMD", diag_cpu_family="25", diag_cpu_model_name="AMD EPYC 7763 64-Core Processor",
    diag_edac_mce_amd="absent", diag_dev_mcelog="present",
    diag_mcelog_service="LoadState=loaded;ActiveState=failed;SubState=failed;Result=exit-code;ConditionResult=yes;ExecMainStatus=1",
)
MCE_JOURNAL = ("[ 1.0] kernel: tail\n--- failed unit mcelog.service\n[ 8.6] mcelog: CPU is unsupported\n"
               "--- kernel mce/edac lines\r\n[ 0.2] kernel: mce: CPU0: Thermal monitoring enabled\r\n"
               "[ 0.3] kernel: EDAC MC: Ver: 3.0.0\r\n--- SELinux AVC denials\ntype=AVC x\n")


class PlatformDiagnosticsTest(unittest.TestCase):
    def test_unit_properties(self):
        self.assertEqual(sc.unit_properties("ActiveState=failed;ExecMainStatus=1;;junk"),
                         {"ActiveState": "failed", "ExecMainStatus": "1"})
        self.assertEqual(sc.unit_properties(None), {})

    def test_report_collects_diagnostics(self):
        for want in ("diag_cpu_vendor=", "diag_edac_mce_amd=absent", "diag_dev_mcelog=absent",
                     "systemctl show mcelog.service -p LoadState", "-p ConditionResult -p ExecMainStatus",
                     sc.MARK_MCE, f"head -n {sc.MCE_LINES}"):
            self.assertIn(want, sc.REPORT_SESSION)
        head, journal = sc.REPORT_SESSION.split(sc.MARK_JOURNAL)
        self.assertIn("diag_mcelog_service=", head)
        self.assertLess(journal.index(sc.MARK_MCE), journal.index(sc.MARK_AVC))

    def test_mcelog_failure_stays_a_failure(self):
        checks = sc.evaluate(MCELOG_FAILED, PLAN)
        self.assertEqual({c["check"] for c in checks if not c["pass"]}, {"no unexpected failed system units"})
        d = sc.platform_diagnostics(MCELOG_FAILED, MCE_JOURNAL)
        self.assertIs(d["mcelog_failed"], True)
        self.assertEqual((d["cpu_vendor"], d["cpu_family"], d["edac_mce_amd"], d["dev_mcelog"]),
                         ("AuthenticAMD", "25", "absent", "present"))
        self.assertEqual(d["mcelog_service"]["Result"], "exit-code")
        self.assertEqual(d["kernel_mce_edac_lines"],
                         ["[ 0.2] kernel: mce: CPU0: Thermal monitoring enabled", "[ 0.3] kernel: EDAC MC: Ver: 3.0.0"])
        md = "\n".join(sc.diagnostics_md(d))
        self.assertIn("- FAIL: mcelog.service (LoadState=loaded ActiveState=failed", md)
        self.assertIn("edac_mce_amd=absent /dev/mcelog=present", md)

    def test_skipped_mcelog_with_edac_loaded(self):
        v = dict(GOOD, diag_edac_mce_amd="live", diag_dev_mcelog="absent",
                 diag_mcelog_service="ActiveState=inactive;SubState=dead;ConditionResult=no;ExecMainStatus=0")
        d = sc.platform_diagnostics(v, "--- kernel mce/edac lines\n--- SELinux AVC denials\n")
        self.assertIs(d["mcelog_failed"], False)
        self.assertEqual((d["edac_mce_amd"], d["mcelog_service"]["ConditionResult"], d["mcelog_service"]["Result"]),
                         ("live", "no", "unknown"))
        self.assertEqual(d["kernel_mce_edac_lines"], [])
        self.assertIn("(none)", sc.diagnostics_md(d))

    def test_missing_diagnostics_are_unknown_and_not_green(self):
        d = sc.platform_diagnostics({"session_results": "missing"}, "")
        self.assertEqual(d["mcelog_failed"], "unknown")
        self.assertTrue(all(d[k] == "unknown" for k in ("cpu_vendor", "cpu_family", "cpu_model_name", "edac_mce_amd", "dev_mcelog")))
        self.assertTrue(all(v == "unknown" for v in d["mcelog_service"].values()))
        self.assertIsNone(d["kernel_mce_edac_lines"])
        self.assertIn("mcelog.service state unknown", "\n".join(sc.diagnostics_md(d)))
        no_units = {k: v for k, v in GOOD.items() if k != "system_failed_units"}
        failed = {c["check"] for c in sc.evaluate(no_units, PLAN) if not c["pass"]}
        self.assertEqual(failed, {"no unexpected failed system units"})

    def test_mce_lines_are_bounded(self):
        journal = sc.MARK_MCE + "\n" + "\n".join(f"mce {i}" for i in range(500)) + "\n" + sc.MARK_AVC
        self.assertEqual(len(sc.mce_lines(journal)), sc.MCE_LINES)


# The mcelog.service section and properties as recorded on kvm3 (CS10 guest, 2026-09-29).
KNOWN_UNIT_JOURNAL = (
    "[   12.389651] localhost.localdomain systemd[1]: Finished plymouth-quit-wait.service - Hold until boot process finishes up.\r\n"
    "--- failed unit mcelog.service\r\n"
    "[    6.914338] localhost systemd[1]: Started mcelog.service - Machine Check Exception Logging Daemon.\r\n"
    "[    6.927391] localhost mcelog[1066]: mcelog: ERROR: AMD Processor family 23: mcelog does not support this processor.  Please use the edac_mce_amd module instead.\r\n"
    "[    6.927391] localhost mcelog[1066]: CPU is unsupported\r\n"
    "[    6.981832] localhost systemd[1]: mcelog.service: Main process exited, code=exited, status=1/FAILURE\r\n"
    "[    6.981943] localhost systemd[1]: mcelog.service: Failed with result 'exit-code'.\r\n"
    "--- kernel mce/edac lines\r\n[    1.054920] localhost kernel: EDAC MC: Ver: 3.0.0\r\n--- SELinux AVC denials\r\n"
)
KNOWN_VALUES = dict(
    GOOD, system_failed_units="mcelog.service",
    diag_cpu_vendor="AuthenticAMD", diag_cpu_family="23", diag_cpu_model_name="AMD Ryzen 5 3600 6-Core Processor",
    diag_edac_mce_amd="absent", diag_dev_mcelog="present",
    diag_mcelog_service="LoadState=loaded;ActiveState=failed;SubState=failed;Result=exit-code;ConditionResult=yes;ExecMainStatus=1",
)
UNITS_CHECK = "no unexpected failed system units"


def units_check(values, journal):
    return next(c for c in sc.evaluate(values, PLAN, journal) if c["check"] == UNITS_CHECK)


class KnownMcelogTest(unittest.TestCase):
    def test_known_signature_is_non_blocking_and_visible(self):
        c = units_check(KNOWN_VALUES, KNOWN_UNIT_JOURNAL)
        self.assertTrue(c["pass"])
        self.assertIn("failed=mcelog.service", c["detail"])
        self.assertIn("known non-blocking: mcelog.service", c["detail"])
        self.assertTrue(all(x["pass"] for x in sc.evaluate(KNOWN_VALUES, PLAN, KNOWN_UNIT_JOURNAL)))
        d = sc.platform_diagnostics(KNOWN_VALUES, KNOWN_UNIT_JOURNAL)
        self.assertIs(d["mcelog_failed"], True)
        self.assertTrue(d["mcelog_known_amd_guest"])
        self.assertEqual(d["mcelog_signature_unmet"], [])
        self.assertEqual(d["mcelog_service"]["ActiveState"], "failed")
        md = "\n".join(sc.diagnostics_md(d))
        self.assertIn("- FAIL (known, non-blocking): mcelog.service, AMD guest without edac_mce_amd (LoadState=loaded ActiveState=failed", md)

    def test_present_no_initstate_also_matches(self):
        self.assertTrue(units_check(dict(KNOWN_VALUES, diag_edac_mce_amd="present-no-initstate"), KNOWN_UNIT_JOURNAL)["pass"])

    def test_any_mismatch_fails(self):
        variants = {
            "another failed unit": dict(KNOWN_VALUES, system_failed_units="mcelog.service,tuned.service"),
            "Intel CPU": dict(KNOWN_VALUES, diag_cpu_vendor="GenuineIntel"),
            "no /dev/mcelog": dict(KNOWN_VALUES, diag_dev_mcelog="absent"),
            "EDAC live": dict(KNOWN_VALUES, diag_edac_mce_amd="live"),
            "other result": dict(KNOWN_VALUES, diag_mcelog_service=KNOWN_VALUES["diag_mcelog_service"].replace("exit-code", "signal")),
            "other status": dict(KNOWN_VALUES, diag_mcelog_service=KNOWN_VALUES["diag_mcelog_service"].replace("ExecMainStatus=1", "ExecMainStatus=2")),
            "not loaded": dict(KNOWN_VALUES, diag_mcelog_service=KNOWN_VALUES["diag_mcelog_service"].replace("LoadState=loaded", "LoadState=masked")),
        }
        for name, v in variants.items():
            with self.subTest(name):
                self.assertFalse(units_check(v, KNOWN_UNIT_JOURNAL)["pass"])
                self.assertFalse(sc.platform_diagnostics(v, KNOWN_UNIT_JOURNAL)["mcelog_known_amd_guest"])

    def test_journal_signature_required(self):
        for phrase in ("does not support this processor", "edac_mce_amd"):
            with self.subTest(phrase):
                j = KNOWN_UNIT_JOURNAL.replace(phrase, "x")
                self.assertFalse(units_check(KNOWN_VALUES, j)["pass"])
        self.assertFalse(units_check(KNOWN_VALUES, "")["pass"])
        # The phrase in another unit's section or the kernel lines does not count.
        other = KNOWN_UNIT_JOURNAL.replace("--- failed unit mcelog.service", "--- failed unit other.service")
        self.assertFalse(units_check(KNOWN_VALUES, other)["pass"])
        md = "\n".join(sc.diagnostics_md(sc.platform_diagnostics(KNOWN_VALUES, "")))
        self.assertIn("- FAIL: mcelog.service", md)
        self.assertIn("not the known AMD guest signature: unit journal lacks", md)

    def test_missing_diagnostics_fail_closed(self):
        bare = dict(GOOD, system_failed_units="mcelog.service")
        c = units_check(bare, KNOWN_UNIT_JOURNAL)
        self.assertFalse(c["pass"])
        self.assertNotIn("known non-blocking", c["detail"])

    def test_no_failed_units_passes_normally(self):
        c = units_check(GOOD, "")
        self.assertTrue(c["pass"])
        self.assertEqual(c["detail"], "failed=")
        self.assertIs(sc.platform_diagnostics(GOOD, "")["mcelog_failed"], False)

    def test_unit_journal_section(self):
        self.assertIn("CPU is unsupported", sc.unit_journal(KNOWN_UNIT_JOURNAL, "mcelog.service"))
        self.assertNotIn("EDAC MC", sc.unit_journal(KNOWN_UNIT_JOURNAL, "mcelog.service"))
        self.assertIsNone(sc.unit_journal(KNOWN_UNIT_JOURNAL, "tuned.service"))


if __name__ == "__main__":
    unittest.main()
