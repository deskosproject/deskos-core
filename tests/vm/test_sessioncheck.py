import base64
import json
import os
import tempfile
import unittest

import sessioncheck as sc

PLAN = {
    "workstation": {"name": "ws"},
    "artifact": {
        "rpmPackages": [{"name": "firefox"}],
        "dconf": {"name": "distro", "defaults": [
            {"key": "/org/gnome/shell/enabled-extensions", "type": "as",
             "value": "['background-logo@fedorahosted.org', 'dash-to-dock@micxgx.gmail.com']"},
            {"key": "/org/gnome/shell/favorite-apps", "type": "as", "value": "['firefox.desktop']"},
            {"key": "/org/gnome/desktop/background/picture-uri", "type": "s",
             "value": "'file:///usr/share/deskos/backgrounds/a.svg'"},
            {"key": "/org/gnome/desktop/session/idle-delay", "type": "u", "value": "uint32 600"},
            {"key": "/org/gnome/desktop/screensaver/lock-enabled", "type": "b", "value": "true"},
            {"key": "/org/gnome/software/download-updates", "type": "b", "value": "false"},
        ]},
    },
}


def reported(defaults):
    """What the guest reports when every default matches the plan, by index.

    The guest reads the effective value (`dconf_<i>`) and the value in the image's
    own database (`dconfdb_<i>`) for each key; both are the plan's here.
    """
    out = {}
    for i, d in enumerate(defaults):
        out[f"dconf_{i}"] = out[f"dconfdb_{i}"] = d["value"]
    return out


GOOD = {
    "session_type": "wayland", "disable_user_extensions": "false",
    "dock_enabled": "Yes", "dock_state": "ACTIVE",
    "favorite_apps": "['firefox.desktop']", "picture_uri": "'file:///usr/share/deskos/backgrounds/a.svg'",
    "firefox_headless_exit": "0", "firefox_screenshot": "yes",
    "system_failed_units": "", "user_failed_units": "",
    **reported(sc.plan_defaults(PLAN)),
}


def default_check(values, key, plan=PLAN):
    name = sc.DEFAULT_CHECK_PREFIX + key
    return next(c for c in sc.evaluate(values, plan) if c["check"] == name)


