#!/usr/bin/env python3
"""Upload one digest's evidence to an S3-compatible bucket.

The object key is `<namespace>/<repository>/sha256/<hex>/<name>`. The script
builds the index, verifies every local object against it, and then uploads the
objects and the index with curl's AWS SigV4 support. It never prints the
credential.
"""
from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import quote

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import index  # noqa: E402
import verify  # noqa: E402

DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
NAME = re.compile(r"^[a-z0-9][a-z0-9._-]*$")


def object_key(namespace, repository, digest, name):
    if not NAME.match(namespace) or not NAME.match(repository):
        raise ValueError("namespace and repository must be lowercase object names")
    if not DIGEST.match(digest):
        raise ValueError("digest is not sha256:<64 hex>")
    if name.startswith("/") or ".." in Path(name).parts:
        raise ValueError(f"refusing path {name!r}")
    return f"{namespace}/{repository}/sha256/{digest.removeprefix('sha256:')}/{name}"


def content_type(name):
    suffix = Path(name).suffix.lower()
    if suffix == ".json":
        return "application/json"
    if suffix in (".txt", ".md", ".yaml", ".yml"):
        return "text/plain; charset=utf-8"
    return "application/octet-stream"


def upload(endpoint, bucket, key, path, access_key, secret_key, region, public):
    url = f"{endpoint.rstrip('/')}/{quote(bucket)}/{quote(key)}"
    argv = [
        "curl",
        "--fail",
        "--silent",
        "--show-error",
        "--retry", "3",
        "--aws-sigv4", f"aws:amz:{region}:s3",
        "--user", f"{access_key}:{secret_key}",
        "--upload-file", str(path),
        "--header", f"Content-Type: {content_type(path.name)}",
    ]
    if public:
        argv += ["--header", "Cache-Control: public, max-age=31536000, immutable"]
    argv.append(url)
    subprocess.run(argv, check=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", required=True, type=Path)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--public", action="store_true",
                        help="send the immutable public cache header")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)

    endpoint = os.environ.get("EVIDENCE_S3_ENDPOINT", "")
    bucket = os.environ.get("EVIDENCE_S3_BUCKET", "")
    access_key = os.environ.get("EVIDENCE_S3_ACCESS_KEY_ID", "")
    secret_key = os.environ.get("EVIDENCE_S3_SECRET_ACCESS_KEY", "")
    region = os.environ.get("EVIDENCE_S3_REGION", "auto")
    if not endpoint or not bucket or not access_key or not secret_key:
        print("evidence upload: EVIDENCE_S3_* is incomplete", file=sys.stderr)
        return 1

    index_path = args.dir / "index.json"
    try:
        data = index.build(args.dir, args.digest, index_path)
        errors = verify.verify(index_path, args.dir)
    except (OSError, ValueError) as exc:
        print(f"evidence upload: {exc}", file=sys.stderr)
        return 1
    if errors:
        for error in errors:
            print(f"evidence upload: {error}", file=sys.stderr)
        return 1

    for obj in data["objects"]:
        key = object_key(args.namespace, args.repository, args.digest, obj["name"])
        if args.dry_run:
            print(f"would upload {args.dir / obj['name']} to {key}")
            continue
        try:
            upload(endpoint, bucket, key, args.dir / obj["name"],
                   access_key, secret_key, region, args.public)
        except (OSError, subprocess.CalledProcessError) as exc:
            print(f"evidence upload: {exc}", file=sys.stderr)
            return 1
    print(f"uploaded {len(data['objects'])} object(s) for {args.digest}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
