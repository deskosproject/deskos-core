#!/usr/bin/env python3
"""Drive the lifecycle A -> B -> A on an installed DeskOS disk.

This is the driver behind docs/design/lifecycle.md. It runs on a Linux host
with rootful podman, qemu, qemu-nbd, skopeo and /dev/kvm, and it is not part of
CI yet: it installs a disk, boots it, upgrades it from a local registry and rolls
it back. The pure parts (qemu arguments, status parsing, block building) are
unit-tested; the run itself is the evidence. `--boot-failure` adds a candidate
that cannot boot and recovers the previous GRUB entry from the host.

The test image is a fixture, never a resource: it carries a throwaway SSH key
for root, enables sshd and marks 10.0.2.2:5000 as an insecure registry. B adds
one marker file, so the two deployments differ without adding a package.

Exit codes: 0 the sequence held, 1 a check failed, 2 the harness could not run.
"""
from __future__ import annotations

import argparse
import json
import os
import shlex
import shutil
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import lifecycle  # noqa: E402

GUEST_REGISTRY = "10.0.2.2:5000"
LOCAL_REGISTRY = "127.0.0.1:5000"
REPO = "deskos/lifecycle"
TAG = "test"
MARKER_DIR = "/var/lib/deskos-lifecycle"
MARKER_FILE = f"{MARKER_DIR}/marker"
B_MARKER = "/usr/share/deskos/lifecycle-b"
B_ETC = "/etc/deskos-lifecycle-image"
LOCAL_ETC = "/etc/deskos-lifecycle-local"
GRUBENV = "boot/grub2/grubenv"

SHORT_SEQUENCE = lifecycle.SEQUENCE[:5]


class HarnessError(Exception):
    """The harness could not run (missing tool, no KVM, install failed)."""


def which(*candidates):
    for name in candidates:
        if not name:
            continue
        if Path(name).is_absolute() and Path(name).exists():
            return name
        found = shutil.which(name)
        if found:
            return found
    return None


def qemu_binary(explicit=None):
    found = which(explicit, "/usr/libexec/qemu-kvm", "qemu-system-x86_64")
    if not found:
        raise HarnessError("no qemu binary; pass --qemu")
    return found


# EL keeps OVMF under edk2, Ubuntu under OVMF with 4M images. The same list as
# tests/vm/bootcheck.py, so the two harnesses find the same firmware.
OVMF_PAIRS = (
    ("/usr/share/edk2/ovmf/OVMF_CODE.fd", "/usr/share/edk2/ovmf/OVMF_VARS.fd"),
    ("/usr/share/OVMF/OVMF_CODE_4M.fd", "/usr/share/OVMF/OVMF_VARS_4M.fd"),
    ("/usr/share/OVMF/OVMF_CODE.fd", "/usr/share/OVMF/OVMF_VARS.fd"),
)


def ovmf_paths(code=None, variables=None):
    """Return the OVMF code and vars paths, or fail with a clear message."""
    if code or variables:
        if not (code and variables):
            raise HarnessError("pass both --ovmf-code and --ovmf-vars")
        return code, variables
    for candidate_code, candidate_vars in OVMF_PAIRS:
        if Path(candidate_code).exists() and Path(candidate_vars).exists():
            return candidate_code, candidate_vars
    raise HarnessError("no OVMF firmware found; pass --ovmf-code and --ovmf-vars")


def qemu_argv(qemu, workdir, disk, ssh_port, ovmf_code, ovmf_vars):
    """The QEMU command line: UEFI, virtio, user networking with SSH forwarded."""
    workdir = Path(workdir)
    return [
        qemu,
        "-machine", "q35,accel=kvm",
        "-cpu", "host",
        "-m", "4096",
        "-smp", "4",
        "-drive", f"if=pflash,format=raw,readonly=on,file={ovmf_code}",
        "-drive", f"if=pflash,format=raw,file={workdir / 'OVMF_VARS.fd'}",
        "-drive", f"if=virtio,format=raw,file={disk}",
        "-device", "virtio-vga",
        "-display", "none",
        "-nic", f"user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:{ssh_port}-:22",
        "-device", "virtio-rng-pci",
        "-serial", f"file:{workdir / 'serial.log'}",
        "-pidfile", str(workdir / "qemu.pid"),
        "-daemonize",
    ]