class TmpfilesTest(unittest.TestCase):
    def test_escape(self):
        self.assertEqual(sc.tmpfiles_arg("a\\b%c\nd"), "a\\\\b%%c\\nd")

    def test_oversized_argument_is_refused_loudly(self):
        # systemd drops a tmpfiles argument over PATH_MAX-1 in silence; refusing
        # it here turns that silence into a local failure.
        with self.assertRaises(ValueError):
            sc.tmpfiles_arg("x" * (sc.TMPFILES_ARG_MAX + 1))
        self.assertEqual(len(sc.tmpfiles_arg("x" * sc.TMPFILES_ARG_MAX)), sc.TMPFILES_ARG_MAX)

    def test_the_cap_is_on_the_resolved_content_not_the_escaped_form(self):
        # systemd expands specifiers after unescaping, and the PATH_MAX-1 cap is
        # on the result, so escaping that only lengthens the text does not count
        # against it: 4095 B of "%" is 8190 B escaped and is still written.
        self.assertEqual(len(sc.tmpfiles_arg("%" * sc.TMPFILES_ARG_MAX)), 2 * sc.TMPFILES_ARG_MAX)
        with self.assertRaises(ValueError):
            sc.tmpfiles_arg("%" * (sc.TMPFILES_ARG_MAX + 1))

    def test_the_cap_counts_bytes_not_characters(self):
        # The cap is on the resolved content in bytes: "é" is two bytes in UTF-8,
        # so 2047 of them (4094 B) pass and 2048 (4096 B) do not.
        self.assertEqual(sc.tmpfiles_arg("é" * 2047), "é" * 2047)
        with self.assertRaises(ValueError):
            sc.tmpfiles_arg("é" * 2048)

    def test_session_credentials(self):
        c = sc.credentials("session")
        self.assertIn("u deskos-qa 1500", c["sysusers.extra"])
        lines = c["tmpfiles.extra"].splitlines()
        self.assertTrue(all("\n" not in l for l in lines))
        self.assertTrue(any(l.startswith("f+ /etc/gdm/custom.conf ") and "AutomaticLogin=deskos-qa" in l for l in lines))
        # GDM's config path is the distribution's: /etc/gdm3 on Debian and Ubuntu.
        self.assertTrue(any(l.startswith("f+ /etc/gdm3/custom.conf ") and "AutomaticLogin=deskos-qa" in l for l in lines))
        self.assertIn("ConditionPathExists=!/etc/initrd-release", c["systemd.extra-unit.deskos-qa-report.service"])
        self.assertIn("Wants=deskos-qa-report.service", c["systemd.unit-dropin.graphical.target"])

    def test_journal_credentials_add_no_user(self):
        c = sc.credentials("journal")
        self.assertNotIn("sysusers.extra", c)
        self.assertIn("systemd.unit-dropin.multi-user.target", c)
        self.assertNotIn("custom.conf", c["tmpfiles.extra"])

    def test_session_credentials_install_the_plan_defaults(self):
        # The plan's keys travel as a file of their own (the tmpfiles argument cap
        # is on one file's content), and a second profile names only the image's
        # own database so its value is read apart from the session's.
        keys = [d["key"] for d in sc.plan_defaults(PLAN)]
        c = sc.credentials("session", dconf_keys=keys, dconf_db="distro")
        lines = c["tmpfiles.extra"].splitlines()
        keyfile = next(l for l in lines if l.startswith(f"f {sc.DCONF_KEYS_PATH} "))
        self.assertEqual(keyfile.split(" - ", 1)[1].replace("\\n", "\n"), "".join(k + "\n" for k in keys))
        profile = next(l for l in lines if l.startswith(f"f {sc.DCONF_PROFILE_PATH} "))
        self.assertTrue(profile.endswith("system-db:distro\\n"))
        # The script reads the keys from the file, and so carries none of them.
        script = next(l for l in lines if l.startswith(f"f {sc.HOME}/deskos-qa-check.sh "))
        for k in keys:
            self.assertNotIn(k, script)
        self.assertIn(f"mapfile -t dconf_keys < {sc.DCONF_KEYS_PATH}", script.replace("\\n", "\n"))

    def test_journal_credentials_ignore_the_plan_defaults(self):
        c = sc.credentials("journal", dconf_keys=[d["key"] for d in sc.plan_defaults(PLAN)], dconf_db="distro")
        self.assertNotIn(sc.DCONF_KEYS_PATH, c["tmpfiles.extra"])
        self.assertNotIn(sc.DCONF_PROFILE_PATH, c["tmpfiles.extra"])
        self.assertNotIn("dconf read", c["tmpfiles.extra"])

    def test_a_session_without_defaults_reads_none(self):
        c = sc.credentials("session")
        self.assertNotIn(sc.DCONF_KEYS_PATH, c["tmpfiles.extra"])
        self.assertNotIn(sc.DCONF_PROFILE_PATH, c["tmpfiles.extra"])

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


