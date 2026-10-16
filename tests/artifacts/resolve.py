#!/usr/bin/env python3
"""Fetch the network artifacts a plan declares, and say what each probe proves.

`validate` and `plan` do not touch the network, which is what keeps compilation
deterministic and offline — and also why a declared artifact URL that does not resolve,
or a version bumped without its asset name and checksum, is invisible to them and only
fails at image build time. A downstream Renovate pull request did exactly that: it moved
a `BinaryArtifact`'s release tag, left the asset filename at the old version, kept the
old checksum, and **would have merged**, because nothing fetched the URL.

What it covers, and what it does not:

- `binaries[].url` and `rpmFiles[].url`: **required** to carry a sha256 (the model
  requires it), required to resolve (2xx), and the bytes must hash to it. This is the
  verified half.
- `rpmRepositories[].gpgKeys[].content`: required to parse as an armored public key.
  Offline, and the only thing about a repository's signing this can prove.
- `rpmRepositories[].baseURL`: probed for `repodata/repomd.xml`, the file a dnf
  repository serves. A 2xx is reported as **verified presence**; anything else — a URL
  carrying a dnf variable (which cannot be resolved literally), a non-2xx, a silent host
  — is reported as **inconclusive**, never as a pass. A repository base URL is not a
  browsable index, so a 4xx there means nothing either way, and the signature itself is
  verified by the build, not here.
- It does **not** check the platform's base image digest, nor Flatpak remotes, nor
  anything about an artifact beyond its URL and checksum.

Exit status: 0 pass, 1 a failure, 2 usage or parse error. Standard library only, like the
VM harness, so CI needs nothing installed.
"""

import argparse
import hashlib
import json
import sys
import time
import urllib.error
import urllib.request

UA = "deskosctl-artifact-check"
# A declared response larger than this is refused rather than downloaded: a plan is data,
# and a URL that serves a stream of unbounded size would otherwise hold a job open until
# its own timeout. Verifying a sha256 needs the whole artifact, so the bound is generous.
MAX_BYTES = 256 * 1024 * 1024
# Total time for one artifact, network operations included. `--timeout` bounds each
# socket operation; this bounds the artifact.
ARTIFACT_SECONDS = 120.0
REPODATA = "repodata/repomd.xml"


class Result:
    """What one probe proved, in the run's own vocabulary."""

    def __init__(self):
        self.lines, self.failures, self.gaps = [], [], []

    def verified(self, what, detail):
        self.lines.append(f"- verified {what}: {detail}")

    def inconclusive(self, what, detail):
        self.lines.append(f"- inconclusive {what}: {detail}")
        self.gaps.append(f"{what}: {detail}")

    def failed(self, what, detail):
        self.lines.append(f"- FAIL {what}: {detail}")
        self.failures.append(f"{what}: {detail}")


def fetch(url, timeout, digest=False, limit=MAX_BYTES, deadline=None):
    """Return (status, sha256 | None, error | None) for one GET.

    A status code is an answer, not a connection failure — the caller decides whether it
    is acceptable. The body is read only when a checksum has to be computed, and never
    beyond `limit` bytes or past `deadline`.
    """
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    h = hashlib.sha256()
    read = 0
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            status = r.status
            if digest:
                while True:
                    if deadline and time.monotonic() > deadline:
                        return status, None, "ran out of time reading the body"
                    chunk = r.read(65536)
                    if not chunk:
                        break
                    read += len(chunk)
                    if read > limit:
                        return status, None, f"body is larger than {limit} bytes"
                    h.update(chunk)
    except urllib.error.HTTPError as e:
        return e.code, None, None
    except urllib.error.URLError as e:
        return None, None, f"{type(e.reason).__name__}: {e.reason}"
    except (TimeoutError, OSError) as e:
        return None, None, f"{type(e).__name__}: {e}"
    return status, (h.hexdigest() if digest else None), None


def repodata_url(base):
    """The dnf repository file, if the base URL is resolvable literally.

    A URL carrying `$releasever` or `$basearch` cannot be: dnf substitutes those from the
    system it runs on, so there is nothing to fetch here and the probe says so.
    """
    if "$" in base:
        return None
    return base.rstrip("/") + "/" + REPODATA


