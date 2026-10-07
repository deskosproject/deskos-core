#!/usr/bin/env python3
"""Validate a deskos-core commit on an entitled RHEL factory host.

For one commit of the canonical repository it renders a RHEL workstation,
builds it with Podman (the Containerfile ends with bootc container lint),
scans every image layer for build-host subscription identity, builds a test
QCOW2 with bootc-image-builder and runs tests/vm/bootcheck.py and
tests/vm/sessioncheck.py from that commit. The image and the disk never
leave the host; only a commit status with a short description is published.

`poll` validates the head of the branch once and skips commits that already
have a status for the context. A commit whose build context and harness are
identical to an already validated commit reuses that result.

It runs as root on a registered RHEL host with registry.redhat.io
credentials, /dev/kvm, QEMU, OVMF, Podman, skopeo, Go and git. It uses only
the Python standard library.
"""

import argparse
import datetime
import glob
import hashlib
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tarfile
import urllib.error
import urllib.request

REPO = "deskosproject/deskos-core"
BRANCH = "main"
CONTEXT = "deskos/rhel10"
WORKSTATION = "example-devops-rhel10"
RESOURCE_ROOTS = ["./resources", "./examples/baseline-and-role"]
# rhel10/bootc-image-builder amd64 manifest; move it deliberately.
BIB = "registry.redhat.io/rhel10/bootc-image-builder@sha256:7f5baead2d4ac2a1035900ced31e4e7600fc98f69aa45ee5d05639bca028e00b"
KEEP_RUNS = 5

# Paths a layer must not carry: build-host repository file, entitlement and
# consumer certificates, and non-empty RHSM state or logs.
RHSM_PATHS = re.compile(
    r"^(\./)?(etc/yum\.repos\.d/redhat\.repo"
    r"|etc/pki/(entitlement|consumer)/.+"
    r"|var/(lib|log)/rhsm/.+)$")
MAX_SCAN_BYTES = 64 << 20


class FactoryError(Exception):
    pass


# ---------------------------------------------------------------- GitHub

