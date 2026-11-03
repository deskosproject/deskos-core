import os
import tempfile
import unittest

import bootcheck

W, H = 320, 200


def frame(paint):
    rgb = bytearray(W * H * 3)
    paint(rgb)
    return bytes(rgb)


def rect(rgb, x0, y0, x1, y1, v=255):
    for y in range(y0, y1):
        for x in range(x0, x1):
            i = (y * W + x) * 3
            rgb[i:i + 3] = bytes((v, v, v))


class ClassifyTest(unittest.TestCase):
    def kind(self, paint):
        return bootcheck.classify(W, H, frame(paint), step=1)[0]

    def test_blank(self):
        self.assertEqual(self.kind(lambda rgb: None), "blank")

    def test_splash(self):
        def paint(rgb):
            rect(rgb, 156, 136, 164, 144)  # spinner at 0.7 height
            rect(rgb, 140, 184, 180, 192)  # watermark near 0.96
        self.assertEqual(self.kind(paint), "splash")

    def test_spinner_without_watermark_is_not_splash(self):
        self.assertEqual(self.kind(lambda rgb: rect(rgb, 156, 136, 164, 144)), "text")

    def test_console_text(self):
        self.assertEqual(self.kind(lambda rgb: rect(rgb, 2, 2, 120, 10)), "text")

    def test_graphical(self):
        self.assertEqual(self.kind(lambda rgb: rect(rgb, 0, 0, W, H, 120)), "graphical")


class ImageTest(unittest.TestCase):
    def test_ppm_and_png(self):
        rgb = frame(lambda b: rect(b, 10, 10, 20, 20))
        with tempfile.TemporaryDirectory() as d:
            ppm = os.path.join(d, "f.ppm")
            with open(ppm, "wb") as f:
                f.write(b"P6\n# c\n%d %d\n255\n" % (W, H) + rgb)
            self.assertEqual(bootcheck.read_ppm(ppm), (W, H, rgb))
            png = os.path.join(d, "f.png")
            bootcheck.write_png(png, W, H, rgb)
            with open(png, "rb") as f:
                self.assertEqual(f.read(8), b"\x89PNG\r\n\x1a\n")


class SerialHintTest(unittest.TestCase):
    """What the guest said, in the failure line that used to say only what it did not."""

    def test_an_empty_log_says_so(self):
        self.assertEqual(bootcheck.serial_hint(b""),
                         "; the serial log is empty, so the guest said nothing at all")

    def test_the_tail_is_quoted(self):
        got = bootcheck.serial_hint(b"\r\nreport.sh: line 2: syntax error near line 2\r\n")
        self.assertEqual(got, "; the guest last said: report.sh: line 2: syntax error near line 2")

    def test_it_is_bounded_and_keeps_the_end(self):
        got = bootcheck.serial_hint(b"x" * 5000)
        self.assertLessEqual(len(got), 430)
        self.assertTrue(got.startswith("; the guest last said: …"))
        self.assertTrue(got.endswith("x"))


class NetworkTest(unittest.TestCase):
    def test_a_normal_boot_carries_a_nic(self):
        self.assertEqual(bootcheck.network_args(type("A", (), {})()),
                         ["-nic", "user,model=virtio-net-pci"])

    def test_an_offline_boot_carries_none(self):
        # QEMU adds a user NIC when nothing is said, so silence is not offline.
        self.assertEqual(bootcheck.network_args(type("A", (), {"network": False})()),
                         ["-nic", "none"])


class KeysymTest(unittest.TestCase):
    def test_lowercase_and_digits_need_no_shift(self):
        self.assertEqual(bootcheck.keysym("a"), (False, ord("a")))
        self.assertEqual(bootcheck.keysym("7"), (False, ord("7")))

    def test_uppercase_is_shift_plus_the_lowercase_key(self):
        self.assertEqual(bootcheck.keysym("A"), (True, ord("a")))

    def test_a_shifted_symbol_uses_its_base_key(self):
        self.assertEqual(bootcheck.keysym("@"), (True, ord("2")))
        self.assertEqual(bootcheck.keysym("_"), (True, ord("-")))


class VncTypeTest(unittest.TestCase):
    """The hypervisor keyboard the lock screen accepts, against a fake RFB server."""

    def fake_server(self, path, events):
        import socket
        import struct

        srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        srv.bind(path)
        srv.listen(1)

        def recv(conn, n):
            out = b""
            while len(out) < n:
                chunk = conn.recv(n - len(out))
                if not chunk:
                    raise AssertionError("client closed early")
                out += chunk
            return out

        def serve():
            conn, _ = srv.accept()
            try:
                conn.sendall(b"RFB 003.008\n")
                recv(conn, 12)
                conn.sendall(b"\x01\x01")  # one security type: None
                recv(conn, 1)
                conn.sendall(struct.pack(">I", 0))
                recv(conn, 1)  # ClientInit
                conn.sendall(struct.pack(">HH", 640, 480) + b"\x00" * 16 + struct.pack(">I", 0))
                while True:
                    head = recv(conn, 8)
                    events.append((head[0], head[1], struct.unpack(">I", head[4:8])[0]))
                    if len(events) >= 2 and events[-1] == (4, 0, 0xFF0D) and events[-2][2] == 0xFF0D:
                        break
            finally:
                conn.close()
                srv.close()

        import threading
        thread = threading.Thread(target=serve, daemon=True)
        thread.start()
        return thread

    def test_it_raises_the_curtain_then_types_the_password(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "vnc.sock")
            events = []
            thread = self.fake_server(path, events)
            bootcheck.vnc_type(path, "redhat")
            thread.join(5)
        # Shift first, so the animation does not swallow the password; then each
        # character and Return, each as a press and a release.
        self.assertEqual(events[0], (4, 1, 0xFFE1))
        self.assertEqual(events[1], (4, 0, 0xFFE1))
        typed = [sym for kind, down, sym in events[2:] if down]
        self.assertEqual(typed, [ord(ch) for ch in "redhat"] + [0xFF0D])


if __name__ == "__main__":
    unittest.main()
