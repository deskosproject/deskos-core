#!/usr/bin/env python3
"""Delete one digest's evidence prefix from an S3-compatible bucket.

The store keeps the evidence of every published digest, so this is not routine
cleanup: it exists to remove a prefix that never became valid, such as an upload
that failed before it wrote `index.json`. A prefix that already has an index is
refused unless `--force` is given, and the command is a dry run until `--apply`
is passed. `--confirm` must repeat the exact prefix, so a typo cannot delete the
wrong one. It never prints the credential.
"""
from __future__ import annotations

import argparse
import os
import subprocess
import sys
import xml.etree.ElementTree as ET
from urllib.parse import quote

from upload import object_key

XMLNS = "{http://s3.amazonaws.com/doc/2006-03-01/}"


def prefix(namespace, repository, digest):
    """Return `<namespace>/<repository>/sha256/<hex>`, validated as for an upload."""
    return object_key(namespace, repository, digest, "index.json").removesuffix(
        "/index.json")


def _signed(endpoint, bucket, access_key, secret_key, region, *args):
    return [
        "curl",
        "--fail",
        "--silent",
        "--show-error",
        "--retry", "3",
        "--aws-sigv4", f"aws:amz:{region}:s3",
        "--user", f"{access_key}:{secret_key}",
        *args,
    ]


def _url(endpoint, bucket, key=""):
    base = f"{endpoint.rstrip('/')}/{quote(bucket)}"
    return f"{base}/{quote(key)}" if key else base


def _curl(argv):
    proc = subprocess.run(argv, check=True, capture_output=True, text=True)
    return proc.stdout


def list_keys(endpoint, bucket, prefix_key, access_key, secret_key, region):
    """List every key under a prefix, following continuation tokens."""
    keys = []
    token = None
    while True:
        params = [
            "--get",
            "--data-urlencode", "list-type=2",
            "--data-urlencode", f"prefix={prefix_key}",
        ]
        if token:
            params += ["--data-urlencode", f"continuation-token={token}"]
        argv = _signed(endpoint, bucket, access_key, secret_key, region,
                       *params, _url(endpoint, bucket))
        root = ET.fromstring(_curl(argv))
        keys += [element.text or "" for element in
                 root.findall(f"{XMLNS}Contents/{XMLNS}Key")]
        truncated = (root.findtext(f"{XMLNS}IsTruncated") or "").lower() == "true"
        token = root.findtext(f"{XMLNS}NextContinuationToken")
        if not truncated or not token:
            return keys


def delete_key(endpoint, bucket, key, access_key, secret_key, region):
    argv = _signed(endpoint, bucket, access_key, secret_key, region,
                   "--request", "DELETE", _url(endpoint, bucket, key))
    _curl(argv)


def _credential():
    endpoint = os.environ.get("EVIDENCE_S3_ENDPOINT", "")
    bucket = os.environ.get("EVIDENCE_S3_BUCKET", "")
    access_key = os.environ.get("EVIDENCE_S3_ACCESS_KEY_ID", "")
    secret_key = os.environ.get("EVIDENCE_S3_SECRET_ACCESS_KEY", "")
    region = os.environ.get("EVIDENCE_S3_REGION", "auto")
    if not endpoint or not bucket or not access_key or not secret_key:
        return None
    return endpoint, bucket, access_key, secret_key, region


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--confirm", required=True,
                        help="repeat the exact prefix this command will delete")
    parser.add_argument("--apply", action="store_true",
                        help="delete; without it the command only lists")
    parser.add_argument("--force", action="store_true",
                        help="allow deleting a prefix that has an index")
    parser.add_argument("--max-delete", type=int, default=1000)
    args = parser.parse_args(argv)

    credential = _credential()
    if credential is None:
        print("evidence prune: EVIDENCE_S3_* is incomplete", file=sys.stderr)
        return 1
    endpoint, bucket, access_key, secret_key, region = credential

    try:
        prefix_key = prefix(args.namespace, args.repository, args.digest)
    except ValueError as exc:
        print(f"evidence prune: {exc}", file=sys.stderr)
        return 2
    if args.confirm != prefix_key:
        print(f"evidence prune: --confirm must be exactly {prefix_key}",
              file=sys.stderr)
        return 2

    try:
        keys = list_keys(endpoint, bucket, prefix_key, access_key, secret_key, region)
    except (OSError, subprocess.CalledProcessError, ET.ParseError) as exc:
        print(f"evidence prune: {exc}", file=sys.stderr)
        return 1

    for key in keys:
        print(key)
    print(f"{len(keys)} object(s) under {prefix_key}")
    if not args.apply:
        print("dry run: pass --apply to delete")
        return 0

    index_key = f"{prefix_key}/index.json"
    if index_key in keys and not args.force:
        print(f"evidence prune: {prefix_key} has an index; a published digest "
              "keeps its evidence, so pass --force to delete it", file=sys.stderr)
        return 1
    if len(keys) > args.max_delete:
        print(f"evidence prune: {len(keys)} objects exceed --max-delete "
              f"{args.max_delete}", file=sys.stderr)
        return 1

    for key in keys:
        try:
            delete_key(endpoint, bucket, key, access_key, secret_key, region)
        except (OSError, subprocess.CalledProcessError) as exc:
            print(f"evidence prune: {exc}", file=sys.stderr)
            return 1

    try:
        remaining = list_keys(endpoint, bucket, prefix_key,
                              access_key, secret_key, region)
    except (OSError, subprocess.CalledProcessError, ET.ParseError) as exc:
        print(f"evidence prune: {exc}", file=sys.stderr)
        return 1
    if remaining:
        print(f"evidence prune: {len(remaining)} object(s) remain", file=sys.stderr)
        return 1
    print(f"deleted {len(keys)} object(s); {prefix_key} is empty")
    return 0


if __name__ == "__main__":
    sys.exit(main())
