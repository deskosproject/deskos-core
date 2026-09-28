#!/usr/bin/env python3
"""Boot an unmodified DeskOS disk image in QEMU (UEFI) and record the screen.

The disk is never written: each run boots a throwaway qcow2 overlay. Frames
are captured through QMP, classified with simple pixel heuristics, and
summarized in summary.json and summary.md. The run fails when the machine
never shows a stable graphical screen, when the expected boot splash does not
appear, or when ACPI shutdown does not power the machine off in time.

The harness only classifies screen pixels (blank, text, splash, graphical).
It does not identify which graphical screen appears (GNOME Initial Setup,
GDM or a desktop) and does not test applications.

It needs Linux with KVM (/dev/kvm), QEMU, qemu-img, OVMF firmware and
Python 3; it uses only the Python standard library.
"""

import argparse
import hashlib
import json
import os
import shutil
import socket
import struct
import subprocess
import sys
import time
import zlib

QEMU_CANDIDATES = ["qemu-system-x86_64", "/usr/libexec/qemu-kvm"]
OVMF_CANDIDATES = [
    ("/usr/share/edk2/ovmf/OVMF_CODE.fd", "/usr/share/edk2/ovmf/OVMF_VARS.fd"),
    ("/usr/share/OVMF/OVMF_CODE_4M.fd", "/usr/share/OVMF/OVMF_VARS_4M.fd"),
    ("/usr/share/OVMF/OVMF_CODE.fd", "/usr/share/OVMF/OVMF_VARS.fd"),
]

# Pixel classes by luma: dark below DARK, lit from LIT (text, logos, spinner).
DARK, LIT = 24, 60


class HarnessError(Exception):
    pass


# ---------------------------------------------------------------- images

def read_ppm(path):
    """Return (width, height, rgb bytes) of a binary P6 PPM."""
    with open(path, "rb") as f:
        data = f.read()
    fields, pos = [], 0
    while len(fields) < 4:
        while data[pos:pos + 1].isspace():
            pos += 1
        if data[pos:pos + 1] == b"#":
            pos = data.index(b"\n", pos) + 1
            continue
        end = pos
        while not data[end:end + 1].isspace():
            end += 1
        fields.append(data[pos:end])
        pos = end
    if fields[0] != b"P6" or int(fields[3]) != 255:
        raise HarnessError(f"{path}: not an 8-bit P6 PPM")
    w, h = int(fields[1]), int(fields[2])
    pixels = data[pos + 1:pos + 1 + w * h * 3]
    if len(pixels) != w * h * 3:
        raise HarnessError(f"{path}: truncated PPM")
    return w, h, pixels


def write_png(path, w, h, rgb):
    """Write an RGB PNG (filter 0 on every row)."""
    raw = b"".join(b"\x00" + rgb[y * w * 3:(y + 1) * w * 3] for y in range(h))

    def chunk(kind, body):
        return struct.pack(">I", len(body)) + kind + body + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)

    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)))
        f.write(chunk(b"IDAT", zlib.compress(raw, 6)))
        f.write(chunk(b"IEND", b""))


def classify(w, h, rgb, step=2):
    """Classify a frame as blank, splash, text or graphical.

    splash: dark screen whose lit pixels sit only in the lower middle
    (two-step spinner and watermark), with some near the bottom edge.
    text: dark screen with lit pixels elsewhere (firmware, GRUB, console).
    graphical: a mostly non-dark screen; which one it is is not identified.
    """
    total = dark = 0
    lit = []
    for y in range(0, h, step):
        row = y * w * 3
        for x in range(0, w, step):
            i = row + x * 3
            luma = (rgb[i] * 299 + rgb[i + 1] * 587 + rgb[i + 2] * 114) // 1000
            total += 1
            if luma < DARK:
                dark += 1
            if luma >= LIT:
                lit.append((x, y))
    dark_ratio = dark / total
    stats = {"dark": round(dark_ratio, 4), "lit": round(len(lit) / total, 4)}
    if dark_ratio < 0.80:
        return "graphical", stats
    if not lit:
        return "blank", stats
    xs = [p[0] for p in lit]
    ys = [p[1] for p in lit]
    stats["bbox"] = [min(xs), min(ys), max(xs), max(ys)]
    lower_middle = min(xs) >= 0.30 * w and max(xs) <= 0.70 * w and min(ys) >= 0.55 * h
    near_bottom = max(ys) >= 0.88 * h
    if dark_ratio > 0.95 and lower_middle and near_bottom:
        return "splash", stats
    return "text", stats


# ---------------------------------------------------------------- QMP

class QMP:
    def __init__(self, path, timeout):
        deadline = time.monotonic() + timeout
        while True:
            try:
                self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self.sock.connect(path)
                break
            except OSError:
                self.sock.close()
                if time.monotonic() > deadline:
                    raise HarnessError("QMP socket did not appear")
                time.sleep(0.2)
        self.sock.settimeout(10)
        self.buf = b""
        self._read()
        self.command("qmp_capabilities")

    def _read(self):
        while True:
            while b"\n" in self.buf:
                line, self.buf = self.buf.split(b"\n", 1)
                if line.strip():
                    msg = json.loads(line)
                    if "event" not in msg:
                        return msg
            data = self.sock.recv(65536)
            if not data:
                raise HarnessError("QMP connection closed")
            self.buf += data

    def command(self, name, **args):
        self.sock.sendall(json.dumps({"execute": name, "arguments": args}).encode() + b"\n")
        reply = self._read()
        if "error" in reply:
            raise HarnessError(f"QMP {name}: {reply['error']}")
        return reply.get("return")


