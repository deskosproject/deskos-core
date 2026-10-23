#!/usr/bin/env python3
"""Write and check the identity record of a tested DeskOS artifact.

The record is deliberately small and canonical: it links the source revision,
the build context, the local image, the exported scan input, the disk that was
booted, and (when published) the registry digest and signature evidence. It is
not an SBOM and it is not a replacement for `/etc/os-release`.

Usage:
  identity.py --output FILE [--set KEY=VALUE]... [--file KEY=PATH]...
              [--require KEY]... [--allow-missing]

Exit 0 when the record is complete, or when --allow-missing is set. A missing
file is recorded with a null digest and named in the record's `missing` list.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys

SCHEMA = "deskos.identity/v1"
HEX = re.compile(r"^[0-9a-f]{64}$")


def sha256(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(8 * 1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def split_pair(value: str, what: str) -> tuple[str, str]:
    if "=" not in value:
        raise ValueError(f"{what} must be KEY=VALUE, got {value!r}")
    key, rest = value.split("=", 1)
    if not key:
        raise ValueError(f"{what} has an empty key")
    return key, rest


def build_identity(sets, files, require, allow_missing):
    fields = {}
    for item in sets:
        key, value = split_pair(item, "--set")
        if key in fields:
            raise ValueError(f"--set repeats {key!r}")
        if not value:
            continue
        fields[key] = value

    file_records = {}
    missing = set()
    for item in files:
        key, path = split_pair(item, "--file")
        if key in file_records:
            raise ValueError(f"--file repeats {key!r}")
        if not os.path.isfile(path):
            missing.add(key)
            file_records[key] = {"path": os.path.basename(path), "sha256": None}
            continue
        file_records[key] = {"path": os.path.basename(path), "sha256": sha256(path)}

    for key in require:
        if key not in fields and key not in file_records:
            missing.add(key)

    record = {
        "schema": SCHEMA,
        "complete": not missing,
        "missing": sorted(missing),
        "fields": dict(sorted(fields.items())),
        "files": dict(sorted(file_records.items())),
    }
    if missing and not allow_missing:
        raise ValueError("missing required identity entries: " + ", ".join(sorted(missing)))
    return record


def write_record(record, output):
    os.makedirs(os.path.dirname(os.path.abspath(output)), exist_ok=True)
    with open(output, "w", encoding="utf-8") as fh:
        json.dump(record, fh, indent=2, sort_keys=True)
        fh.write("\n")


def check_record(path):
    with open(path, encoding="utf-8") as fh:
        record = json.load(fh)
    if record.get("schema") != SCHEMA:
        raise ValueError(f"{path}: schema is not {SCHEMA}")
    for key, item in (record.get("files") or {}).items():
        digest = item.get("sha256")
        if digest is not None and not HEX.match(digest):
            raise ValueError(f"{path}: files.{key}.sha256 is not a sha256")
    if not record.get("complete"):
        raise ValueError(f"{path}: identity is incomplete: {', '.join(record.get('missing') or [])}")
    return record


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output")
    parser.add_argument("--set", action="append", default=[], metavar="KEY=VALUE")
    parser.add_argument("--file", action="append", default=[], metavar="KEY=PATH")
    parser.add_argument("--require", action="append", default=[], metavar="KEY")
    parser.add_argument("--allow-missing", action="store_true")
    parser.add_argument("--check", metavar="FILE")
    args = parser.parse_args(argv)

    try:
        if args.check:
            check_record(args.check)
            print(f"identity complete: {args.check}")
            return 0
        if not args.output:
            parser.error("--output is required unless --check is used")
        record = build_identity(args.set, args.file, args.require, args.allow_missing)
        write_record(record, args.output)
        if record["complete"]:
            print(f"identity complete: {args.output}")
        else:
            print(
                f"identity incomplete ({', '.join(record['missing'])}): {args.output}",
                file=sys.stderr,
            )
        return 0
    except (OSError, ValueError) as exc:
        print(f"identity.py: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
