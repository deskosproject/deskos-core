#!/usr/bin/env python3
"""Fetch the network artifacts a plan declares, and say what each probe proves.

`validate` and `plan` do not touch the network, which is what keeps compilation
deterministic and offline — and also why a declared artifact URL that does not resolve,
or a version bumped without its asset name and checksum, is invisible to them and only
fails at image build time. A downstream Renovate pull request did exactly that: it moved
a `BinaryArtifact`'s release tag, left the asset filename at the old version and kept the
old checksum, and **would have merged**, because nothing fetched the URL.

What it covers, and what it does not:

- `binaries[].url` and `rpmFiles[].url`: **required** to carry a sha256 (the model
  requires it), required to resolve (2xx), and the bytes must hash to it. An artifact
  larger than `--max-bytes` is a **failure that says it was not verified**, with its
  Content-Length reported — not a claim that it is broken, which was never read, but the
  run cannot be green either: a gate that passes without verifying is theatre. The bound
  exists to stop an unbounded stream; raising it is a decision the size makes informed.
- `rpmRepositories[].gpgKeys[].content`: required to **look like** an armored public key
  — the armor lines, and a body with key-sized lines. That is a shape check, not a
  parse: proving it *is* a key means importing it, which needs GnuPG (the build has
  GnuPG and verifies the signature there). What this refuses is a field that cannot be a
  key at all, which is the failure worth catching in a resource.
- `rpmRepositories[].baseURL`: probed for `repodata/repomd.xml` and accepted as
  **verified presence** only when the body looks like that file (it begins with `<?xml`
  or `<repomd`), so a captive portal answering 200 with HTML is not mistaken for a
  repository. Anything else — a URL carrying a dnf variable, a non-2xx, a body that is
  not repomd, a silent host — is **inconclusive**, never a pass. A repository base URL
  is not a browsable index, so a 4xx there means nothing either way, and the signature
  itself is left to the build.

It does **not** check the platform's base image digest, nor Flatpak remotes, nor anything
about an artifact beyond its URL and checksum.

The per-artifact deadline bounds the socket operation, so an artifact whose peer stops
sending can overrun it by at most one socket timeout — a bound between operations, not a
wall-clock cap on the whole artifact.

Exit status: 0 pass, 1 a failure, 2 a plan that could not be read at all. Standard
library only, like the VM harness, so CI needs nothing installed.
"""

import argparse
import hashlib
import json
import sys
import time
import urllib.error
import urllib.request

UA = "deskosctl-artifact-check"
# A body larger than this is refused rather than downloaded: a plan is data, and a URL
# serving an unbounded stream would otherwise hold a job open until its own timeout.
MAX_BYTES = 1024 * 1024 * 1024
# Time allowed for one artifact, handed to the socket. See the module comment.
ARTIFACT_SECONDS = 120.0
REPODATA = "repodata/repomd.xml"
# PGP armor lines are 64 columns; a body with no line nearly that long is not a key.
ARMOR_MIN_LINE = 40


class Response:
    """One GET: the status is an answer, and an HTTPError is one too."""

    def __init__(self, status=None, digest=None, prefix=None, error=None,
                 length=None, too_large=False):
        self.status, self.digest, self.prefix, self.error = status, digest, prefix, error
        self.length, self.too_large = length, too_large

    @property
    def ok(self):
        return self.error is None and self.status is not None and 200 <= self.status < 300

    def reason(self):
        if self.error:
            return self.error
        return f"HTTP {self.status}"


def fetch(url, timeout, digest=False, limit=MAX_BYTES, deadline=None, prefix=False):
    """GET one URL, reading the body only when a digest or a prefix is wanted.

    `timeout` is the socket timeout; when a `deadline` is given, the smaller of the two
    is used, so the time left is handed to the socket instead of being checked only
    between reads.
    """
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    h, read, head = hashlib.sha256(), 0, b""
    if deadline is not None:
        timeout = max(0.1, min(timeout, deadline - time.monotonic()))
    # read1 returns what is available instead of blocking until 65536 bytes arrive, which
    # is what lets the deadline be checked between reads at all: with a plain read() a
    # peer dribbling a byte at a time never trips the socket timeout and never returns.
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            read_chunk = getattr(r, "read1", r.read)
            status = r.status
            length = r.headers.get("Content-Length")
            while digest or prefix:
                if deadline is not None and time.monotonic() > deadline:
                    return Response(status, None, None, "ran out of time reading the body")
                chunk = read_chunk(65536)
                if not chunk:
                    break
                read += len(chunk)
                if read > limit:
                    # Not downloading something is not evidence that it is broken: the
                    # caller decides, and the size is reported so the decision is informed.
                    return Response(status, length=length, too_large=True,
                                    error=f"larger than the {limit} byte bound")
                if prefix and len(head) < 64:
                    head += chunk[:64 - len(head)]
                    if not digest and len(head) >= 64:
                        break     # the shape needs 64 bytes; no reason to read the rest
                if digest:
                    h.update(chunk)
    except urllib.error.HTTPError as e:
        # The status is the answer. Close the wrapped response so its socket is not left
        # to the garbage collector — which warns about exactly that.
        e.close()
        return Response(e.code)
    except urllib.error.URLError as e:
        return Response(error=f"{type(e.reason).__name__}: {e.reason}")
    except (TimeoutError, OSError) as e:
        return Response(error=f"{type(e).__name__}: {e}")
    return Response(status, h.hexdigest() if digest else None, head if prefix else None)


