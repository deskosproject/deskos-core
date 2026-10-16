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

    def test_failed_service_fails(self):
        got = results(dict(GOOD, system_failed_services="gdm.service"))
        self.assertFalse(got["no failed system service units"])

    def test_missing_failed_services_fails(self):
        # A report that does not say is not a pass: unknown is not clean.
        values = dict(GOOD)
        del values["system_failed_services"]
        self.assertFalse(results(values)["no failed system service units"])

    def test_failed_sockets_are_reported_but_not_asserted(self):
        # The Ubuntu image's sssd responder sockets report `dependency` because
        # sssd.service is skipped by its own condition. That is a socket, not a
        # service, and the full list stays in the detail so nothing is hidden.
        checks = fb.evaluate(GOOD)
        self.assertTrue(all(c["pass"] for c in checks))
        detail = [c["detail"] for c in checks if c["check"] == "no failed system service units"][0]
        self.assertIn("sssd-nss.socket", detail)


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
