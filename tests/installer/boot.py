#!/usr/bin/env python3
"""Boot the test-only installer ISO in QEMU and record the screen.

The ISO is built from tests/installer/config.toml, whose kickstart answers
storage, locale and network, so the install runs without a person. This
harness installs into a throwaway qcow2, never a real disk, and captures
screendumps through QMP. Review the last screenshots: a complete install
reboots into GNOME Initial Setup.

It needs Linux with KVM (/dev/kvm), QEMU, qemu-img, OVMF firmware and
Python 3; it uses only the Python standard library.
"""
from __future__ import annotations

import argparse
import json
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path


def qmp_command(sock, command, arguments=None):
    payload = {"execute": command}
    if arguments:
        payload["arguments"] = arguments
    sock.sendall((json.dumps(payload) + "\n").encode())
    buffer = b""
    while b"\n" not in buffer:
        buffer += sock.recv(65536)
    return json.loads(buffer.split(b"\n", 1)[0])


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--iso", required=True)
    p.add_argument("--workdir", required=True)
    p.add_argument("--disk-gb", type=int, default=20)
    p.add_argument("--minutes", type=float, default=25)
    p.add_argument("--interval", type=float, default=20)
    p.add_argument("--qemu", default="/usr/libexec/qemu-kvm")
    p.add_argument("--ovmf-code", default="/usr/share/edk2/ovmf/OVMF_CODE.fd")
    p.add_argument("--ovmf-vars", default="/usr/share/edk2/ovmf/OVMF_VARS.fd")
    p.add_argument("--marker", default="",
                   help="text that must appear in the serial log (for example "
                        "\"Install finished\")")
    args = p.parse_args()

    work = Path(args.workdir)
    work.mkdir(parents=True, exist_ok=True)
    shots = work / "shots"
    shots.mkdir(exist_ok=True)
    disk = work / "disk.qcow2"
    if not disk.exists():
        subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk),
                        f"{args.disk_gb}G"],
                       check=True, stdout=subprocess.DEVNULL)
    shutil.copy(args.ovmf_vars, work / "OVMF_VARS.fd")
    (work / "serial.log").write_text("")

    argv = [
        args.qemu, "-machine", "q35,accel=kvm", "-cpu", "host",
        "-m", "4096", "-smp", "4",
        "-drive", f"if=pflash,format=raw,readonly=on,file={args.ovmf_code}",
        "-drive", f"if=pflash,format=raw,file={work / 'OVMF_VARS.fd'}",
        "-drive", f"if=virtio,format=qcow2,file={disk}",
        "-drive", f"file={args.iso},media=cdrom,readonly=on",
        # The CD installs once; after the kickstart's reboot, the disk boots.
        "-boot", "once=d,menu=on",
        "-device", "virtio-vga", "-display", "none",
        "-nic", "user,model=virtio-net-pci",
        "-device", "virtio-rng-pci",
        "-serial", f"file:{work / 'serial.log'}",
        "-qmp", f"unix:{work / 'qmp.sock'},server,nowait",
        "-pidfile", str(work / "qemu.pid"),
        "-daemonize",
    ]
    subprocess.run(argv, check=True)
    print(f"qemu up; screenshots every {args.interval}s for {args.minutes} min",
          flush=True)

    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    for _ in range(60):
        try:
            sock.connect(str(work / "qmp.sock"))
            break
        except OSError:
            time.sleep(1)
    else:
        sys.exit("qmp socket never came up")
    qmp_command(sock, "qmp_capabilities")

    deadline = time.time() + args.minutes * 60
    index = 0
    while time.time() < deadline:
        index += 1
        name = shots / f"shot-{index:02d}.ppm"
        try:
            qmp_command(sock, "screendump", {"filename": str(name)})
        except Exception as exc:  # noqa: BLE001 - report and keep going
            print(f"screendump failed: {exc}", file=sys.stderr)
        status = qmp_command(sock, "query-status").get("return", {}).get("status")
        print(f"{index:02d} {time.strftime('%H:%M:%S')} status={status} {name.name}",
              flush=True)
        time.sleep(args.interval)

    print("stopping qemu")
    try:
        qmp_command(sock, "quit")
    except Exception:  # noqa: BLE001
        pass
    sock.close()
    time.sleep(2)
    print(f"screenshots in {shots}")

    if args.marker:
        serial = (work / "serial.log").read_text(errors="replace")
        if args.marker not in serial:
            print(f"serial log does not contain {args.marker!r}; the install did "
                  "not finish", file=sys.stderr)
            sys.exit(1)
        print(f"serial log contains {args.marker!r}")


if __name__ == "__main__":
    main()
