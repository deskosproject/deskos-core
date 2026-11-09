#!/usr/bin/env python3
"""Boot an unmodified DeskOS disk image in QEMU (UEFI) and record the screen.

The disk is never written: each run boots a throwaway qcow2 overlay. Frames
are captured through QMP, classified with simple pixel heuristics, and
summarized in summary.json and summary.md. The run fails when the machine
never shows a stable graphical screen, when the expected boot splash does not
appear, or when ACPI shutdown does not power the machine off in time.

The harness only classifies screen pixels (blank, text, splash, graphical).
It does not identify which graphical screen appears (GNOME Initial Setup,
GDM or a desktop) and does not test applications. A caller may pass
`keys_on_marker`: when the serial log shows a text marker, the harness types
the paired keys on a Unix-socket VNC keyboard, the input path a lock screen
accepts.

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

# The Secure Boot firmware. The code file holds the firmware; the variable store
# holds the enrolled PK/KEK/db and SecureBootEnable. The plain .fd templates are
# empty (setup mode, Secure Boot off), so only the enrolled stores are accepted:
# Ubuntu's .ms.fd and Fedora's edk2 .secboot.fd. A host without one fails the
# prerequisite instead of reporting a boot that proves nothing.
SECBOOT_OVMF_CANDIDATES = [
    ("/usr/share/OVMF/OVMF_CODE_4M.secboot.fd", "/usr/share/OVMF/OVMF_VARS_4M.ms.fd"),
    ("/usr/share/edk2/ovmf/OVMF_CODE.secboot.fd", "/usr/share/edk2/ovmf/OVMF_VARS.secboot.fd"),
]

# Pixel classes by luma: dark below DARK, lit from LIT (text, logos, spinner).
DARK, LIT = 24, 60

# A login that does not start is retried while the guest is still up. The keys
# are lost for minutes at a time in some boots and land later (runs 42292630365,
# 42317585720: nothing at 198 s, 418 s, then a working window past 500 s), so
# the rounds cover the whole boot: eight attempts a minute apart.
LOGIN_ROUNDS = 8
LOGIN_RETRY_AFTER = 60.0


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


# ---------------------------------------------------------------- input

# The unshifted key for a shifted symbol on the US layout. QEMU maps the
# direct keysym for "@" to the "2" key without Shift, so these need the chord.
SHIFTED = {
    "!": "1", "@": "2", "#": "3", "$": "4", "%": "5", "^": "6", "&": "7",
    "*": "8", "(": "9", ")": "0", "_": "-", "+": "=", "{": "[", "}": "]",
    "|": "\\", ":": ";", "\"": "'", "<": ",", ">": ".", "?": "/", "~": "`",
}


def keysym(ch):
    """The X11 keysym for `ch`, and whether Shift is needed.

    Uppercase letters go as their own keysym: QEMU maps them with a Caps Lock
    toggle, which is correct while Caps Lock starts off. Sending Shift plus the
    lower key instead toggled Caps Lock on the tested image. Shifted symbols
    have no such mapping and use the chord.
    """
    if ch in SHIFTED:
        return True, ord(SHIFTED[ch])
    return False, ord(ch)


def vnc_type(path, text, select_first=False, click=None, wake_click=False, timeout=15,
             snapshot=None, settle=5.0, retries=1):
    """Type `text` and Return into a QEMU VNC server on a Unix socket.

    The lock screen refuses remote-desktop portal input: Mutter tears the
    remote session down when the screen locks. The hypervisor's own keyboard
    does reach it, and a Unix-socket VNC server keeps that input on the host.
    The client is the RFB 3.8 handshake and key events only; it does not read
    the framebuffer.

    The first keysym is Shift, which raises the lock screen's curtain without
    typing anything, so the password is not swallowed by the animation. The
    delay after it covers the curtain; Shift is harmless when the password
    field already has focus.

    A list types a login: each part gets Return, with a pause between parts for
    the greeter to draw the next field. `select_first` sends Ctrl+A before the
    first part, so a remembered username is replaced instead of appended to.
    `click` is a framebuffer coordinate to press first; a drawn entry does not
    always own keyboard focus, and the click gives it to the field.

    `wake_click` presses the framebuffer's centre before anything else, for the
    lock screen: the display may be in DPMS off by then (run 42154612511, frame
    83.4 s reads "Display output is not active"), and Shift alone wakes the
    panel without raising the curtain, so the password would be typed into a
    screen that is still waking. A pointer event wakes and raises it at once.

    `snapshot` makes the typing checkable. When it is given, the caller passes a
    callable that returns a digest of the screen; after each part the keyboard
    waits `settle` seconds for that digest to change, because a keystroke that
    lands always changes the pixels it lands in. If nothing changed, the part is
    cleared and typed again, `retries` times. This is the difference between
    "sent" and "landed": run 42215986390 typed the AD user at 136.7 s, the
    greeter drew at 139.7 s, and the field stayed empty for the next 250 s while
    the harness went on to type a password into nothing.
    """
    typed_result = {"attempts": 0, "landed": True}
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    try:
        sock.connect(path)

        def recv(n):
            out = b""
            while len(out) < n:
                chunk = sock.recv(n - len(out))
                if not chunk:
                    raise HarnessError("VNC connection closed during the handshake")
                out += chunk
            return out

        recv(12)
        sock.sendall(b"RFB 003.008\n")
        count = recv(1)[0]
        offered = set(recv(count))
        if 1 not in offered:
            raise HarnessError(f"VNC security types {sorted(offered)} do not include None")
        sock.sendall(b"\x01")
        result = struct.unpack(">I", recv(4))[0]
        if result != 0:
            raise HarnessError(f"VNC security failed with status {result}")
        sock.sendall(b"\x01")
        head = recv(24)
        name_len = struct.unpack(">I", head[20:24])[0]
        recv(name_len)

        def key(sym, down):
            sock.sendall(struct.pack(">BBHI", 4, 1 if down else 0, 0, sym))

        def pointer(x, y, down):
            sock.sendall(struct.pack(">BBHH", 5, 1 if down else 0, x, y))

        def type_chars(part):
            for ch in part:
                shift, sym = keysym(ch)
                if shift:
                    key(0xFFE1, True)
                key(sym, True)
                key(sym, False)
                if shift:
                    key(0xFFE1, False)
                time.sleep(0.02)

        def press_return():
            # The guest has to read the submit before the socket closes; a
            # close right after the write dropped it on the tested image.
            time.sleep(0.3)
            key(0xFF0D, True)
            time.sleep(0.1)
            key(0xFF0D, False)
            time.sleep(1.0)

        def select_all():
            key(0xFFE3, True)       # Control_L
            key(ord("a"), True)
            key(ord("a"), False)
            key(0xFFE3, False)
            time.sleep(0.1)

        def changed(base, seconds):
            """Whether the screen changed within `seconds`, given a digest."""
            deadline = time.monotonic() + seconds
            while time.monotonic() < deadline:
                time.sleep(0.3)
                now = snapshot()
                if now and base and now != base:
                    return True
            return False

        parts = list(text) if isinstance(text, (list, tuple)) else [text]
        if click:
            # The greeter draws the username entry before it owns keyboard
            # focus; a click gives it to the field. This was a documented
            # parameter that nothing used, which is why a drawn field could
            # swallow every key: runs 42292630365 and 42297834953.
            pointer(click[0], click[1], True)
            pointer(click[0], click[1], False)
            time.sleep(0.5)
        if wake_click:
            # A pointer event wakes the panel and raises the curtain; Shift on a
            # dark screen only wakes it, and the typing is lost (run 42154612511,
            # frame 83.4 s: "Display output is not active").
            pointer(640, 478, True)
            pointer(640, 478, False)
            time.sleep(1.0)
        # Shift_L raises the curtain; Return on an empty field would submit.
        key(0xFFE1, True)
        key(0xFFE1, False)
        time.sleep(2.0)
        if select_first:
            # The greeter can draw the username field before it owns keyboard
            # focus; the ready marker comes from the shell process, not the
            # dialog. Give the dialog time to take the seat.
            time.sleep(5.0)
            select_all()
        attempts = 0
        landed = True
        for i, part in enumerate(parts):
            base = snapshot() if snapshot else None
            type_chars(part)
            press_return()
            attempts += 1
            landed = True
            if snapshot:
                landed = changed(base, settle)
                retried = 0
                while not landed and retried < retries:
                    # The keys were sent while the greeter was still drawing and
                    # nothing reached the field. Click the field again (the first
                    # click landed before the field existed), clear what is
                    # there, and type once more.
                    pointer(640, 478, True)
                    pointer(640, 478, False)
                    time.sleep(1.0)
                    select_all()
                    base = snapshot()
                    type_chars(part)
                    press_return()
                    attempts += 1
                    retried += 1
                    landed = changed(base, settle)
            if i + 1 < len(parts):
                # The password page follows the directory lookup, not the Return,
                # and the lookup takes 50 to 90 seconds with a disposable realm.
                # A fixed sixty seconds typed the password 5 s before the page
                # existed on run 42292630365 (round 3 landed, no session). Wait
                # for the page to draw, and type anyway if it never does.
                if snapshot:
                    page = snapshot()
                    deadline = time.monotonic() + 90
                    while time.monotonic() < deadline and not changed(page, 2.0):
                        pass
                    time.sleep(0.5)
                else:
                    time.sleep(60.0)
        typed_result["attempts"] = attempts
        typed_result["landed"] = landed
    finally:
        sock.close()
    return typed_result


# ---------------------------------------------------------------- run

def find_qemu(explicit):
    for c in [explicit] if explicit else QEMU_CANDIDATES:
        path = shutil.which(c) or (c if os.access(c, os.X_OK) else None)
        if path:
            return path
    raise HarnessError("no QEMU binary found; pass --qemu")


def find_ovmf(code, vars_, secure=False):
    if code and vars_:
        return code, vars_
    candidates = SECBOOT_OVMF_CANDIDATES if secure else OVMF_CANDIDATES
    for c, v in candidates:
        if os.path.exists(c) and os.path.exists(v):
            return c, v
    what = "Secure Boot OVMF" if secure else "OVMF"
    raise HarnessError(f"no {what} firmware found; pass --ovmf-code and --ovmf-vars")


def machine_spec(secure=False):
    """q35 with SMM, which the Secure Boot pflash device requires."""
    return "q35,accel=kvm,smm=on" if secure else "q35,accel=kvm"


def pflash_secure_args(secure=False):
    """Mark the pflash devices secure, which is what enforces the signatures."""
    return ["-global", "driver=cfi.pflash01,property=secure,value=on"] if secure else []


def serial_hint(text, limit=400):
    """What the guest last said, when what it did not say is the failure.

    A report unit that never runs and a report that never arrives look identical from the
    end marker alone. The unit's output goes to the console for that reason, so the tail of
    the serial log is the sentence that turns a long wait into a reason. Bounded: a failure
    is read in a summary, not in a file.
    """
    tail = " ".join(text.decode("utf-8", "replace").split())
    if not tail:
        return "; the serial log is empty, so the guest said nothing at all"
    if len(tail) > limit:
        tail = "…" + tail[-limit:]
    return f"; the guest last said: {tail}"


def network_args(args):
    """The guest's NIC, or nothing for an offline boot.

    The session check proves a login needs no network; a boot that quietly kept
    user networking would make the claim untested.
    """
    # QEMU adds a user-mode NIC of its own when no network option is given, so
    # an offline boot has to say `none`; leaving the argument out would keep the
    # default NIC and make the offline claim untested.
    return ["-nic", "user,model=virtio-net-pci"] if getattr(args, "network", True) else ["-nic", "none"]


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
    secure = bool(getattr(args, "secure_boot", False))
    code, vars_template = find_ovmf(args.ovmf_code, args.ovmf_vars, secure=secure)
    qemu_img = shutil.which("qemu-img")
    if not qemu_img:
        raise HarnessError("qemu-img not found")

    overlay = os.path.join(out, "overlay.qcow2")
    # Do not assume the backing format. bootc-image-builder writes qcow2, while a
    # disk from `bootc install to-disk` is raw, and both should be checkable with
    # the same harness.
    backing_format = json.loads(subprocess.run(
        [qemu_img, "info", "--output=json", disk],
        check=True, capture_output=True, text=True).stdout)["format"]
    subprocess.run([qemu_img, "create", "-q", "-f", "qcow2", "-F", backing_format,
                    "-b", disk, overlay], check=True)
    vars_copy = os.path.join(out, "OVMF_VARS.fd")
    shutil.copyfile(vars_template, vars_copy)
    qmp_path = os.path.join(out, "qmp.sock")
    keys_on_marker = list(getattr(args, "keys_on_marker", None) or [])
    vnc_path = os.path.join(out, "vnc.sock")

    cmd = [
        qemu, "-machine", machine_spec(secure), "-cpu", "host",
        "-m", str(args.memory), "-smp", str(args.cpus),
        "-drive", f"if=pflash,format=raw,unit=0,readonly=on,file={code}",
        "-drive", f"if=pflash,format=raw,unit=1,file={vars_copy}",
    ] + pflash_secure_args(secure) + [
        "-drive", f"if=virtio,format=qcow2,file={overlay}",
        "-device", "virtio-vga", "-display", "none",
    ] + network_args(args) + [
        "-device", "virtio-rng-pci",
        "-serial", f"file:{os.path.join(out, 'serial.log')}",
        "-qmp", f"unix:{qmp_path},server=on,wait=off",
    ] + (["-vnc", f"unix:{vnc_path}"] if keys_on_marker else []) + list(getattr(args, "extra_qemu", None) or [])
    serial_path = os.path.join(out, "serial.log")
    marker = getattr(args, "until_serial", None)
    ready_marker = getattr(args, "until_serial_ready", None)
    ready_timeout = getattr(args, "ready_timeout", None)
    summary = {
        "disk": disk, "disk_sha256": sha256(disk) if args.hash_disk else None,
        "qemu": qemu, "ovmf_code": code, "secure_boot": secure,
        "memory_mib": args.memory, "cpus": args.cpus,
        "boot_timeout_s": args.boot_timeout, "shutdown_timeout_s": args.shutdown_timeout,
        "expect_splash": args.expect_splash, "until_serial": marker,
        "until_serial_ready": ready_marker, "ready_timeout_s": ready_timeout,
        "frames": [], "result": "error", "failures": [],
    }
    log = open(os.path.join(out, "qemu.log"), "wb")
    proc = subprocess.Popen(cmd, stdout=log, stderr=subprocess.STDOUT)
    qmp = None
    try:
        qmp = QMP(qmp_path, 30)
        t0 = time.monotonic()
        last_digest, stable, graphical_at, splash_at, stable_digest = None, 0, None, None, None
        marker_at, ready_at, ready_failed = None, None, False
        keys_sent = set()
        retry_login = []

        def serial_text():
            try:
                with open(serial_path, "rb") as f:
                    return f.read()
            except OSError:
                return b""

        def marker_seen():
            return marker.encode() in serial_text()

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

        def band_digest():
            """Digest of the middle of the screen, where a prompt draws.

            The status bar carries a clock that changes once a minute and the
            dock animates, so a whole-frame digest answers "did anything move",
            not "did my text land". A typed character changes the middle rows.
            """
            ppm = os.path.join(out, "band.ppm")
            try:
                qmp.command("screendump", filename=ppm)
                w, h, rgb = read_ppm(ppm)
            except (HarnessError, OSError, ValueError):
                return None
            finally:
                if os.path.exists(ppm):
                    os.remove(ppm)
            top, bottom = int(h * 0.30), int(h * 0.80)
            return hashlib.sha256(rgb[top * w * 3:bottom * w * 3]).hexdigest()[:16]

        def changed_in(base, seconds):
            """Whether the middle of the screen moved within `seconds`."""
            if base is None:
                return False
            deadline = time.monotonic() + seconds
            while time.monotonic() < deadline:
                time.sleep(0.25)
                if band_digest() != base:
                    return True
            return False

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
            if ready_marker and ready_at is None:
                if ready_marker.encode() in serial_text():
                    ready_at = round(time.monotonic() - t0, 1)
                elif ready_timeout and time.monotonic() - t0 > ready_timeout:
                    # The guest never signalled that the instrumentation installed,
                    # so the end marker will not come: a credential was dropped. Fail
                    # now, with the reason, instead of waiting for the end marker.
                    summary["failures"].append(
                        f"serial marker {ready_marker!r} not seen within {ready_timeout}s: "
                        f"the guest never signalled it was instrumented "
                        f"(a credential may have been dropped)"
                        + serial_hint(serial_text()))
                    ready_failed = True
                    break
            for i, (marker_text, keys) in enumerate(keys_on_marker):
                if i in keys_sent or marker_text.encode() not in serial_text():
                    continue
                try:
                    login = isinstance(keys, (list, tuple))
                    if login:
                        # The ready marker is the greeter's shell process; the
                        # username field is drawn later (run 42269507235: typed at
                        # 163.0 s, field drawn at 166.0 s). Wait up to ten seconds
                        # for the screen to move before the first keystroke, then
                        # type anyway: when the marker arrives after the field is
                        # drawn, nothing moves and the extra wait would be waste.
                        base = band_digest()
                        deadline = time.monotonic() + 10
                        while time.monotonic() < deadline and not changed_in(base, 1.0):
                            pass
                        time.sleep(1.5)
                    typed = vnc_type(vnc_path, keys, select_first=login,
                                     click=(640, 478) if login else None,
                                     wake_click=not login, snapshot=band_digest)
                    summary.setdefault("keys_sent", []).append(
                        {"marker": marker_text, "t": round(time.monotonic() - t0, 1),
                         "attempts": typed.get("attempts"), "landed": typed.get("landed")})
                    if login:
                        # A login that did not start is the same problem one step
                        # later: the keys landed nowhere, or the password went to a
                        # page that was not there. Type the whole login again while
                        # the guest keeps running, and record how many rounds it
                        # took; the session report's first line is the signal.
                        retry_login.append({
                            "keys": keys,
                            "next": time.monotonic() + LOGIN_RETRY_AFTER,
                            "rounds": 1,
                        })
                except (HarnessError, OSError) as exc:
                    summary["failures"].append(f"typing on VNC after {marker_text!r}: {exc}")
                keys_sent.add(i)

            for pending in list(retry_login):
                if b"session_type=" in serial_text():
                    retry_login.remove(pending)
                    continue
                if time.monotonic() < pending["next"]:
                    continue
                if pending["rounds"] >= LOGIN_ROUNDS:
                    retry_login.remove(pending)
                    summary["failures"].append(
                        f"no session after {pending['rounds']} login rounds: "
                        "the guest never wrote a session report" + serial_hint(serial_text()))
                    continue
                try:
                    typed = vnc_type(vnc_path, pending["keys"], select_first=True,
                                     click=(640, 478), snapshot=band_digest)
                    pending["rounds"] += 1
                    pending["next"] = time.monotonic() + LOGIN_RETRY_AFTER
                    summary.setdefault("login_rounds", []).append(
                        {"round": pending["rounds"], "t": round(time.monotonic() - t0, 1),
                         "attempts": typed.get("attempts"), "landed": typed.get("landed")})
                except (HarnessError, OSError) as exc:
                    # A broken pipe means QEMU closed the VNC client, which it
                    # does when the display mode changes under it: the keys were
                    # not delivered, so this is another round, not the end.
                    # Measured on run 42309673200, round 2.
                    pending["rounds"] += 1
                    pending["next"] = time.monotonic() + LOGIN_RETRY_AFTER
                    summary["failures"].append(f"login round {pending['rounds']}: {exc}")
                    summary.setdefault("login_rounds", []).append(
                        {"round": pending["rounds"], "t": round(time.monotonic() - t0, 1),
                         "error": str(exc)})
            if marker and marker_seen():
                marker_at = round(time.monotonic() - t0, 1)
                capture("boot")
                break
            if proc.poll() is not None:
                raise HarnessError(f"QEMU exited during boot with status {proc.returncode}")
            # The splash can last well under a second on a fast boot, and the
            # loop used to sample once a second: one run saw the graphical
            # desktop and no splash, and the gate failed on a good image
            # (2026-10-28). Sample finely until the splash or the graphical
            # screen is seen, then fall back to the configured interval. Frames
            # are only written when the picture changes, so this adds a handful.
            hunting = splash_at is None and graphical_at is None
            time.sleep(min(args.interval, 0.25) if hunting else args.interval)
        summary["splash_first_s"] = splash_at
        summary["graphical_first_s"] = graphical_at
        summary["ready_serial_s"] = ready_at
        if marker:
            summary["serial_marker_s"] = marker_at
            if marker_at is None and not ready_failed:
                summary["failures"].append(
                    f"serial marker {marker!r} not seen within {args.boot_timeout}s"
                    + serial_hint(serial_text()))
        elif stable < args.stable_frames:
            summary["failures"].append(f"no stable graphical screen within {args.boot_timeout}s")
        if args.expect_splash and splash_at is None:
            summary["failures"].append("the graphical boot splash never appeared")
        if splash_at is not None and graphical_at is not None:
            summary["text_frames_between_splash_and_graphical"] = sum(
                1 for f in summary["frames"] if f["phase"] == "boot" and f["class"] == "text" and splash_at < f["t"] < graphical_at)

        # An instrumented guest powers itself off after reporting. When the guest
        # never signalled that it was instrumented, there is nothing to wait for:
        # the run fails with that reason alone and qemu is quit in the finally block.
        if not ready_failed:
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
        for p in (qmp_path, vnc_path if keys_on_marker else None,
                  overlay if not args.keep_overlay else None):
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
        f"- Secure Boot: {'on' if s.get('secure_boot') else 'off'}",
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
    p.add_argument("--disk", required=True, help="disk to boot (never written): qcow2 from bootc-image-builder, or raw from `bootc install to-disk`")
    p.add_argument("--out", required=True, help="new output directory")
    p.add_argument("--qemu")
    p.add_argument("--ovmf-code")
    p.add_argument("--ovmf-vars")
    p.add_argument("--secure-boot", action="store_true",
                   help="boot with UEFI Secure Boot enabled (the vendor chain must be signed)")
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
