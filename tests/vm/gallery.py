#!/usr/bin/env python3
"""Select the screenshots the documentation site shows.

`image-bootcheck` uploads one artifact per workstation. This reads those
artifacts and copies two frames per shipped platform into the built site: the
boot splash and the final graphical frame of the instrumented session. It also
writes `manifest.json`, which names the run, the commit, the workstation and the
frame each image came from, so a page can link to the evidence instead of
asking a reader to trust a picture.

Only the Core Fedora 45 and CentOS Stream 10 workstations are selected. The
example organization image is a test fixture, not a shipped desktop.
"""
from __future__ import annotations

import argparse
import json
import shutil
from pathlib import Path

PLATFORMS = {
    "fedora-45": "vm-evidence-core-fedora-45-attempt-",
    "centos-stream-10": "vm-evidence-core-centos-stream-10-attempt-",
}


def attempt_number(path: Path) -> int:
    """The run attempt at the end of an artifact name."""
    return int(path.name.rsplit("-attempt-", 1)[-1])


def latest_artifact(root: Path, prefix: str) -> Path | None:
    """The highest attempt of a platform, so a retry wins over the first try."""
    matches = [p for p in root.iterdir() if p.is_dir() and p.name.startswith(prefix)]
    return max(matches, key=attempt_number) if matches else None


def choose(summary: dict) -> tuple[dict | None, dict | None]:
    """The boot splash and the last graphical frame of the session."""
    frames = [f for f in summary.get("frames", []) if f.get("file")]
    splash = next(
        (f for f in frames if f.get("phase") == "boot" and f.get("class") == "splash"),
        None,
    )
    graphical = [
        f for f in frames
        if f.get("phase") == "boot" and f.get("class") == "graphical"
    ]
    return splash, graphical[-1] if graphical else None


def build(root: Path, out: Path) -> dict:
    out.mkdir(parents=True, exist_ok=True)
    manifest = {"schema": "deskos.qa-gallery/v1", "platforms": {}}
    for platform, prefix in PLATFORMS.items():
        artifact = latest_artifact(root, prefix)
        if artifact is None:
            continue
        session_dir = artifact / "session"
        summary = json.loads((session_dir / "summary.json").read_text())
        identity = json.loads((artifact / "evidence" / "identity.json").read_text())
        fields = identity.get("fields", {})
        record = {
            "artifact": artifact.name,
            "run_id": fields.get("run_id"),
            "commit": fields.get("commit"),
            "workstation": fields.get("workstation"),
            "image_id": fields.get("image_id"),
            "frames": {},
        }
        for name, frame in zip(("splash", "session"), choose(summary)):
            if frame is None:
                continue
            source = session_dir / frame["file"]
            target = out / f"{platform}-{name}.png"
            shutil.copyfile(source, target)
            record["frames"][name] = {
                "file": target.name,
                "source": frame["file"],
                "t": frame.get("t"),
                "class": frame.get("class"),
            }
        manifest["platforms"][platform] = record
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return manifest


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifacts", required=True, type=Path,
                        help="directory holding the downloaded image-bootcheck artifacts")
    parser.add_argument("--out", required=True, type=Path,
                        help="directory to write the images and manifest.json")
    args = parser.parse_args()
    manifest = build(args.artifacts, args.out)
    print(f"gallery: {len(manifest['platforms'])} platform(s) to {args.out}")
    return 0 if manifest["platforms"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
