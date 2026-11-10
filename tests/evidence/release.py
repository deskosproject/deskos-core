#!/usr/bin/env python3
"""Write and upload the release record for one promotion.

The approval record says who moved a digest and why. The release record is the
wider fact: which image, which digest, which evidence, which signature, which
installer and which channels. It is the document the release contract names
(``docs/design/release-contract.md``), stored beside the evidence under
``releases/<repository>/<channel>/<timestamp>-<hex>.json``.

It is built from the evidence index and the approval record, so it cannot claim
anything those two do not already hold.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import upload  # noqa: E402

SCHEMA = "https://deskos.org/supply-chain/release/v1"
DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")


def record_key(repository, channel, stamp, digest):
    if not upload.NAME.match(repository) or not upload.NAME.match(channel):
        raise ValueError("repository and channel must be lowercase object names")
    if not DIGEST.match(digest):
        raise ValueError("digest is not sha256:<64 hex>")
    return f"releases/{repository}/{channel}/{stamp}-{digest.removeprefix('sha256:')}.json"


def build(index, approval, iso="", signature_scheme="", signature_identity="",
          now=None):
    """Return the release record implied by the evidence index and approval."""
    digest = index.get("digest")
    if not DIGEST.match(digest or ""):
        raise ValueError("the evidence index has no digest")
    if approval.get("digest") != digest:
        raise ValueError("the approval record is for another digest")
    if not approval.get("repository"):
        raise ValueError("the approval record has no repository")
    if approval.get("to") != "stable":
        raise ValueError("the approval record is not for the stable channel")
    if not approval.get("reason"):
        raise ValueError("the approval record has no reason")
    index_sha256 = approval.get("evidence_index_sha256")
    if not re.fullmatch(r"[0-9a-f]{64}", index_sha256 or ""):
        raise ValueError("the approval record has no evidence index hash")
    return {
        "schema": SCHEMA,
        "repository": approval["repository"],
        "image": approval["image"],
        "digest": digest,
        "channel": {
            "from": approval.get("from"),
            "to": approval.get("to"),
            "previous": approval.get("previous"),
        },
        "iso": {"digest": iso} if iso else None,
        "signature": {
            "scheme": signature_scheme or "unknown",
            "identity": signature_identity or None,
        },
        "evidence": {
            "index_sha256": index_sha256,
            "objects": index.get("objects") or [],
        },
        "approval": approval,
        "time": (now or datetime.now(timezone.utc)).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }


def write(path, data):
    Path(path).write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")
    return path


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dir", required=True, type=Path,
                        help="directory holding the fetched evidence index")
    parser.add_argument("--approval", required=True, type=Path)
    parser.add_argument("--iso", default="",
                        help="installer digest for the image, when one was promoted")
    parser.add_argument("--signature-scheme", default="",
                        help="keyless or key-based")
    parser.add_argument("--signature-identity", default="")
    parser.add_argument("--stamp", required=True,
                        help="UTC timestamp for the key, YYYYMMDDTHHMMSSZ")
    parser.add_argument("--output", type=Path,
                        help="also keep the record here, for the workflow artifact")
    parser.add_argument("--private", action="store_true",
                        help="upload without the public-read ACL (a private organization's bucket)")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)

    endpoint = os.environ.get("EVIDENCE_S3_ENDPOINT", "")
    bucket = os.environ.get("EVIDENCE_S3_BUCKET", "")
    access_key = os.environ.get("EVIDENCE_S3_ACCESS_KEY_ID", "")
    secret_key = os.environ.get("EVIDENCE_S3_SECRET_ACCESS_KEY", "")
    region = os.environ.get("EVIDENCE_S3_REGION", "auto")

    try:
        index = json.loads((args.dir / "index.json").read_text())
        approval = json.loads(args.approval.read_text())
        key = record_key(approval["repository"], approval["to"], args.stamp,
                         approval["digest"])
        data = build(index, approval, args.iso, args.signature_scheme,
                     args.signature_identity)
    except (OSError, ValueError, KeyError) as exc:
        print(f"release: {exc}", file=sys.stderr)
        return 1

    with tempfile.TemporaryDirectory() as tmp:
        path = write(args.output or (Path(tmp) / "release.json"), data)
        print(json.dumps(data, indent=2, sort_keys=True))
        if args.dry_run:
            print(f"would upload to {key}")
            return 0
        if not endpoint or not bucket or not access_key or not secret_key:
            print("release: EVIDENCE_S3_* is incomplete", file=sys.stderr)
            return 1
        try:
            upload.upload(endpoint, bucket, key, path, access_key, secret_key,
                          region, not args.private)
        except (OSError, subprocess.CalledProcessError) as exc:
            print(f"release: {exc}", file=sys.stderr)
            return 1
    print(f"uploaded {key}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