def github(method, path, token=None, body=None):
    req = urllib.request.Request(
        "https://api.github.com" + path, method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Accept": "application/vnd.github+json",
                 "X-GitHub-Api-Version": "2022-11-28",
                 "User-Agent": "deskos-rhel-factory"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def head_sha(repo, branch, token):
    return github("GET", f"/repos/{repo}/commits/{branch}", token)["sha"]


def final_status(statuses, context):
    """Newest final state for context; a newest pending one is stale, since runs never overlap."""
    for s in statuses:
        if s.get("context") == context:
            return s["state"] if s["state"] != "pending" else None
    return None


def has_status(repo, sha, context, token):
    statuses = github("GET", f"/repos/{repo}/commits/{sha}/statuses?per_page=100", token)
    return final_status(statuses, context) is not None


def set_status(repo, sha, state, description, token):
    github("POST", f"/repos/{repo}/statuses/{sha}", token, {
        "state": state, "context": CONTEXT, "description": description[:140]})


def read_token():
    creds = os.environ.get("CREDENTIALS_DIRECTORY")
    if creds and os.path.exists(os.path.join(creds, "github-token")):
        with open(os.path.join(creds, "github-token")) as f:
            return f.read().strip()
    return None


# ---------------------------------------------------------------- helpers

def run(cmd, log, cwd=None, timeout=None):
    with open(log, "ab") as out:
        out.write(("\n$ " + " ".join(cmd) + "\n").encode())
        out.flush()
        proc = subprocess.run(cmd, cwd=cwd, stdout=out, stderr=subprocess.STDOUT, timeout=timeout)
    if proc.returncode != 0:
        raise FactoryError(f"{os.path.basename(cmd[0])} {cmd[1] if len(cmd) > 1 else ''} failed ({proc.returncode}); see {os.path.basename(log)}")


def output(cmd):
    return subprocess.run(cmd, check=True, capture_output=True, text=True).stdout


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def validation_key(src, ctx, tool=__file__):
    """Identity of what a validation exercised: build context, harness and this tool."""
    h = hashlib.sha256()
    files = [("plan.json", os.path.join(ctx, "plan.json")),
             ("generated-manifest.json", os.path.join(ctx, "generated-manifest.json")),
             ("factory.py", tool)]
    files += [(os.path.relpath(p, src), p) for p in sorted(glob.glob(os.path.join(src, "tests/vm/*.py")))]
    for name, path in files:
        h.update(f"{name} {sha256_file(path)}\n".encode())
    h.update(f"bib {BIB}\n".encode())
    return h.hexdigest()


# ---------------------------------------------------------------- layer scan

def host_markers():
    """Strings that identify this build host's subscription."""
    markers = [socket.gethostname().split(".")[0]]
    for pem in glob.glob("/etc/pki/entitlement/*.pem"):
        if not pem.endswith("-key.pem"):
            markers.append(os.path.basename(pem)[:-len(".pem")])
    ident = subprocess.run(["subscription-manager", "identity"], capture_output=True, text=True).stdout
    m = re.search(r"system identity:\s*(\S+)", ident)
    if m:
        markers.append(m.group(1))
    return [x for x in markers if x]


def scan_layers(layout, markers):
    """Return (layer count, findings) for an OCI image layout directory."""
    with open(os.path.join(layout, "index.json")) as f:
        manifest_digest = json.load(f)["manifests"][0]["digest"]
    with open(os.path.join(layout, "blobs", *manifest_digest.split(":"))) as f:
        layers = json.load(f)["layers"]
    needles = [m.encode() for m in markers]
    findings = []
    for n, layer in enumerate(layers, 1):
        with tarfile.open(os.path.join(layout, "blobs", *layer["digest"].split(":")), "r:*") as tf:
            for member in tf:
                if RHSM_PATHS.match(member.name) and (member.size > 0 or member.name.endswith("redhat.repo")):
                    findings.append(f"layer {n}: {member.name}")
                if member.isfile() and needles and member.size <= MAX_SCAN_BYTES:
                    data = tf.extractfile(member).read()
                    findings += [f"layer {n}: {member.name} contains {nd.decode()}" for nd in needles if nd in data]
    return len(layers), findings


# ---------------------------------------------------------------- validation

def validate(sha, work, repo):
    """Validate one commit. Returns (ok, description, run directory)."""
    run_dir = os.path.join(work, "runs", sha)
    shutil.rmtree(run_dir, ignore_errors=True)
    os.makedirs(run_dir)
    log = os.path.join(run_dir, "factory.log")
    src, ctx, out = (os.path.join(run_dir, d) for d in ("src", "ctx", "output"))
    image = f"localhost/{WORKSTATION}:{sha[:12]}"
    summary = {"commit": sha, "workstation": WORKSTATION, "bib": BIB,
               "started": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")}
    try:
        run(["git", "init", "-q", src], log)
        run(["git", "-C", src, "fetch", "-q", "--depth=1", f"https://github.com/{repo}.git", sha], log, timeout=600)
        run(["git", "-C", src, "checkout", "-q", "FETCH_HEAD"], log)
        env_go = ["env", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly"]
        run(env_go + ["go", "build", "-o", "bin/deskosctl", "./cmd/deskosctl"], log, cwd=src, timeout=900)
        run(["bin/deskosctl", "render", *RESOURCE_ROOTS, "--workstation", WORKSTATION, "--output", ctx], log, cwd=src)
        key = validation_key(src, ctx)
        summary.update(key=key, plan_sha256=sha256_file(os.path.join(ctx, "plan.json")))
        previous = find_validated(work, key, sha)
        if previous:
            summary.update(result="pass", reused=previous)
            return True, f"same build context and harness as {previous[:7]}, which passed", run_dir

        run(["podman", "build", "--pull=missing", "-t", image, ctx], log, timeout=5400)
        summary["image"] = output(["podman", "image", "inspect", image, "--format", "{{.Id}}"]).strip()

        layout = os.path.join(run_dir, "oci")
        run(["podman", "save", "--format", "oci-dir", "-o", layout, image], log, timeout=1800)
        count, findings = scan_layers(layout, host_markers())
        shutil.rmtree(layout)
        summary.update(layers=count, layer_findings=findings)
        if findings:
            raise FactoryError(f"{len(findings)} build-host identity findings in {count} layers")

        os.makedirs(out)
        run(["podman", "run", "--rm", "--privileged", "--pull=missing",
             "--security-opt", "label=type:unconfined_t",
             "-v", f"{out}:/output", "-v", "/var/lib/containers/storage:/var/lib/containers/storage",
             BIB, "build", "--type", "qcow2", "--no-default-kernel-args", image], log, timeout=3600)
        disk = os.path.join(out, "qcow2", "disk.qcow2")
        summary["qcow2_sha256"] = sha256_file(disk)

        run([sys.executable, "tests/vm/bootcheck.py", "--disk", disk, "--out", os.path.join(run_dir, "bootcheck"),
             "--expect-splash", "--boot-timeout", "420", "--shutdown-timeout", "120"], log, cwd=src, timeout=1200)
        run([sys.executable, "tests/vm/sessioncheck.py", "--mode", "session", "--disk", disk,
             "--plan", os.path.join(ctx, "plan.json"), "--out", os.path.join(run_dir, "session"),
             "--boot-timeout", "900"], log, cwd=src, timeout=1800)
        summary["result"] = "pass"
        return True, f"lint, {count} layers clean, boot and session checks passed (image {summary['image'][:12]})", run_dir
    except (FactoryError, subprocess.TimeoutExpired, OSError) as e:
        summary.update(result="fail", error=str(e))
        return False, str(e), run_dir
    finally:
        summary["finished"] = datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds")
        with open(os.path.join(run_dir, "summary.json"), "w") as f:
            json.dump(summary, f, indent=2, sort_keys=True)
            f.write("\n")
        shutil.rmtree(out, ignore_errors=True)
        subprocess.run(["podman", "rmi", "--ignore", image], capture_output=True)
        # Older dangling images only, so recent build layers stay cached.
        subprocess.run(["podman", "image", "prune", "-f", "--filter", "until=168h"], capture_output=True)


def find_validated(work, key, sha):
    """Commit of a passed, non-reused run with the same validation key."""
    for path in sorted(glob.glob(os.path.join(work, "runs", "*", "summary.json"))):
        with open(path) as f:
            s = json.load(f)
        if s.get("key") == key and s.get("result") == "pass" and not s.get("reused") and s.get("commit") != sha:
            return s["commit"]
    return None


def prune_runs(work, keep=KEEP_RUNS):
    """Keep the newest runs; drop bulky sources of all but the newest."""
    runs = sorted(glob.glob(os.path.join(work, "runs", "*", "summary.json")), key=os.path.getmtime)
    for path in runs[:-keep]:
        shutil.rmtree(os.path.dirname(path), ignore_errors=True)
    for path in runs[-keep:-1]:
        shutil.rmtree(os.path.join(os.path.dirname(path), "src"), ignore_errors=True)


# ---------------------------------------------------------------- main

def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("command", choices=["poll", "check"])
    ap.add_argument("--sha", help="commit to check (check only; default: branch head)")
    ap.add_argument("--repo", default=REPO)
    ap.add_argument("--branch", default=BRANCH)
    ap.add_argument("--work", default="/var/lib/deskos-factory")
    ap.add_argument("--publish", action="store_true", help="post the commit status (needs the github-token credential)")
    args = ap.parse_args(argv)

    if args.sha and not re.fullmatch(r"[0-9a-f]{40}", args.sha):
        print("factory: --sha must be a full 40-character commit id", file=sys.stderr)
        return 2
    token = read_token()
    if args.publish and not token:
        print("factory: --publish needs the github-token credential", file=sys.stderr)
        return 2
    try:
        sha = args.sha or head_sha(args.repo, args.branch, token)
        if args.command == "poll" and has_status(args.repo, sha, CONTEXT, token):
            print(f"factory: {sha[:7]} already has a {CONTEXT} status")
            return 0
        if args.publish:
            set_status(args.repo, sha, "pending", "building and booting on the RHEL factory", token)
    except (urllib.error.URLError, KeyError) as e:
        print(f"factory: GitHub: {e}", file=sys.stderr)
        return 2

    try:
        ok, description, run_dir = validate(sha, args.work, args.repo)
    except Exception as e:
        ok, description, run_dir = False, f"factory error: {e}", os.path.join(args.work, "runs", sha)
    print(f"factory: {sha[:7]} {'pass' if ok else 'fail'}: {description} ({run_dir})")
    if args.publish:
        try:
            set_status(args.repo, sha, "success" if ok else "failure", description, token)
        except urllib.error.URLError as e:
            print(f"factory: GitHub: {e}", file=sys.stderr)
            return 2
    prune_runs(args.work)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
