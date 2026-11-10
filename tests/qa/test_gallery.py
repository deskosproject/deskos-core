import json
import tempfile
import unittest
from pathlib import Path

import gallery


def artifact(root: Path, name: str, attempt: int, frames: list[dict], **identity) -> Path:
    path = root / f"{name}-attempt-{attempt}"
    session = path / "session"
    session.mkdir(parents=True)
    (session / "summary.json").write_text(
        json.dumps({"result": "pass", "frames": frames})
    )
    (session / "frames").mkdir()
    for frame in frames:
        (session / frame["file"]).write_bytes(b"png")
    evidence = path / "evidence"
    evidence.mkdir()
    (evidence / "identity.json").write_text(
        json.dumps({"fields": {"run_id": "1", **identity}})
    )
    return path


class GalleryTest(unittest.TestCase):
    def test_choose_uses_the_boot_splash_and_the_last_graphical_frame(self):
        splash, session = gallery.choose({"frames": [
            {"phase": "boot", "class": "text", "file": "a.png"},
            {"phase": "boot", "class": "splash", "file": "b.png"},
            {"phase": "boot", "class": "graphical", "file": "c.png"},
            {"phase": "boot", "class": "graphical", "file": "d.png"},
            {"phase": "shutdown", "class": "splash", "file": "e.png"},
        ]})
        self.assertEqual(splash["file"], "b.png")
        self.assertEqual(session["file"], "d.png")

    def test_build_uses_the_highest_attempt_and_writes_the_manifest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "artifacts"
            root.mkdir()
            frames = [
                {"phase": "boot", "class": "splash", "file": "frames/splash.png"},
                {"phase": "boot", "class": "graphical", "file": "frames/session.png"},
            ]
            artifact(root, "vm-evidence-core-fedora-45", 1, frames,
                     commit="old", workstation="deskos-core-fedora-45")
            artifact(root, "vm-evidence-core-fedora-45", 2, frames,
                     commit="new", workstation="deskos-core-fedora-45")
            out = Path(tmp) / "out"

            manifest = gallery.build(root, out)

            fedora = manifest["platforms"]["fedora-45"]
            self.assertEqual(fedora["commit"], "new")
            self.assertEqual(fedora["artifact"], "vm-evidence-core-fedora-45-attempt-2")
            self.assertTrue((out / "fedora-45-splash.png").is_file())
            self.assertTrue((out / "fedora-45-session.png").is_file())
            self.assertTrue((out / "manifest.json").is_file())

    def test_frame_path_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "artifacts"
            root.mkdir()
            frames = [
                {"phase": "boot", "class": "splash", "file": "frames/splash.png"},
                {"phase": "boot", "class": "graphical", "file": "frames/session.png"},
            ]
            path = artifact(root, "vm-evidence-core-fedora-45", 1, frames)
            outside = root / "outside.png"
            outside.write_bytes(b"png")
            summary = json.loads((path / "session" / "summary.json").read_text())
            summary["frames"][0]["file"] = "../../outside.png"
            (path / "session" / "summary.json").write_text(json.dumps(summary))
            with self.assertRaises(SystemExit):
                gallery.build(root, Path(tmp) / "out")

    def test_missing_platforms_do_not_write_a_manifest_entry(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "artifacts"
            root.mkdir()
            out = Path(tmp) / "out"
            self.assertEqual(gallery.build(root, out)["platforms"], {})
            self.assertTrue((out / "manifest.json").is_file())


if __name__ == "__main__":
    unittest.main()
