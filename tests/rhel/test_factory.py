import hashlib
import io
import json
import os
import tarfile
import tempfile
import time
import unittest

import factory


def write_layout(root, layers):
    """Write an OCI image layout whose layers hold the given {path: bytes}."""
    blobs = os.path.join(root, "blobs", "sha256")
    os.makedirs(blobs)

    def blob(data):
        digest = hashlib.sha256(data).hexdigest()
        with open(os.path.join(blobs, digest), "wb") as f:
            f.write(data)
        return "sha256:" + digest

    descs = []
    for files in layers:
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            for name, data in files.items():
                info = tarfile.TarInfo(name)
                if data is None:
                    info.type = tarfile.DIRTYPE
                    tf.addfile(info)
                else:
                    info.size = len(data)
                    tf.addfile(info, io.BytesIO(data))
        descs.append({"digest": blob(buf.getvalue())})
    manifest = blob(json.dumps({"layers": descs}).encode())
    with open(os.path.join(root, "index.json"), "w") as f:
        json.dump({"manifests": [{"digest": manifest}]}, f)


class ScanLayersTest(unittest.TestCase):
    def scan(self, layers, markers=()):
        with tempfile.TemporaryDirectory() as d:
            write_layout(d, layers)
            return factory.scan_layers(d, list(markers))

    def test_clean_layers(self):
        count, findings = self.scan([
            {"usr/bin/true": b"\x7fELF", "var/lib/rhsm/": None, "var/log/rhsm/": None},
            {"etc/rhsm/rhsm.conf": b"[server]\n"},
        ], ["factory-host", "1234567890"])
        self.assertEqual(count, 2)
        self.assertEqual(findings, [])

    def test_rhsm_paths(self):
        _, findings = self.scan([
            {"etc/yum.repos.d/redhat.repo": b""},
            {"etc/pki/entitlement/1.pem": b"cert", "var/log/rhsm/rhsm.log": b"x"},
        ])
        self.assertEqual(findings, [
            "layer 1: etc/yum.repos.d/redhat.repo",
            "layer 2: etc/pki/entitlement/1.pem",
            "layer 2: var/log/rhsm/rhsm.log",
        ])

    def test_host_markers_in_content(self):
        _, findings = self.scan([{"etc/motd": b"built on factory-host serial 1234567890"}],
                                ["factory-host", "1234567890"])
        self.assertEqual(findings, [
            "layer 1: etc/motd contains factory-host",
            "layer 1: etc/motd contains 1234567890",
        ])


class StatusTest(unittest.TestCase):
    def test_newest_final_state(self):
        statuses = [
            {"context": "other", "state": "failure"},
            {"context": factory.CONTEXT, "state": "success"},
            {"context": factory.CONTEXT, "state": "pending"},
        ]
        self.assertEqual(factory.final_status(statuses, factory.CONTEXT), "success")

    def test_stale_pending_is_not_final(self):
        statuses = [
            {"context": factory.CONTEXT, "state": "pending"},
            {"context": factory.CONTEXT, "state": "failure"},
        ]
        self.assertIsNone(factory.final_status(statuses, factory.CONTEXT))

    def test_no_status(self):
        self.assertIsNone(factory.final_status([], factory.CONTEXT))


class RunsTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.work = self.tmp.name

    def tearDown(self):
        self.tmp.cleanup()

    def tree(self, plan=b"{}", harness=b"pass", tool=b"v1"):
        src = os.path.join(self.work, "src")
        ctx = os.path.join(self.work, "ctx")
        os.makedirs(os.path.join(src, "tests", "vm"), exist_ok=True)
        os.makedirs(ctx, exist_ok=True)
        tool_path = os.path.join(self.work, "factory.py")
        for path, data in [(os.path.join(ctx, "plan.json"), plan),
                           (os.path.join(ctx, "generated-manifest.json"), b"[]"),
                           (os.path.join(src, "tests", "vm", "bootcheck.py"), harness),
                           (tool_path, tool)]:
            with open(path, "wb") as f:
                f.write(data)
        return src, ctx, tool_path

    def summary(self, sha, **fields):
        d = os.path.join(self.work, "runs", sha)
        os.makedirs(os.path.join(d, "src"), exist_ok=True)
        with open(os.path.join(d, "summary.json"), "w") as f:
            json.dump({"commit": sha, **fields}, f)
        return d

    def test_key_follows_context_harness_and_tool(self):
        key = factory.validation_key(*self.tree())
        self.assertEqual(key, factory.validation_key(*self.tree()))
        self.assertNotEqual(key, factory.validation_key(*self.tree(plan=b'{"a":1}')))
        self.assertNotEqual(key, factory.validation_key(*self.tree(harness=b"changed")))
        self.assertNotEqual(key, factory.validation_key(*self.tree(tool=b"v2")))

    def test_reuse_only_passed_original_runs(self):
        self.summary("aaa", key="k", result="fail")
        self.summary("bbb", key="k", result="pass", reused="ccc")
        self.assertIsNone(factory.find_validated(self.work, "k", "new"))
        self.summary("ccc", key="k", result="pass")
        self.assertEqual(factory.find_validated(self.work, "k", "new"), "ccc")
        self.assertIsNone(factory.find_validated(self.work, "k", "ccc"))
        self.assertIsNone(factory.find_validated(self.work, "other", "new"))

    def test_prune_keeps_newest(self):
        dirs = []
        for i in range(4):
            dirs.append(self.summary(f"sha{i}", key="k"))
            past = time.time() - 100 + i
            os.utime(os.path.join(dirs[-1], "summary.json"), (past, past))
        factory.prune_runs(self.work, keep=2)
        self.assertEqual([os.path.exists(d) for d in dirs], [False, False, True, True])
        self.assertFalse(os.path.exists(os.path.join(dirs[2], "src")))
        self.assertTrue(os.path.exists(os.path.join(dirs[3], "src")))


if __name__ == "__main__":
    unittest.main()
