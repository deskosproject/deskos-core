#!/usr/bin/env python3
"""Build the digest-keyed index that verify.py checks.

    index.py --digest sha256:<64 hex> --dir EVIDENCE --output index.json

Every regular file under EVIDENCE except the output is hashed and listed by its
relative path. Symlinks and special files are refused: the evidence directory is
data, and a link could name bytes outside it. The JSON is sorted so two runs over
the same directory produce the same bytes.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from pathlib import Path

DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def build(directory: Path, digest: str, output: Path) -> dict:
    if not DIGEST.match(digest):
        raise ValueError("digest is not sha256:<64 hex>")
    if not directory.is_dir():
        raise ValueError(f"{directory} is not a directory")
    objects = []
    for root, dirs, files in os.walk(directory):
        dirs.sort()
        for name in sorted(files):
            path = Path(root) / name
            if path.resolve() == output.resolve():
                continue
            if path.is_symlink():
                raise ValueError(f"refusing symlink {path.relative_to(directory)}")
            if not path.is_file():
                raise ValueError(f"refusing non-regular file {path.relative_to(directory)}")
            objects.append({
                "name": path.relative_to(directory).as_posix(),
                "sha256": sha256_file(path),
            })
    if not objects:
        raise ValueError("no evidence files")
    data = {"digest": digest, "objects": objects}
    output.write_text(json.dumps(data, indent=2, sort_keys=False) + "\n")
    return data


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--digest", required=True)
    p.add_argument("--dir", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    args = p.parse_args()
    try:
        data = build(args.dir, args.digest, args.output)
    except (OSError, ValueError) as exc:
        print(f"evidence index: {exc}", file=sys.stderr)
        return 1
    print(f"wrote {len(data['objects'])} object(s) for {data['digest']} to {args.output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
