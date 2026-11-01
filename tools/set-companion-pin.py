#!/usr/bin/env python3
"""Set the runtime companion pin to the asset this release just built.

The release builds `deskos` first and `deskosctl` second. The pin is a build
input, not a file in the tagged commit: this rewrites the working tree before the
compiler and the resources are built, so the release's binary, tarball and URL
agree without the tag referencing an asset it cannot know yet.

Usage:
  set-companion-pin.py TAG SHA256 REPO [PATH]

PATH defaults to resources/software/deskos.yaml.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) not in (4, 5):
        print("usage: set-companion-pin.py TAG SHA256 REPO [PATH]", file=sys.stderr)
        return 2
    tag, sha, repo = sys.argv[1:4]
    path = Path(sys.argv[4] if len(sys.argv) == 5 else "resources/software/deskos.yaml")
    version = tag[1:] if tag.startswith("v") else tag
    url = f"https://github.com/{repo}/releases/download/{tag}/deskos-{tag}-linux-amd64"
    text = path.read_text(encoding="utf-8")
    for pattern, replacement in (
        (r"(?m)^  version: [^\n]+$", f"  version: {version}"),
        (r"(?m)^    url: [^\n]+$", f"    url: {url}"),
        (r"(?m)^    sha256: [^\n]+$", f"    sha256: {sha}"),
    ):
        text, count = re.subn(pattern, replacement, text)
        if count != 1:
            print(f"{path}: {pattern!r} matched {count} lines, want one", file=sys.stderr)
            return 1
    path.write_text(text, encoding="utf-8")
    print(f"{path}: version {version}, {url}, sha256 {sha}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
