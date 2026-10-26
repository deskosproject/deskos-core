#!/usr/bin/env python3
"""Write and upload the record of one promotion.

A promotion is a move, not a build: the digest stays the same, and the record is
the source of truth for who moved it, from where to where, why, and which
evidence was checked first. The record is stored beside the evidence under
`approvals/<repository>/<channel>/<timestamp>-<hex>.json`, so it never rewrites
a digest's immutable index.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import upload  # noqa: E402

SCHEMA = "https://deskos.org/supply-chain/promotion/v1"


def record_key(repository, channel, stamp, digest):
    if not upload.NAME.match(repository) or not upload.NAME.match(channel):
        raise ValueError("repository and channel must be lowercase object names")
    if not upload.DIGEST.match(digest):
        raise ValueError("digest is not sha256:<64 hex>")
    return f"approvals/{repository}/{channel}/{stamp}-{digest.removeprefix('sha256:')}.json"


def build(repository, image, digest, source, target, previous, actor, approver,
          reason, evidence_index_sha256, run_url, now=None):
    """The approval record, with stable key order for comparison and review."""
    if not reason.strip():
        raise ValueError("a promotion needs a reason")
    return {
        "schema": SCHEMA,
        "repository": repository,
        "image": image,
        "digest": digest,
        "from": source,
        "to": target,
        "previous": previous or None,
        "actor": actor,
        "approver": approver or actor,
        "reason": reason.strip(),
        "evidence_index_sha256": evidence_index_sha256,
        "run": run_url,
        "time": (now or datetime.now(timezone.utc)).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }


def write(path, data):
    Path(path).write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")
    return path


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", required=True, type=Path,
                        help="directory holding the fetched evidence index")
    parser.add_argument("--repository", required=True)
    parser.add_argument("--image", required=True)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--from", dest="source", required=True)
    parser.add_argument("--to", dest="target", required=True)
    parser.add_argument("--previous", default="")
    parser.add_argument("--actor", required=True)
    parser.add_argument("--approver", default="")
    parser.add_argument("--reason", required=True)
    parser.add_argument("--run-url", required=True)
    parser.add_argument("--stamp", required=True,
                        help="UTC timestamp for the key, YYYYMMDDTHHMMSSZ")
    parser.add_argument("--output", type=Path,
                        help="also keep the record here, for the workflow artifact")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)

    endpoint = os.environ.get("EVIDENCE_S3_ENDPOINT", "")
    bucket = os.environ.get("EVIDENCE_S3_BUCKET", "")
    access_key = os.environ.get("EVIDENCE_S3_ACCESS_KEY_ID", "")
    secret_key = os.environ.get("EVIDENCE_S3_SECRET_ACCESS_KEY", "")
    region = os.environ.get("EVIDENCE_S3_REGION", "auto")
    if not endpoint or not bucket or not access_key or not secret_key:
        print("approval: EVIDENCE_S3_* is incomplete", file=sys.stderr)
        return 1

    index = args.dir / "index.json"
    try:
        digest = json.loads(index.read_text()).get("digest")
        evidence_sha = __import__("hashlib").sha256(index.read_bytes()).hexdigest()
        key = record_key(args.repository, args.target, args.stamp, args.digest)
        data = build(args.repository, args.image, args.digest, args.source, args.target,
                     args.previous, args.actor, args.approver, args.reason,
                     evidence_sha, args.run_url)
    except (OSError, ValueError) as exc:
        print(f"approval: {exc}", file=sys.stderr)
        return 1
    if digest != args.digest:
        print(f"approval: the index is for {digest}, not {args.digest}", file=sys.stderr)
        return 1

    with tempfile.TemporaryDirectory() as tmp:
        path = write(args.output or (Path(tmp) / "approval.json"), data)
        print(json.dumps(data, indent=2, sort_keys=True))
        if args.dry_run:
            print(f"would upload to {key}")
            return 0
        try:
            upload.upload(endpoint, bucket, key, path, access_key, secret_key,
                          region, True)
        except (OSError, subprocess.CalledProcessError) as exc:
            print(f"approval: {exc}", file=sys.stderr)
            return 1
    print(f"uploaded {key}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