def check_plan(path, timeout, r, artifact_seconds=ARTIFACT_SECONDS, limit=MAX_BYTES):
    try:
        with open(path) as f:
            plan = json.load(f)
    except (OSError, ValueError) as e:
        # A plan that cannot be read is not a pass and not "0 failures": it is the reason
        # the run cannot say anything.
        r.failed(path, f"cannot be read: {e}")
        return

    r.lines.append(f"## {path}")
    art = plan.get("artifact", {})

    for kind, key in (("binary", "binaries"), ("rpmFile", "rpmFiles")):
        for item in art.get(key) or []:
            name = item.get("artifact") or item.get("url") or "?"
            url, sha256 = item.get("url", ""), item.get("sha256", "")
            what = f"{kind} {name}"
            if not url:
                r.failed(what, "declares no url")
                continue
            if not sha256:
                # The model requires it (internal/providers/software/software.go), so a
                # plan without one is a bug in what produced it, not a URL to check.
                r.failed(what, f"declares no sha256 ({url})")
                continue
            status, got, err = fetch(url, timeout, digest=True, limit=limit,
                                     deadline=time.monotonic() + artifact_seconds)
            if err:
                r.failed(what, f"{err} ({url})")
            elif not (200 <= status < 300):
                r.failed(what, f"HTTP {status} ({url})")
            elif got != sha256:
                r.failed(what, f"sha256 {got} != declared {sha256}")
            else:
                r.verified(what, f"HTTP {status}, sha256 verified")

    for repo in art.get("rpmRepositories") or []:
        rid = repo.get("id", "?")
        for key in repo.get("gpgKeys") or []:
            content = key.get("content") or ""
            if "BEGIN PGP PUBLIC KEY BLOCK" not in content:
                r.failed(f"repo {rid} key {key.get('name')}", "is not an armored public key")
            else:
                r.verified(f"repo {rid} key {key.get('name')}", "armored public key")

        base = repo.get("baseURL", "")
        if not base:
            r.inconclusive(f"repo {rid}", "declares no baseURL")
            continue
        probe = repodata_url(base)
        if probe is None:
            r.inconclusive(f"repo {rid}", "the base URL carries a dnf variable, so "
                                          "repodata/repomd.xml cannot be resolved from a plan")
            continue
        status, _, err = fetch(probe, timeout, deadline=time.monotonic() + artifact_seconds)
        if err:
            r.inconclusive(f"repo {rid}", f"{err} ({probe})")
        elif 200 <= status < 300:
            r.verified(f"repo {rid}", f"repodata/repomd.xml answered HTTP {status}")
        else:
            r.inconclusive(f"repo {rid}", f"repodata/repomd.xml answered HTTP {status} ({probe})")


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("plans", nargs="+", help="plan.json files to check")
    p.add_argument("--timeout", type=float, default=30.0, help="seconds per network operation")
    p.add_argument("--artifact-seconds", type=float, default=ARTIFACT_SECONDS,
                   help="total seconds allowed per artifact")
    p.add_argument("--max-bytes", type=int, default=MAX_BYTES, help="largest body to download")
    p.add_argument("--summary", help="write the per-artifact lines here")
    a = p.parse_args()
    if a.artifact_seconds <= 0 or a.max_bytes <= 0:
        p.error("--artifact-seconds and --max-bytes must be positive")

    r = Result()
    for path in a.plans:
        check_plan(path, a.timeout, r, a.artifact_seconds, a.max_bytes)

    for line in r.lines:
        print(line)
    if a.summary:
        with open(a.summary, "w") as f:
            f.write("\n".join(r.lines) + "\n")
    for f in r.failures:
        print(f"resolve: {f}", file=sys.stderr)
    print(f"resolve: {'fail' if r.failures else 'pass'} ({len(a.plans)} plan(s), "
          f"{len(r.failures)} failure(s), {len(r.gaps)} inconclusive)")
    return 1 if r.failures else 0


if __name__ == "__main__":
    sys.exit(main())
