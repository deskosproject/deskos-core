import unittest

import lifecycle


def block(**kw):
    lines = [lifecycle.BEGIN]
    for key, value in kw.items():
        lines.append(f"{key}={value}")
    lines.append(lifecycle.END)
    return "\n".join(lines) + "\n"


def full_sequence(images=("sha-a", "sha-b")):
    out = []
    for boot in lifecycle.SEQUENCE:
        out.append(block(
            boot=boot.number,
            step=boot.step,
            image=images[0] if boot.image == "a" else images[1],
            staged=boot.staged or "no",
            rollback=boot.rollback or "no",
            marker=boot.expect_marker if boot.expect_marker is not None else "-",
        ))
    return "".join(out)


class ParseTests(unittest.TestCase):
    def test_parses_blocks_in_order(self):
        text = block(boot=1, step="install", image="x", staged="no", marker="-") + \
            block(boot=2, step="stage", image="x", staged="yes", marker="A1")
        blocks = lifecycle.parse_blocks(text)
        self.assertEqual([b["boot"] for b in blocks], ["1", "2"])
        self.assertEqual(blocks[1]["staged"], "yes")

    def test_unfinished_block_is_an_error(self):
        text = lifecycle.BEGIN + "\nboot=1\n"
        with self.assertRaisesRegex(ValueError, "did not finish"):
            lifecycle.parse_blocks(text)

    def test_end_without_begin_is_an_error(self):
        with self.assertRaisesRegex(ValueError, "END without BEGIN"):
            lifecycle.parse_blocks(lifecycle.END + "\n")

    def test_line_without_key_value_is_an_error(self):
        with self.assertRaisesRegex(ValueError, "without key=value"):
            lifecycle.parse_blocks(lifecycle.BEGIN + "\nnoise\n" + lifecycle.END + "\n")

    def test_duplicate_key_is_an_error(self):
        text = lifecycle.BEGIN + "\nboot=1\nboot=2\n" + lifecycle.END + "\n"
        with self.assertRaisesRegex(ValueError, "duplicate"):
            lifecycle.parse_blocks(text)


class EvaluateTests(unittest.TestCase):
    images = {"a": "sha-a", "b": "sha-b"}

    def test_a_good_sequence_holds(self):
        blocks = lifecycle.parse_blocks(full_sequence())
        self.assertEqual(lifecycle.evaluate(blocks, images=self.images), [])

    def test_a_short_run_fails(self):
        blocks = lifecycle.parse_blocks(full_sequence())[:-1]
        failures = lifecycle.evaluate(blocks, images=self.images)
        self.assertTrue(any("no report" in f for f in failures))
        self.assertTrue(any("boots reported" in f for f in failures))

    def test_the_wrong_image_fails(self):
        text = full_sequence().replace("image=sha-b", "image=sha-a", 1)
        failures = lifecycle.evaluate(lifecycle.parse_blocks(text), images=self.images)
        self.assertTrue(any("is not b" in f for f in failures))

    def test_an_unknown_image_fails(self):
        text = full_sequence().replace("image=sha-a", "image=something-else", 1)
        failures = lifecycle.evaluate(lifecycle.parse_blocks(text), images=self.images)
        self.assertTrue(any("is not a" in f for f in failures))

    def test_lost_var_state_fails(self):
        text = full_sequence().replace("marker=A1", "marker=-", 1)
        failures = lifecycle.evaluate(lifecycle.parse_blocks(text), images=self.images)
        self.assertTrue(any("/var marker" in f for f in failures))

    def test_a_staged_deployment_that_did_not_stage_fails(self):
        text = full_sequence().replace("staged=yes", "staged=no", 1)
        failures = lifecycle.evaluate(lifecycle.parse_blocks(text), images=self.images)
        self.assertTrue(any("staged=no, expected yes" in f for f in failures))

    def test_a_missing_rollback_queue_fails(self):
        blocks = lifecycle.parse_blocks(full_sequence())
        blocks[3]["rollback"] = "no"
        failures = lifecycle.evaluate(blocks, images=self.images)
        self.assertTrue(any("rollback=no, expected yes" in f for f in failures))

    def test_missing_key_fails(self):
        blocks = lifecycle.parse_blocks(full_sequence())
        del blocks[1]["staged"]
        failures = lifecycle.evaluate(blocks, images=self.images)
        self.assertTrue(any("missing staged" in f for f in failures))


if __name__ == "__main__":
    unittest.main()
