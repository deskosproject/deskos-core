import unittest

import firstbootcheck as fb

GOOD = {
    "gis_processes": "1", "gdm_active": "active", "user_sessions": "gnome-initial-setup,gdm",
    "system_failed_services": "", "system_failed_units": "sssd-nss.socket,sssd-pam.socket",
    "loginable_accounts": "", "homes": "",
}


def results(values):
    return {c["check"]: c["pass"] for c in fb.evaluate(values)}


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

    def test_failed_units_are_reported_and_do_not_gate(self):
        # Judging a failed unit needs the known guest signature (CPU, unit
        # properties, journal), and sessioncheck.py applies it to the same disk in
        # the same job. Here a failed unit is reported and does not fail this
        # check, which is the wizard and the accounts question.
        values = dict(GOOD, system_failed_services="mcelog.service",
                      system_failed_units="mcelog.service,sssd-nss.socket")
        self.assertTrue(all(results(values).values()), results(values))
        obs = fb.observations(values)
        self.assertEqual(obs["failed_services"], "mcelog.service")
        self.assertEqual(obs["failed_units"], "mcelog.service,sssd-nss.socket")

    def test_a_report_that_does_not_say_is_unknown_not_clean(self):
        values = dict(GOOD)
        del values["system_failed_services"]
        self.assertEqual(fb.observations(values)["failed_services"], "unknown")
        self.assertEqual(fb.observations(GOOD)["failed_services"], "none")


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
