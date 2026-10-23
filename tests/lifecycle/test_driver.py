import unittest

import driver


class QemuTests(unittest.TestCase):
    def test_the_command_forwards_ssh_and_uses_uefi(self):
        argv = driver.qemu_argv("/usr/libexec/qemu-kvm", "/tmp/w", "/tmp/w/disk.raw", 2222)
        text = " ".join(argv)
        self.assertIn("hostfwd=tcp:127.0.0.1:2222-:22", text)
        self.assertIn("OVMF_CODE.fd", text)
        self.assertIn("/tmp/w/qemu.pid", text)
        self.assertIn("-daemonize", argv)
        self.assertNotIn("qemu-system", text)

    def test_a_named_binary_wins(self):
        # Any existing absolute path stands in for qemu here.
        self.assertEqual(driver.qemu_binary("/bin/sh"), "/bin/sh")


class StatusTests(unittest.TestCase):
    STATUS = (
        'Some banner\n{"status": {"booted": {"image": {"imageDigest": "sha256:a"}},'
        ' "staged": {"image": {"imageDigest": "sha256:b"}}, "rollbackQueued": true}}'
    )

    def test_parses_json_after_a_banner(self):
        status = driver.parse_status(self.STATUS)
        self.assertEqual(driver.booted_digest(status), "sha256:a")

    def test_staged_and_rollback(self):
        status = driver.parse_status(self.STATUS)
        self.assertEqual(driver.staged_digest(status), "sha256:b")
        self.assertTrue(driver.rollback_queued(status))

    def test_no_staged_deployment_is_none(self):
        status = driver.parse_status(
            '{"status": {"booted": {"image": {"imageDigest": "sha256:a"}}, "staged": null}}')
        self.assertIsNone(driver.staged_digest(status))
        self.assertFalse(driver.rollback_queued(status))

    def test_a_non_json_report_is_an_error(self):
        with self.assertRaises(ValueError):
            driver.parse_status("not a report")


class BlockTests(unittest.TestCase):
    def test_a_block_names_what_the_evaluator_reads(self):
        entry = driver.block(2, "stage", "sha256:a", True, False, None)
        self.assertEqual(entry, {
            "boot": "2", "step": "stage", "image": "sha256:a",
            "staged": "yes", "rollback": "no", "marker": "-",
        })


if __name__ == "__main__":
    unittest.main()
