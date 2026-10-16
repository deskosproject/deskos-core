import unittest

import firstbootcheck as fb

GOOD = {
    "gis_processes": "1", "gdm_active": "active", "user_sessions": "gnome-initial-setup,gdm",
    "sessions_raw": " 1 972 gnome-initial-setup|c1 972 gnome-initial-setup seat0",
    "system_failed_services": "", "system_failed_units": "sssd-nss.socket,sssd-pam.socket",
    "loginable_accounts": "", "homes": "",
}


def results(values, journal=""):
    return {c["check"]: c["pass"] for c in fb.evaluate(values, journal)}


class EvaluateTest(unittest.TestCase):
    def test_good_first_boot_passes(self):
        self.assertTrue(all(results(GOOD).values()), results(GOOD))

    def test_wizard_not_running_fails(self):
        # This is the case a screenshot cannot tell apart from a good boot: a stable
        # graphical screen with the wizard suppressed, which is what GDM alone shows.
        got = results(dict(GOOD, gis_processes="0"))
        self.assertFalse(got[f"the first-boot wizard ({fb.WIZARD}) is running"])

    def test_missing_wizard_value_fails(self):
        values = dict(GOOD)
        del values["gis_processes"]
        self.assertFalse(results(values)[f"the first-boot wizard ({fb.WIZARD}) is running"])

    def test_loginable_account_fails(self):
        got = results(dict(GOOD, loginable_accounts="someone", homes="someone"))
        self.assertFalse(got["no account can log in with a password"])

    def test_wizard_process_without_a_session_fails(self):
        # A process is not the session that came up: without a loginctl session on a
        # seat, a stable GDM screen with the wizard suppressed looks the same as the
        # wizard, which is what this check exists to tell apart.
        got = results(dict(GOOD, user_sessions="gdm", sessions_raw=" 1 100 gdm|c1 100 gdm seat0"))
        self.assertFalse(got[f"the wizard is the session that came up ({fb.WIZARD} on a seat)"])

    def test_known_guest_failure_needs_the_full_signature(self):
        # The session check's rule, reused here rather than copied: mcelog.service
        # failing on an AMD guest without edac_mce_amd, with the unit's properties and
        # its journal saying so. Every condition matters, and one that cannot be
        # verified leaves the failure unexpected.
        signature = {
            "system_failed_services": "mcelog.service", "system_failed_units": "mcelog.service",
            "diag_cpu_vendor": "AuthenticAMD", "diag_edac_mce_amd": "absent",
            "diag_dev_mcelog": "present",
            "diag_mcelog_service": "LoadState=loaded;ActiveState=failed;SubState=failed;"
                                   "Result=exit-code;ConditionResult=yes;ExecMainStatus=1",
        }
        journal = "--- failed unit mcelog.service\nmcelog: does not support this processor edac_mce_amd\n"
        got = results(dict(GOOD, **signature), journal)
        self.assertTrue(all(got.values()), got)

        for name, values, jrn in [
            ("an Intel guest", dict(signature, diag_cpu_vendor="GenuineIntel"), journal),
            ("no journal evidence", signature, ""),
            ("the unit not loaded", dict(signature, diag_mcelog_service="LoadState=not-found"), journal),
            ("another unit failed too", dict(signature, system_failed_services="mcelog.service,gdm.service"), journal),
        ]:
            self.assertFalse(results(dict(GOOD, **values), jrn)["no unexpected failed system services"], name)

    def test_a_guest_that_does_not_report_failed_services_fails(self):
        # A report that does not say is not a pass: unknown is not clean.
        values = dict(GOOD)
        del values["system_failed_services"]
        self.assertFalse(results(values)["no unexpected failed system services"])

    def test_failed_units_are_reported_in_full(self):
        # The check above judges services; the report still carries every failed unit,
        # including a socket a skipped service left behind, so nothing is hidden.
        values = dict(GOOD, system_failed_units="mcelog.service,sssd-nss.socket")
        self.assertEqual(fb.observations(values)["failed_units"], "mcelog.service,sssd-nss.socket")
        self.assertEqual(fb.observations(values)["failed_services"], "none")
        del values["system_failed_units"]
        self.assertEqual(fb.observations(values)["failed_units"], "unknown")


class CredentialsTest(unittest.TestCase):
    def test_no_account_and_no_session_are_added(self):
        creds = fb.credentials()
        self.assertNotIn("sysusers.extra", creds)
        self.assertNotIn("systemd.unit-dropin.gdm.service", creds)

    def test_installs_the_report_and_runs_it_after_the_graphical_target(self):
        creds = fb.credentials()
        self.assertIn(fb.REPORT, creds["tmpfiles.extra"])
        unit = creds["systemd.extra-unit.deskos-qa-firstboot.service"]
        self.assertIn(f"ExecStart=/bin/bash {fb.REPORT}", unit)
        self.assertIn("After=graphical.target", unit)
        self.assertIn("Wants=deskos-qa-firstboot.service", creds["systemd.unit-dropin.graphical.target"])

    def test_the_report_asserts_the_wizard_and_loginable_accounts(self):
        # The wizard's comm is truncated to 15 characters by the kernel, so the
        # report matches the command line, not the name.
        self.assertIn("GIS_RE", fb.REPORT_SCRIPT)
        self.assertIn('pgrep -f "$GIS_RE"', fb.REPORT_SCRIPT)
        self.assertIn("loginable_accounts=", fb.REPORT_SCRIPT)
        self.assertIn("/etc/shadow", fb.REPORT_SCRIPT)


if __name__ == "__main__":
    unittest.main()
