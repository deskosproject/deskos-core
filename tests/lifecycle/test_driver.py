import unittest

import driver


class QemuTests(unittest.TestCase):
    def test_the_command_forwards_ssh_and_uses_uefi(self):
        argv = driver.qemu_argv("/usr/libexec/qemu-kvm", "/tmp/w", "/tmp/w/disk.raw",
                                2222, "/code.fd", "/vars.fd")
        text = " ".join(argv)
        self.assertIn("hostfwd=tcp:127.0.0.1:2222-:22", text)
        self.assertIn("/code.fd", text)
        self.assertIn("/tmp/w/OVMF_VARS.fd", text)
        self.assertIn("/tmp/w/qemu.pid", text)
        self.assertIn("-daemonize", argv)
        self.assertNotIn("qemu-system", text)

    def test_both_ovmf_paths_must_be_given_together(self):
        with self.assertRaises(driver.HarnessError):
            driver.ovmf_paths("/code.fd", None)

    def test_an_explicit_ovmf_pair_is_used(self):
        self.assertEqual(driver.ovmf_paths("/code.fd", "/vars.fd"),
                         ("/code.fd", "/vars.fd"))

    def test_a_named_binary_wins(self):
        # Any existing absolute path stands in for qemu here.
        self.assertEqual(driver.qemu_binary("/bin/sh"), "/bin/sh")


class ImageBTests(unittest.TestCase):
    def test_both_markers_are_one_run_instruction(self):
        text = driver.image_b_containerfile()
        self.assertIn(f"RUN echo b > {driver.B_MARKER} && echo b > {driver.B_ETC}",
                      text)
        self.assertNotIn("\n &&", text)


class ImageDTests(unittest.TestCase):
    def test_the_candidate_removes_the_init_binary(self):
        text = driver.image_d_containerfile()
        self.assertIn("FROM localhost/deskos-lifecycle:a", text)
        self.assertIn("RUN rm -f /usr/lib/systemd/systemd", text)


class GrubEnvTests(unittest.TestCase):
    ENV = "default=0\nsaved_entry=0\n" + "#" * 32 + "\n"

    def test_sets_the_default_without_changing_the_block_size(self):
        updated = driver.with_grub_default(self.ENV, "1")
        self.assertIn("default=1\n", updated)
        self.assertEqual(len(updated), len(self.ENV))

    def test_inserts_a_default_when_the_environment_has_none(self):
        text = "# GRUB Environment Block\n# WARNING: do not edit\n" + "#" * 64
        updated = driver.with_grub_default(text, "1")
        self.assertIn("default=1\n", updated)
        self.assertEqual(len(updated), len(text))

    def test_refuses_an_environment_with_no_padding(self):
        with self.assertRaises(ValueError):
            driver.with_grub_default("# GRUB Environment Block\n# WARNING: do not edit\n", "1")

    def test_refuses_a_value_that_changes_the_block_size(self):
        with self.assertRaises(ValueError):
            driver.with_grub_default(self.ENV, "previous")

    def test_finds_the_entry_by_verity_and_uses_grub_order(self):
        entries = [
            ("bootc-bootc-1", "options composefs.digest=?v1-sha512-12:new"),
            ("bootc-bootc-0", "options composefs.digest=?v1-sha512-12:old"),
        ]
        self.assertEqual(driver.grub_entry_index(entries, "old"), 0)
        self.assertEqual(driver.grub_entry_index(entries, "new"), 1)
        with self.assertRaises(ValueError):
            driver.grub_entry_index(entries, "missing")


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
        entry = driver.block(2, "stage", "sha256:a", True, False, None,
                             "local", True)
        self.assertEqual(entry, {
            "boot": "2", "step": "stage", "image": "sha256:a",
            "staged": "yes", "rollback": "no", "marker": "-",
            "etc": "local", "image_etc": "yes",
        })


if __name__ == "__main__":
    unittest.main()