# ---------------------------------------------------------------- run

def find_qemu(explicit):
    for c in [explicit] if explicit else QEMU_CANDIDATES:
        path = shutil.which(c) or (c if os.access(c, os.X_OK) else None)
        if path:
            return path
    raise HarnessError("no QEMU binary found; pass --qemu")


def find_ovmf(code, vars_):
    if code and vars_:
        return code, vars_
    for c, v in OVMF_CANDIDATES:
        if os.path.exists(c) and os.path.exists(v):
            return c, v
    raise HarnessError("no OVMF firmware found; pass --ovmf-code and --ovmf-vars")


def run(args):
    if not os.access("/dev/kvm", os.R_OK | os.W_OK):
        raise HarnessError("/dev/kvm is not accessible")
    disk = os.path.abspath(args.disk)
    if not os.path.isfile(disk):
        raise HarnessError(f"{disk}: no such disk image")
    out = os.path.abspath(args.out)
    os.makedirs(out)  # refuses an existing directory: runs never mix
    frames = os.path.join(out, "frames")
    os.makedirs(frames)
    qemu = find_qemu(args.qemu)
    code, vars_template = find_ovmf(args.ovmf_code, args.ovmf_vars)
    qemu_img = shutil.which("qemu-img")
    if not qemu_img:
        raise HarnessError("qemu-img not found")

    overlay = os.path.join(out, "overlay.qcow2")
    subprocess.run([qemu_img, "create", "-q", "-f", "qcow2", "-F", "qcow2", "-b", disk, overlay], check=True)
    vars_copy = os.path.join(out, "OVMF_VARS.fd")
    shutil.copyfile(vars_template, vars_copy)
    qmp_path = os.path.join(out, "qmp.sock")

    cmd = [
        qemu, "-machine", "q35,accel=kvm", "-cpu", "host",
        "-m", str(args.memory), "-smp", str(args.cpus),
        "-drive", f"if=pflash,format=raw,unit=0,readonly=on,file={code}",
        "-drive", f"if=pflash,format=raw,unit=1,file={vars_copy}",
        "-drive", f"if=virtio,format=qcow2,file={overlay}",
        "-device", "virtio-vga", "-display", "none",
        "-nic", "user,model=virtio-net-pci",
        "-device", "virtio-rng-pci",
        "-serial", f"file:{os.path.join(out, 'serial.log')}",
        "-qmp", f"unix:{qmp_path},server=on,wait=off",
    ] + list(getattr(args, "extra_qemu", None) or [])
    serial_path = os.path.join(out, "serial.log")
    marker = getattr(args, "until_serial", None)
    summary = {
        "disk": disk, "disk_sha256": sha256(disk) if args.hash_disk else None,
        "qemu": qemu, "ovmf_code": code, "memory_mib": args.memory, "cpus": args.cpus,
        "boot_timeout_s": args.boot_timeout, "shutdown_timeout_s": args.shutdown_timeout,
        "expect_splash": args.expect_splash, "until_serial": marker,
        "frames": [], "result": "error", "failures": [],
    }
    log = open(os.path.join(out, "qemu.log"), "wb")
    proc = subprocess.Popen(cmd, stdout=log, stderr=subprocess.STDOUT)
    qmp = None
    try:
        qmp = QMP(qmp_path, 30)
        t0 = time.monotonic()
        last_digest, stable, graphical_at, splash_at, stable_digest = None, 0, None, None, None
        marker_at = None

        def marker_seen():
            try:
                with open(serial_path, "rb") as f:
                    return marker.encode() in f.read()
            except OSError:
                return False

        def capture(phase):
            nonlocal last_digest
            t = round(time.monotonic() - t0, 1)
            ppm = os.path.join(out, "frame.ppm")
            try:
                qmp.command("screendump", filename=ppm)
                w, h, rgb = read_ppm(ppm)
            except (HarnessError, OSError, ValueError):
                return None
            finally:
                if os.path.exists(ppm):
                    os.remove(ppm)
            kind, stats = classify(w, h, rgb)
            digest = hashlib.sha256(rgb).hexdigest()[:16]
            entry = {"t": t, "phase": phase, "class": kind, "size": f"{w}x{h}", "digest": digest, **stats}
            if digest != last_digest:
                name = f"{phase}-{t:06.1f}-{kind}.png"
                write_png(os.path.join(frames, name), w, h, rgb)
                entry["file"] = f"frames/{name}"
                last_digest = digest
            summary["frames"].append(entry)
            return entry

        while time.monotonic() - t0 < args.boot_timeout:
            e = capture("boot")
            if e:
                if e["class"] == "splash" and splash_at is None:
                    splash_at = e["t"]
                if e["class"] == "graphical":
                    graphical_at = graphical_at or e["t"]
                    stable = stable + 1 if e["digest"] == stable_digest else 1
                    stable_digest = e["digest"]
                else:
                    stable, stable_digest = 0, None
                if not marker and stable >= args.stable_frames:
                    break
            if marker and marker_seen():
                marker_at = round(time.monotonic() - t0, 1)
                capture("boot")
                break
            if proc.poll() is not None:
                raise HarnessError(f"QEMU exited during boot with status {proc.returncode}")
            time.sleep(args.interval)
        summary["splash_first_s"] = splash_at
        summary["graphical_first_s"] = graphical_at
        if marker:
            summary["serial_marker_s"] = marker_at
            if marker_at is None:
                summary["failures"].append(f"serial marker {marker!r} not seen within {args.boot_timeout}s")
        elif stable < args.stable_frames:
            summary["failures"].append(f"no stable graphical screen within {args.boot_timeout}s")
        if args.expect_splash and splash_at is None:
            summary["failures"].append("the graphical boot splash never appeared")
        if splash_at is not None and graphical_at is not None:
            summary["text_frames_between_splash_and_graphical"] = sum(
                1 for f in summary["frames"] if f["phase"] == "boot" and f["class"] == "text" and splash_at < f["t"] < graphical_at)

        # An instrumented guest powers itself off after reporting.
        if not getattr(args, "guest_powers_off", False):
            qmp.command("system_powerdown")
        t1 = time.monotonic()
        while proc.poll() is None and time.monotonic() - t1 < args.shutdown_timeout:
            capture("shutdown")
            time.sleep(min(args.interval, 0.3))
        if proc.poll() is None:
            how = "guest poweroff" if getattr(args, "guest_powers_off", False) else "ACPI shutdown"
            summary["failures"].append(f"{how} did not power off within {args.shutdown_timeout}s")
        else:
            summary["shutdown_s"] = round(time.monotonic() - t1, 1)
        summary["shutdown_text_frames"] = sum(1 for f in summary["frames"] if f["phase"] == "shutdown" and f["class"] == "text")
        summary["result"] = "fail" if summary["failures"] else "pass"
    finally:
        if proc.poll() is None:
            try:
                if qmp:
                    qmp.command("quit")
            except (HarnessError, OSError):
                pass
            try:
                proc.wait(10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
        log.close()
        for p in (qmp_path, overlay if not args.keep_overlay else None):
            if p and os.path.exists(p):
                os.remove(p)
        write_summary(out, summary)
    return summary


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def write_summary(out, s):
    with open(os.path.join(out, "summary.json"), "w") as f:
        json.dump(s, f, indent=2)
        f.write("\n")
    lines = [
        f"# Boot check: {s['result'].upper()}", "",
        f"- Disk: `{s['disk']}`" + (f" (sha256 `{s['disk_sha256']}`)" if s.get("disk_sha256") else ""),
        f"- First splash frame: {s.get('splash_first_s')} s; first graphical frame: {s.get('graphical_first_s')} s",
        f"- Text frames between splash and graphical: {s.get('text_frames_between_splash_and_graphical')}",
        f"- Shutdown: {s.get('shutdown_s')} s; text frames during shutdown: {s.get('shutdown_text_frames')}",
    ]
    lines += [f"- Failure: {f}" for f in s["failures"]]
    lines += ["", "| t (s) | phase | class | frame |", "|---|---|---|---|"]
    lines += [f"| {f['t']} | {f['phase']} | {f['class']} | {f.get('file', '')} |" for f in s["frames"] if f.get("file")]
    with open(os.path.join(out, "summary.md"), "w") as f:
        f.write("\n".join(lines) + "\n")


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--disk", required=True, help="qcow2 disk to boot (never written)")
    p.add_argument("--out", required=True, help="new output directory")
    p.add_argument("--qemu")
    p.add_argument("--ovmf-code")
    p.add_argument("--ovmf-vars")
    p.add_argument("--memory", type=int, default=4096, help="MiB")
    p.add_argument("--cpus", type=int, default=4)
    p.add_argument("--boot-timeout", type=int, default=240, help="seconds")
    p.add_argument("--shutdown-timeout", type=int, default=90, help="seconds")
    p.add_argument("--interval", type=float, default=1.0, help="seconds between frames")
    p.add_argument("--stable-frames", type=int, default=5, help="identical graphical frames in a row that count as booted")
    p.add_argument("--expect-splash", action="store_true", help="fail unless a graphical splash is seen")
    p.add_argument("--hash-disk", action="store_true", help="record the disk sha256")
    p.add_argument("--keep-overlay", action="store_true")
    args = p.parse_args()
    try:
        s = run(args)
    except HarnessError as e:
        print(f"bootcheck: {e}", file=sys.stderr)
        return 2
    print(f"bootcheck: {s['result']} ({args.out}/summary.md)")
    for f in s["failures"]:
        print(f"  {f}", file=sys.stderr)
    return 0 if s["result"] == "pass" else 1


if __name__ == "__main__":
    sys.exit(main())