def booted_digest(status):
    return status["status"]["booted"]["image"]["imageDigest"]


def staged_digest(status):
    staged = (status.get("status") or {}).get("staged") or {}
    return (staged.get("image") or {}).get("imageDigest")


def rollback_queued(status):
    return bool((status.get("status") or {}).get("rollbackQueued"))


def image_b_containerfile():
    """The image B Containerfile: both markers in one RUN instruction.

    A line beginning with `&&` is a new instruction, not a continuation, so the
    second `echo` has to stay on the `RUN` line.
    """
    return (
        "FROM localhost/deskos-lifecycle:a\n"
        f"RUN echo b > {B_MARKER} && echo b > {B_ETC}\n"
    )


def image_d_containerfile():
    """A candidate with one deliberate defect: the kernel cannot start init."""
    return (
        "FROM localhost/deskos-lifecycle:a\n"
        "RUN rm -f /usr/lib/systemd/systemd\n"
    )


def with_grub_default(text, value="1"):
    """Return `grubenv` with `default` set, keeping the block size.

    GRUB reads a fixed-size environment block. Only the value of an existing
    `default=` line is changed, so a caller cannot accidentally resize it.
    """
    lines = text.splitlines(keepends=True)
    for index, line in enumerate(lines):
        if line.startswith("default="):
            ending = "\n" if line.endswith("\n") else ""
            lines[index] = f"default={value}{ending}"
            result = "".join(lines)
            if len(result) != len(text):
                raise ValueError("grubenv block size would change")
            return result
    # A fresh grubenv has no default line: insert one before the # padding and
    # take the same number of bytes from the padding to keep the 1024-byte block.
    for index, line in enumerate(lines):
        if line.startswith("#") and not line.startswith("# GRUB") and not line.startswith("# WARNING"):
            addition = f"default={value}\n"
            padding = line.rstrip("\n")
            if len(padding) < len(addition):
                raise ValueError("grubenv has no room for default")
            lines[index] = padding[:-len(addition)] + ("\n" if line.endswith("\n") else "")
            lines.insert(index, addition)
            result = "".join(lines)
            if len(result) != len(text):
                raise ValueError("grubenv block size would change")
            return result
    raise ValueError("grubenv has no padding for default")


def block(boot, step, digest, staged, rollback, marker, etc, image_etc):
    """One report block, in the shape lifecycle.evaluate expects."""
    return {
        "boot": str(boot),
        "step": step,
        "image": digest,
        "staged": "yes" if staged else "no",
        "rollback": "yes" if rollback else "no",
        "marker": marker or "-",
        "etc": etc or "-",
        "image_etc": "yes" if image_etc else "no",
    }


def parse_status(raw):
    """Parse the JSON `bootc status` prints, tolerating a trailing banner."""
    start = raw.find("{")
    if start < 0:
        raise ValueError("not JSON")
    return json.loads(raw[start:])