def looks_armored(content):
    """A shape check: the armor lines, and a body with key-sized lines."""
    lines = [l.strip() for l in (content or "").strip().splitlines()]
    if len(lines) < 3 or lines[0] != "-----BEGIN PGP PUBLIC KEY BLOCK-----":
        return False
    if lines[-1] != "-----END PGP PUBLIC KEY BLOCK-----":
        return False
    return max((len(l) for l in lines[1:-1]), default=0) >= ARMOR_MIN_LINE


def looks_like_repomd(prefix):
    """repomd.xml is XML, so a body that begins as HTML is not mistaken for one."""
    head = (prefix or b"").lstrip()
    return head.startswith(b"<?xml") or head.startswith(b"<repomd")


def repodata_url(base):
    """The dnf repository file, if the base URL is resolvable literally.

    `$releasever` and `$basearch` are substituted by dnf from the system it runs on, so a
    URL carrying them has nothing to fetch from a plan, and the probe says so.
    """
    if "$" in base:
        return None
    return base.rstrip("/") + "/" + REPODATA


class Report:
    """What the probes proved, in the run's own vocabulary."""

    def __init__(self):
        self.lines, self.failures, self.gaps, self.unreadable = [], [], [], []

    def verified(self, what, detail):
        self.lines.append(f"- verified {what}: {detail}")

    def inconclusive(self, what, detail):
        self.lines.append(f"- inconclusive {what}: {detail}")
        self.gaps.append(f"{what}: {detail}")

    def failed(self, what, detail):
        self.lines.append(f"- FAIL {what}: {detail}")
        self.failures.append(f"{what}: {detail}")

    def cannot_read(self, path, detail):
        # Both a failure and its own class: the run cannot say anything about a plan it
        # could not read, so it exits 2 rather than reporting a verdict.
        self.failed(path, f"cannot be read: {detail}")
        self.unreadable.append(f"{path}: {detail}")


def check_plan(path, timeout, r, artifact_seconds=ARTIFACT_SECONDS, limit=MAX_BYTES):
    try:
        with open(path) as f:
            plan = json.load(f)
    except (OSError, ValueError) as e:
        r.cannot_read(path, str(e))
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
                # The model requires one, so a plan without it is a bug in what produced
                # it, not a URL to check.
                r.failed(what, f"declares no sha256 ({url})")
                continue
            got = fetch(url, timeout, digest=True, limit=limit,
                        deadline=time.monotonic() + artifact_seconds)
            if got.too_large:
                # Not a claim that the artifact is broken — it was never read — but the
                # run cannot be green either: a gate that passes without verifying is
                # theatre. The size is reported so raising the bound is a decision.
                size = f", Content-Length {got.length}" if got.length else ""
                r.failed(what, f"not verified: {got.error}{size} ({url}); "
                               f"raise --max-bytes to verify it")
            elif not got.ok:
                r.failed(what, f"{got.reason()} ({url})")
            elif got.digest != sha256:
                r.failed(what, f"sha256 {got.digest} != declared {sha256}")
            else:
                r.verified(what, f"HTTP {got.status}, sha256 verified")

    for repo in art.get("rpmRepositories") or []:
        rid = repo.get("id", "?")
        for k in repo.get("gpgKeys") or []:
            if looks_armored(k.get("content")):
                r.verified(f"repo {rid} key {k.get('name')}", "armored public key (shape)")
            else:
                r.failed(f"repo {rid} key {k.get('name')}", "is not an armored public key")

        base = repo.get("baseURL", "")
        if not base:
            r.inconclusive(f"repo {rid}", "declares no baseURL")
            continue
        probe = repodata_url(base)
        if probe is None:
            r.inconclusive(f"repo {rid}", "the base URL carries a dnf variable, so "
                                          f"{REPODATA} cannot be resolved from a plan")
            continue
        got = fetch(probe, timeout, prefix=True, deadline=time.monotonic() + artifact_seconds)
        if not got.ok:
            r.inconclusive(f"repo {rid}", f"{got.reason()} ({probe})")
        elif not looks_like_repomd(got.prefix):
            r.inconclusive(f"repo {rid}", f"{probe} answered HTTP {got.status} but its body "
                                          "does not look like repomd.xml")
        else:
            r.verified(f"repo {rid}", f"{probe} answered HTTP {got.status} and looks like XML")


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("plans", nargs="+", help="plan.json files to check")
    p.add_argument("--timeout", type=float, default=30.0, help="seconds per socket operation")
    p.add_argument("--artifact-seconds", type=float, default=ARTIFACT_SECONDS,
                   help="seconds allowed per artifact")
    p.add_argument("--max-bytes", type=int, default=MAX_BYTES, help="largest body to read")
    p.add_argument("--summary", help="write the per-artifact lines here")
    a = p.parse_args()
    if a.artifact_seconds <= 0 or a.max_bytes <= 0 or a.timeout <= 0:
        p.error("--timeout, --artifact-seconds and --max-bytes must be positive")

    r = Report()
    for path in a.plans:
        check_plan(path, a.timeout, r, a.artifact_seconds, a.max_bytes)

    for line in r.lines:
        print(line)
    if a.summary:
        with open(a.summary, "w") as f:
            f.write("\n".join(r.lines) + "\n")
    for f in r.failures:
        print(f"resolve: {f}", file=sys.stderr)
    verdict = "error" if r.unreadable else ("fail" if r.failures else "pass")
    print(f"resolve: {verdict} ({len(a.plans)} plan(s), {len(r.failures)} failure(s), "
          f"{len(r.gaps)} inconclusive)")
    if r.unreadable:
        return 2          # nothing could be said about at least one plan
    return 1 if r.failures else 0


if __name__ == "__main__":
    sys.exit(main())
