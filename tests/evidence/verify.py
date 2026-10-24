#!/usr/bin/env python3
"""Verify a digest-keyed evidence index and every object it names.

The index is the small document a durable store keeps beside an image digest:

    {
      "digest": "sha256:<64 hex>",
      "objects": [
        {"name": "installed.sbom.cdx.json", "sha256": "<64 hex>"},
        ...
      ]
    }

Every path is relative to the index's directory (or --dir). Verification fails
closed: a missing file, a changed hash, a duplicate name, a traversal path or a
malformed index is an error, never "nothing found". A 404 is not a clean result.

Exit 0 verified, 1 evidence failed (including an unreadable or malformed
index), 2 usage.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")


def load_index(path: Path) -> dict:
    try:
        data = json.loads(path.read_text())
    except OSError as exc:
        raise ValueError(f"cannot read {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"{path} is not valid JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise ValueError("index is not an object")
    digest = data.get("digest")
    if not isinstance(digest, str) or not DIGEST.match(digest):
        raise ValueError("index.digest is not sha256:<64 hex>")
    objects = data.get("objects")
    if not isinstance(objects, list) or not objects:
        raise ValueError("index.objects must be a non-empty list")
    seen: set[str] = set()
    for i, obj in enumerate(objects):
        if not isinstance(obj, dict):
            raise ValueError(f"index.objects[{i}] is not an object")
        name = obj.get("name")
        sha = obj.get("sha256")
        if not isinstance(name, str) or not name:
            raise ValueError(f"index.objects[{i}].name is missing")
        if name.startswith("/") or ".." in Path(name).parts:
            raise ValueError(f"index.objects[{i}].name escapes the evidence directory")
        if not isinstance(sha, str) or not SHA256.match(sha):
            raise ValueError(f"index.objects[{i}].sha256 is not 64 lowercase hex")
        if name in seen:
            raise ValueError(f"index.objects repeats {name!r}")
        seen.add(name)
    return data


def file_sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def verify(index_path: Path, directory: Path) -> list[str]:
    data = load_index(index_path)
    errors: list[str] = []
    for obj in data["objects"]:
        path = directory / obj["name"]
        if not path.is_file():
            errors.append(f"missing: {obj['name']}")
            continue
        got = file_sha256(path)
        if got != obj["sha256"]:
            errors.append(f"changed: {obj['name']} is {got}, index says {obj['sha256']}")
    return errors


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--index", required=True, type=Path)
    p.add_argument("--dir", type=Path, help="directory holding the objects (default: the index's)")
    p.add_argument("--digest", help="require the index to name this image digest")
    args = p.parse_args()

    directory = args.dir or args.index.parent
    try:
        data = load_index(args.index)
    except ValueError as exc:
        print(f"evidence: {exc}", file=sys.stderr)
        return 1
    if args.digest and data["digest"] != args.digest:
        print(f"evidence: index is for {data['digest']}, expected {args.digest}", file=sys.stderr)
        return 1
    errors = verify(args.index, directory)
    if errors:
        for e in errors:
            print(f"evidence: {e}", file=sys.stderr)
        return 1
    print(f"verified {len(data['objects'])} object(s) for {data['digest']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