class Driver:
    def __init__(self, args):
        self.args = args
        self.work = Path(args.workdir)
        self.key = self.work / "id_lifecycle"
        self.qemu = qemu_binary(args.qemu)
        self.ovmf_code, self.ovmf_vars = ovmf_paths(args.ovmf_code, args.ovmf_vars)
        self.ssh_port = args.ssh_port
        parts = shlex.split(args.podman)
        if not parts or parts[-1] != "podman":
            raise HarnessError("--podman must end in the podman binary")
        self.podman_cmd = parts
        self.sudo = parts[:-1]
        self.blocks = []
        self.extra_blocks = []
        self.boots = 0
        self.images = {}
        self.marker_a = "A1"
        self.marker_b = "A2"
        self.failure = []
        self.seq = lifecycle.SEQUENCE if args.interrupt else SHORT_SEQUENCE

    def sh(self, *cmd, **kw):
        kw.setdefault("check", True)
        return subprocess.run(cmd, **kw)

    def podman(self, *cmd, **kw):
        return self.sh(*self.podman_cmd, *cmd, **kw)

    def skopeo(self, *cmd, **kw):
        return self.sh(*self.sudo, "skopeo", *cmd, **kw)

    def say(self, text):
        print(f"\n=== {text}", flush=True)

    def ssh(self, command, check=True):
        argv = [
            "ssh", "-i", str(self.key), "-p", str(self.ssh_port),
            "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
            "-o", "ConnectTimeout=5", "-o", "LogLevel=ERROR",
            "root@127.0.0.1", command,
        ]
        return subprocess.run(argv, check=False, capture_output=True, text=True)

    def guest(self, command, check=True):
        result = self.ssh(command)
        if check and result.returncode != 0:
            raise HarnessError(f"guest command failed: {command}\n{result.stderr}")
        return result.stdout

    def wait_ssh(self, timeout=300):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.ssh("true").returncode == 0:
                return
            time.sleep(5)
        raise HarnessError(f"guest ssh did not come up within {timeout}s")

    def wait_no_ssh(self, timeout=60):
        """Wait long enough for a failed candidate to show it cannot log in."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.ssh("true").returncode == 0:
                raise HarnessError("the failed candidate reached ssh")
            time.sleep(5)

    def status(self):
        return parse_status(self.guest("bootc status --json"))

    def read_marker(self):
        raw = self.guest(f"cat {MARKER_FILE} 2>/dev/null || true").strip()
        return raw or None

    def write_marker(self, value):
        self.guest(f"mkdir -p {MARKER_DIR} && printf '%s' '{value}' > {MARKER_FILE}")

    def install(self):
        self.say("source image")
        # A digest ref is pulled; a local tag built by the caller is used as is.
        if self.podman("image", "exists", self.args.image, check=False).returncode != 0:
            self.podman("pull", "-q", f"docker://{self.args.image}")
        else:
            print(f"using the local image {self.args.image}")

        self.say("build the test images A and B")
        ctx = self.work / "ctx"
        ctx.mkdir(parents=True, exist_ok=True)
        shutil.copy(self.key.with_suffix(".pub"), ctx / "id_lifecycle.pub")
        (ctx / "Containerfile.a").write_text(
            f"FROM {self.args.image}\n"
            "COPY id_lifecycle.pub /root/.ssh/authorized_keys\n"
            "RUN mkdir -p /etc/containers/registries.conf.d \\\n"
            " && chmod 700 /root/.ssh && chmod 600 /root/.ssh/authorized_keys \\\n"
            f" && printf '[[registry]]\\nlocation = \"{GUEST_REGISTRY}\"\\ninsecure = true\\n'"
            " > /etc/containers/registries.conf.d/999-lifecycle.conf \\\n"
            " && printf 'PermitRootLogin prohibit-password\\n'"
            " > /etc/ssh/sshd_config.d/10-lifecycle.conf \\\n"
            " && systemctl enable sshd.service\n"
        )
        (ctx / "Containerfile.b").write_text(image_b_containerfile())
        (ctx / "Containerfile.c").write_text(
            "FROM localhost/deskos-lifecycle:a\n"
            "RUN echo c > /usr/share/deskos/lifecycle-c\n"
        )
        self.podman("build", "-q", "-t", "localhost/deskos-lifecycle:a",
                    "-f", str(ctx / "Containerfile.a"), str(ctx))
        self.podman("build", "-q", "-t", "localhost/deskos-lifecycle:b",
                    "-f", str(ctx / "Containerfile.b"), str(ctx))
        self.podman("build", "-q", "-t", "localhost/deskos-lifecycle:c",
                    "-f", str(ctx / "Containerfile.c"), str(ctx))

        self.say("start the registry and push A")
        self.podman("rm", "-f", "lifecycle-registry", check=False,
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.podman("run", "-d", "--name", "lifecycle-registry",
                    "-p", f"{LOCAL_REGISTRY}:5000", "docker.io/library/registry:2",
                    stdout=subprocess.DEVNULL)
        time.sleep(2)
        self.podman("push", "--tls-verify=false", "localhost/deskos-lifecycle:a",
                    f"docker://{LOCAL_REGISTRY}/{REPO}:{TAG}")

        self.say("install the disk")
        disk = self.work / "disk.raw"
        if disk.exists():
            self.sh(*self.sudo, "rm", "-f", str(disk))
        with open(disk, "wb") as fh:
            fh.truncate(self.args.disk_size_gb * 1024 ** 3)
        oci = self.work / "oci"
        if oci.exists():
            self.sh(*self.sudo, "rm", "-rf", str(oci))
        self.skopeo("copy", "containers-storage:localhost/deskos-lifecycle:a",
                    f"oci:{oci}:{TAG}")
        self.podman(
            "run", "--rm", "--privileged", "--pid=host",
            "--security-opt", "label=type:unconfined_t",
            "-v", "/dev:/dev", "-v", "/var/lib/containers:/var/lib/containers",
            "-v", f"{oci}:/src-oci:ro", "-v", f"{self.work}:/output",
            "localhost/deskos-lifecycle:a",
            "bootc", "install", "to-disk", "--via-loopback", "/output/disk.raw",
            "--source-imgref", "oci:/src-oci:test",
            "--target-imgref", f"{GUEST_REGISTRY}/{REPO}:{TAG}",
            "--composefs-backend", "--bootloader", "grub", "--wipe",
            "--filesystem", "ext4", "--allow-missing-verity",
        )

    def boot(self, expect_ssh=True):
        self.boots += 1
        self.say(f"boot {self.boots}")
        shutil.copy(self.ovmf_vars, self.work / "OVMF_VARS.fd")
        (self.work / "serial.log").write_text("")
        argv = qemu_argv(self.qemu, self.work, str(self.work / "disk.raw"),
                         self.ssh_port, self.ovmf_code, self.ovmf_vars)
        self.sh(*self.sudo, *argv)
        if expect_ssh:
            self.wait_ssh()
        else:
            self.wait_no_ssh()

    def read_local_etc(self):
        raw = self.guest(f"cat {LOCAL_ETC} 2>/dev/null || true").strip()
        return raw or None

    def has_image_etc(self):
        return self.ssh(f"test -f {B_ETC}").returncode == 0

    def record(self, step, write=None, collect=True):
        status = self.status()
        digest = booted_digest(status)
        marker = self.read_marker()
        local_etc = self.read_local_etc()
        image_etc = self.has_image_etc()
        if write:
            self.write_marker(write)
        entry = block(self.boots, step, digest,
                      bool(staged_digest(status)),
                      rollback_queued(status),
                      marker, local_etc, image_etc)
        (self.blocks if collect else self.extra_blocks).append(entry)
        self.save_journal(step)
        print(f"boot {self.boots} {step}: booted {digest[:19]}… "
              f"staged={'yes' if staged_digest(status) else 'no'} "
              f"rollback={'yes' if rollback_queued(status) else 'no'} "
              f"marker={marker} etc={local_etc} image_etc={'yes' if image_etc else 'no'}")
        return digest

    def save_journal(self, step):
        """Keep this boot's journal; the evaluator judges, a person reads."""
        try:
            journal = self.guest("journalctl -b --no-pager -o short")
        except HarnessError:
            return
        name = f"journal-boot-{self.boots}-{step}.log"
        (self.work / name).write_text(journal)

    def reboot(self):
        self.guest("systemctl reboot", check=False)
        self.boots += 1
        time.sleep(15)
        self.wait_ssh()

    def push_image(self, name, key):
        self.podman("push", "--tls-verify=false", f"localhost/deskos-lifecycle:{name}",
                    f"docker://{LOCAL_REGISTRY}/{REPO}:{TAG}")
        digest = self.skopeo(
            "inspect", "--tls-verify=false", "--format", "{{.Digest}}",
            f"docker://{LOCAL_REGISTRY}/{REPO}:{TAG}",
            capture_output=True, text=True).stdout.strip()
        self.images[key] = digest
        return digest

    def free_nbd(self):
        for number in range(16):
            device = f"/dev/nbd{number}"
            if not Path(device).exists():
                continue
            pid = Path(f"/sys/block/nbd{number}/pid")
            if not pid.exists() or not pid.read_text().strip():
                return device
        raise HarnessError("no free nbd device")

    def set_boot_default(self, value="1"):
        """Mount the installed root and set the GRUB default entry.

        GRUB is not driven through QMP: the two BLS entries have identical
        titles and a one-second timeout, so key injection is not deterministic.
        The accepted recovery path is host-side: mount the disk, set the
        previous entry as default, boot it.
        """
        self.sh(*self.sudo, "modprobe", "nbd", check=False)
        nbd = self.free_nbd()
        mount = self.work / "mnt"
        mount.mkdir(exist_ok=True)
        try:
            self.sh(*self.sudo, "qemu-nbd", "--format=raw", "-c", nbd,
                    str(self.work / "disk.raw"))
            time.sleep(2)
            root = None
            listing = subprocess.run(
                ["lsblk", "-ln", "-o", "NAME,TYPE"], check=True,
                capture_output=True, text=True).stdout
            for line in listing.splitlines():
                name, _, kind = line.partition(" ")
                if kind.strip() != "part" or not name.startswith(nbd.rsplit("/", 1)[1]):
                    continue
                path = f"/dev/{name}"
                fstype = subprocess.run(
                    [*self.sudo, "blkid", "-o", "value", "-s", "TYPE", path],
                    check=False, capture_output=True, text=True).stdout.strip()
                if fstype in ("ext4", "xfs"):
                    root = path
                    break
            if root is None:
                raise HarnessError("no ext4/xfs root partition on the disk")
            self.sh(*self.sudo, "mount", "-o", "rw", root, str(mount))
            try:
                grubenv = mount / GRUBENV
                raw = subprocess.run(
                    [*self.sudo, "cat", str(grubenv)], check=True,
                    capture_output=True, text=True).stdout
                updated = with_grub_default(raw, value)
                subprocess.run(
                    [*self.sudo, "tee", str(grubenv)], input=updated,
                    check=True, text=True, stdout=subprocess.DEVNULL)
            finally:
                self.sh(*self.sudo, "umount", str(mount), check=False)
        finally:
            self.sh(*self.sudo, "qemu-nbd", "-d", nbd, check=False)

    def failure_scenario(self):
        """Stage a candidate with no init, fail to boot it, recover the old entry."""
        self.say("build and stage a candidate that cannot boot")
        ctx = self.work / "ctx"
        (ctx / "Containerfile.d").write_text(image_d_containerfile())
        self.podman("build", "-q", "-t", "localhost/deskos-lifecycle:d",
                    "-f", str(ctx / "Containerfile.d"), str(ctx))
        self.push_image("d", "d")
        self.guest("bootc upgrade")
        self.record("stage-d", collect=False)

        self.say("boot the failed candidate")
        self.kill_qemu()
        try:
            self.set_boot_default("1")
        except (HarnessError, ValueError) as exc:
            self.failure.append(f"could not select the candidate boot entry: {exc}")
            return
        try:
            self.boot(expect_ssh=False)
        except HarnessError as exc:
            self.failure.append(str(exc))
        serial = (self.work / "serial.log").read_text()
        (self.work / "serial-failed.log").write_text(serial)
        if "Booting" not in serial:
            self.failure.append("the failed candidate did not reach a boot")
        self.kill_qemu()

        self.say("set the previous GRUB entry and boot it")
        try:
            self.set_boot_default("0")
        except (HarnessError, ValueError) as exc:
            self.failure.append(f"could not set the previous boot entry: {exc}")
            return
        self.boot()
        digest = self.record("recover-a", collect=False)
        if digest != self.images.get("a"):
            self.failure.append(
                f"recovery booted {digest}, expected A {self.images.get('a')}")
        if self.extra_blocks[-1]["marker"] != self.marker_b:
            self.failure.append("recovery lost the /var marker")

    def run(self):
        self.say("preflight")
        tools = ["skopeo", "qemu-img"]
        if self.args.boot_failure:
            tools.append("qemu-nbd")
        for tool in tools:
            if not which(tool):
                raise HarnessError(f"missing {tool}")
        if not Path("/dev/kvm").exists():
            raise HarnessError("no /dev/kvm")
        self.work.mkdir(parents=True, exist_ok=True)
        os.chmod(self.work, 0o700)
        if shutil.disk_usage(self.work).free < (self.args.disk_size_gb + 6) * 1024 ** 3:
            raise HarnessError("not enough free space for the disk and images")
        if not self.key.exists():
            self.sh("ssh-keygen", "-t", "ed25519", "-f", str(self.key), "-N", "", "-C", "lifecycle")

        self.install()

        self.say("boot A, write the /var marker and edit /etc")
        self.boot()
        self.guest(f"printf local > {LOCAL_ETC}")
        self.images["a"] = self.record("install", write=self.marker_a)

        self.say("push B and stage it")
        self.push_image("b", "b")
        self.guest("bootc upgrade")
        self.record("stage")
        self.reboot()

        self.say("boot B")
        self.record("boot-b")

        self.say("roll back to A")
        self.guest("bootc rollback")
        self.record("rollback")
        self.reboot()
        self.record("boot-a", write=self.marker_b)

        if self.args.interrupt:
            # A third image: after the rollback, B is the rollback deployment,
            # and upgrading to the rollback image is not a real update.
            self.say("interrupt an upgrade with a power loss")
            self.push_image("c", "c")
            self.guest("setsid nohup bootc upgrade >/dev/null 2>&1 < /dev/null & true")
            time.sleep(3)
            self.kill_qemu()
            self.boot()
            self.record("interrupt", write=self.marker_b)
            self.guest("bootc upgrade")
            self.record("retry")

        if self.args.boot_failure:
            self.failure_scenario()

        failures = lifecycle.evaluate(self.blocks, seq=self.seq, images=self.images)
        failures += self.failure
        self.write_report(failures)
        return failures

    def kill_qemu(self):
        raw = subprocess.run([*self.sudo, "cat", str(self.work / "qemu.pid")],
                             check=True, capture_output=True, text=True).stdout
        self.sh(*self.sudo, "kill", "-9", raw.strip())
        time.sleep(2)

    def write_report(self, failures):
        report = {
            "sequence": [boot.step for boot in self.seq],
            "blocks": self.blocks,
            "extra_blocks": self.extra_blocks,
            "boot_failure": self.failure,
            "images": self.images,
            "failures": failures,
        }
        (self.work / "lifecycle.json").write_text(json.dumps(report, indent=2) + "\n")
        lines = ["# Lifecycle", "", f"Source: `{self.args.image}`", ""]
        for entry in self.blocks + self.extra_blocks:
            lines.append(f"- boot {entry['boot']} {entry['step']}: `{entry['image']}` "
                         f"staged={entry['staged']} rollback={entry['rollback']} "
                         f"marker={entry['marker']} etc={entry['etc']} "
                         f"image_etc={entry['image_etc']}")
        lines += ["", "**PASS**" if not failures else "**FAIL**", ""]
        lines += [f"- {failure}" for failure in failures]
        (self.work / "summary.md").write_text("\n".join(lines) + "\n")
        if failures:
            for failure in failures:
                print(f"FAIL: {failure}", file=sys.stderr)
        else:
            print(f"ok: {len(self.blocks)} steps, A -> B -> A held")

    def cleanup(self):
        pid_file = self.work / "qemu.pid"
        if pid_file.exists():
            try:
                self.kill_qemu()
            except subprocess.CalledProcessError:
                pass
        self.podman("rm", "-f", "lifecycle-registry", check=False,
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--image", required=True,
                        help="source image reference, pinned by digest")
    parser.add_argument("--workdir", default="/var/tmp/lifecycle")
    parser.add_argument("--disk-size-gb", type=int, default=16)
    parser.add_argument("--ssh-port", type=int, default=2222)
    parser.add_argument("--podman", default="sudo podman",
                        help="podman command, default 'sudo podman'")
    parser.add_argument("--qemu", help="qemu binary; detected when omitted")
    parser.add_argument("--ovmf-code", help="OVMF code firmware; detected when omitted")
    parser.add_argument("--ovmf-vars", help="OVMF vars template; detected when omitted")
    parser.add_argument("--interrupt", action="store_true",
                        help="also interrupt an upgrade with a power loss")
    parser.add_argument("--boot-failure", action="store_true",
                        help="also stage an image with no init and recover the previous entry")
    parser.add_argument("--keep", action="store_true",
                        help="keep the disk, images and registry when done")
    args = parser.parse_args(argv)

    driver = Driver(args)
    try:
        failures = driver.run()
    except HarnessError as exc:
        print(f"lifecycle: {exc}", file=sys.stderr)
        return 2
    finally:
        if not args.keep:
            driver.cleanup()
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