class PlanDefaultsTest(unittest.TestCase):
    def test_the_plan_is_the_list_of_defaults(self):
        # What is asserted comes from the plan, not from a constant in the checker.
        checks = [c for c in sc.evaluate(GOOD, PLAN) if c["check"].startswith(sc.DEFAULT_CHECK_PREFIX)]
        self.assertEqual([c["check"] for c in checks],
                         [sc.DEFAULT_CHECK_PREFIX + d["key"] for d in sc.plan_defaults(PLAN)])
        self.assertTrue(all(c["pass"] for c in checks))

    def test_a_default_declared_later_is_asserted_without_touching_the_checker(self):
        plan = json.loads(json.dumps(PLAN))
        plan["artifact"]["dconf"]["defaults"].append(
            {"key": "/org/gnome/desktop/interface/clock-format", "type": "s", "value": "'24h'"})
        # The guest reported the plan without the new default, so it has no value.
        values = dict(GOOD)
        self.assertFalse(default_check(values, "/org/gnome/desktop/interface/clock-format", plan)["pass"])

    def test_a_value_that_matches_neither_the_session_nor_the_image_fails(self):
        values = dict(GOOD, dconf_3="uint32 900", dconfdb_3="uint32 900")
        self.assertFalse(default_check(values, "/org/gnome/desktop/session/idle-delay")["pass"])

    def test_a_default_the_guest_never_reported_fails(self):
        values = {k: v for k, v in GOOD.items() if k != "dconf_4"}
        c = default_check(values, "/org/gnome/desktop/screensaver/lock-enabled")
        self.assertFalse(c["pass"])
        self.assertIn("did not report", c["detail"])

    def test_an_array_whose_order_differs_passes_and_says_so(self):
        # The plan does not say whether an array's order matters, so it is compared
        # as a multiset; the order-sensitive checks (favorites) stay their own.
        values = dict(GOOD, dconf_0="['dash-to-dock@micxgx.gmail.com', 'background-logo@fedorahosted.org']")
        c = default_check(values, "/org/gnome/shell/enabled-extensions")
        self.assertTrue(c["pass"])
        self.assertFalse(c["skipped"])
        self.assertIn("different order", c["detail"])

    def test_an_array_with_other_members_fails(self):
        wrong = "['background-logo@fedorahosted.org', 'other@example.com']"
        values = dict(GOOD, dconf_0=wrong, dconfdb_0=wrong)
        self.assertFalse(default_check(values, "/org/gnome/shell/enabled-extensions")["pass"])

    def test_an_empty_array_matches_its_typed_form(self):
        # dconf prints an empty array as `@as []` where the plan stores `[]`: the
        # comparison is by value, not by text.
        plan = json.loads(json.dumps(PLAN))
        plan["artifact"]["dconf"]["defaults"][1] = {"key": "/org/gnome/shell/favorite-apps", "value": "[]"}
        values = dict(GOOD, dconf_1="@as []")
        self.assertTrue(default_check(values, "/org/gnome/shell/favorite-apps", plan)["pass"])

    def test_a_value_the_session_changed_is_reported_not_failed(self):
        # The image still carries the plan's default; a user, or a local or site
        # database, wins over one by design, so the effective value is reported.
        values = dict(GOOD, dconf_1="['other.desktop']")
        c = default_check(values, "/org/gnome/shell/favorite-apps")
        self.assertTrue(c["pass"])
        self.assertTrue(c["skipped"])
        self.assertIn("session sees", c["detail"])
        self.assertEqual(sc.check_status(c), "SKIP")
        summary = sc.plan_default_summary(sc.evaluate(values, PLAN))
        self.assertEqual(summary["asserted"], len(sc.plan_defaults(PLAN)) - 1)
        self.assertEqual([s["check"] for s in summary["skipped"]], [c["check"]])

    def test_a_default_missing_from_the_image_database_fails(self):
        # The session value and the image's own database both differ: the image
        # does not carry the declared default, so this is not a session override.
        values = dict(GOOD, dconf_1="['other.desktop']", dconfdb_1=sc.UNSET)
        c = default_check(values, "/org/gnome/shell/favorite-apps")
        self.assertFalse(c["pass"])
        self.assertFalse(c["skipped"])

    def test_a_key_the_session_cannot_see_is_reported(self):
        # The image carries the default, but the session's own profile does not
        # reach the database that holds it: reported, not asserted.
        values = dict(GOOD, dconf_4=sc.UNSET)
        c = default_check(values, "/org/gnome/desktop/screensaver/lock-enabled")
        self.assertTrue(c["pass"])
        self.assertTrue(c["skipped"])
        self.assertIn("profile", c["detail"])

    def test_an_unreadable_effective_value_is_reported(self):
        values = dict(GOOD, dconf_4=sc.ERROR)
        c = default_check(values, "/org/gnome/desktop/screensaver/lock-enabled")
        self.assertTrue(c["pass"])
        self.assertTrue(c["skipped"])
        self.assertIn("could not be read", c["detail"])

    def test_a_plan_without_defaults_asserts_none(self):
        checks = sc.evaluate(GOOD, {"artifact": {"dconf": None}})
        self.assertEqual(sc.plan_default_summary(checks)["declared"], 0)
        self.assertTrue(all(c["pass"] for c in checks))


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
