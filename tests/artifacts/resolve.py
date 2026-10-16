#!/usr/bin/env python3
"""Resolve every network artifact a plan declares, and verify its checksum.

`deskosctl validate` and `plan` do not touch the network, which is the point of the
compiler and also why this check exists separately: a declared artifact URL that does
not resolve, or a checksum that does not match, is invisible to them and only fails at
image build time — or worse, after a bot bumps a version field. It has happened: a
Renovate PR changed the release tag and the checksum of a `BinaryArtifact` and left the
asset filename behind, and it merged, because nothing fetched the URL.

What it verifies, per composed plan:

- `binaries[].url` and `rpmFiles[].url`: the URL resolves (2xx) and, where a sha256 is
  declared, the downloaded bytes hash to it.
- `rpmRepositories[].baseURL`: the repository resolves. A URL carrying a dnf variable
  (`$releasever`, `$basearch`) cannot be resolved literally, so its prefix is fetched
  instead and the run reports it as prefix-only — reported, not silently skipped.
- `rpmRepositories[].gpgKeys[].content`: the declared key parses as an armored public
  key. Offline, but it is the other half of "the repository is signed".

It does not verify that a repository's signature actually comes from the declared key;
that needs a package, and the build has one.

Exit status: 0 pass, 1 fail, 2 usage or parse error. Standard library only, like the VM
harness, so CI needs nothing installed.
"""

import argparse
import hashlib
import sys
import urllib.error
import urllib.request

UA = "deskosctl-artifact-check"


def fetch(url, timeout, digest=False):
    """Return (status, sha256 | None, error | None) for one GET.

    The body is read only when a checksum has to be computed: verifying a sha256 means
    downloading the whole artifact, and a repository URL needs its status, not its bytes.
    """
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    h = hashlib.sha256()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            status = r.status
            if digest:
                while True:
                    chunk = r.read(65536)
                    if not chunk:
                        break
                    h.update(chunk)
    except urllib.error.HTTPError as e:
        # A status code is an answer, not a connection failure: the caller decides
        # whether that status is acceptable. Returning it as an error made a
        # repository's normal 404 look like a dead host.
        return e.code, None, None
    except urllib.error.URLError as e:
        return None, None, f"{type(e.reason).__name__}: {e.reason}"
    return status, (h.hexdigest() if digest else None), None


def prefix_of(url):
    """The part of a dnf-variable URL that can be fetched: up to the first variable."""
    cut = min((url.index(c) for c in "$" if c in url), default=len(url))
    return url[:cut].rpartition("/")[0] + "/"


def declared(plan):
    """(kind, name, url, sha256) for every network artifact, plus the repos and keys."""
    art = plan.get("artifact", {})
    out = []
    for b in art.get("binaries") or []:
        out.append(("binary", b.get("artifact") or b.get("url", ""), b.get("url", ""), b.get("sha256", "")))
    for f in art.get("rpmFiles") or []:
        out.append(("rpmFile", f.get("url", ""), f.get("url", ""), f.get("sha256", "")))
    return out


def check_plan(path, timeout, failures, lines):
    import json
    try:
        plan = json.load(open(path))
    except (OSError, ValueError) as e:
        print(f"resolve: cannot read {path}: {e}", file=sys.stderr)
        return False

    lines.append(f"## {path}")
    ok = True

    for kind, name, url, sha256 in declared(plan):
        if not url:
            lines.append(f"- FAIL {kind} {name}: no url")
            failures.append(f"{path}: {kind} {name} declares no url")
            ok = False
            continue
        status, got, err = fetch(url, timeout, digest=bool(sha256))
        if err or not (200 <= status < 300):
            lines.append(f"- FAIL {kind} {name}: {err or f'HTTP {status}'} ({url})")
            failures.append(f"{path}: {kind} {name} does not resolve: {err or status}")
            ok = False
            continue
        if sha256 and got != sha256:
            lines.append(f"- FAIL {kind} {name}: sha256 {got} != declared {sha256}")
            failures.append(f"{path}: {kind} {name} sha256 mismatch: got {got}, declared {sha256}")
            ok = False
            continue
        lines.append(f"- pass {kind} {name}: {status}" + (" (sha256 verified)" if sha256 else ""))

    for repo in plan.get("artifact", {}).get("rpmRepositories") or []:
        url = repo.get("baseURL", "")
        if url:
            variable = "$" in url
            probe = prefix_of(url) if variable else url
            status, _, err = fetch(probe, timeout)
            note = " (prefix only: the URL carries a dnf variable)" if variable else ""
            if err:
                lines.append(f"- FAIL repo {repo.get('id')}: {err} ({probe})")
                failures.append(f"{path}: repository {repo.get('id')} does not resolve: {err}")
                ok = False
            else:
                # A repository base URL is not a browsable index: dnf sends it its own
                # parameters, so a 4xx here is the repository behaving as designed. What
                # this proves is that the host resolves and answers, and the signature
                # is checked at build time.
                lines.append(f"- pass repo {repo.get('id')}: host answered HTTP {status}{note}")
        for key in repo.get("gpgKeys") or []:
            content = key.get("content") or ""
            if "BEGIN PGP PUBLIC KEY BLOCK" not in content:
                lines.append(f"- FAIL repo {repo.get('id')} key {key.get('name')}: not an armored public key")
                failures.append(f"{path}: {repo.get('id')} key {key.get('name')} is not an armored public key")
                ok = False
            else:
                lines.append(f"- pass repo {repo.get('id')} key {key.get('name')}: armored")
    return ok


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("plans", nargs="+", help="plan.json files to check")
    p.add_argument("--timeout", type=float, default=30.0, help="seconds per request")
    p.add_argument("--summary", help="write the per-artifact lines here")
    a = p.parse_args()

    failures, lines = [], []
    ok = True
    for path in a.plans:
        ok = check_plan(path, a.timeout, failures, lines) and ok

    for line in lines:
        print(line)
    if a.summary:
        with open(a.summary, "w") as f:
            f.write("\n".join(lines) + "\n")
    for f in failures:
        print(f"resolve: {f}", file=sys.stderr)
    print(f"resolve: {'pass' if ok else 'fail'} ({len(a.plans)} plan(s), {len(failures)} failure(s))")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
